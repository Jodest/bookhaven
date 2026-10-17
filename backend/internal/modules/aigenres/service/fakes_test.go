package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/modules/aigenres/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var now0 = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// schemaDDL mirrors migration 000032 with SQLite-native types.
const schemaDDL = `
CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE);
CREATE TABLE genre_ai_runs (
	id TEXT PRIMARY KEY, trigger TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'queued', error_code TEXT,
	model TEXT NOT NULL DEFAULT '', prompt_version INTEGER NOT NULL, fingerprint TEXT NOT NULL DEFAULT '',
	batches_total INTEGER NOT NULL DEFAULT 0, batches_done INTEGER NOT NULL DEFAULT 0,
	suggestions_created INTEGER NOT NULL DEFAULT 0, input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0, created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL, started_at DATETIME, finished_at DATETIME
);
CREATE TABLE genre_ai_suggestions (
	id TEXT PRIMARY KEY, run_id TEXT REFERENCES genre_ai_runs(id) ON DELETE SET NULL, kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'open', set_key TEXT NOT NULL, reason_codes TEXT NOT NULL DEFAULT '[]',
	reason TEXT NOT NULL DEFAULT '', confidence REAL NOT NULL, model TEXT NOT NULL, prompt_version INTEGER NOT NULL,
	applied_target_id TEXT, decided_by TEXT REFERENCES users(id) ON DELETE SET NULL, decided_at DATETIME,
	created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX uq_genre_ai_suggestions_set_key_live ON genre_ai_suggestions (set_key) WHERE status IN ('open','dismissed');
CREATE TABLE genre_ai_suggestion_genres (
	suggestion_id TEXT NOT NULL REFERENCES genre_ai_suggestions(id) ON DELETE CASCADE, genre_id TEXT NOT NULL,
	role TEXT NOT NULL, position INTEGER NOT NULL, name_snapshot TEXT NOT NULL,
	PRIMARY KEY (suggestion_id, genre_id)
);
`

func newRepo(t *testing.T) (*repository.GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(schemaDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return repository.NewGormRepository(db), db
}

func insertUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", id, id.String()+"@x.test").Error; err != nil {
		t.Fatal(err)
	}
	return id
}

// fakeCatalog is an in-memory genre catalog that also plays the applier. It
// counts the writes so tests can assert the analyzer never changes a genre.
type fakeCatalog struct {
	mu      sync.Mutex
	genres  map[uuid.UUID]*adminservice.GenreState
	created map[uuid.UUID]time.Time
	samples map[uuid.UUID][]adminrepository.SampleBook
	pairs   []adminservice.GenreHintPair

	mergeCalls  [][]uuid.UUID // sources of every MergeGenresBatch call
	mergeTarget []uuid.UUID
	statusCalls int
	mergeErr    error
}

func newFakeCatalog() *fakeCatalog {
	return &fakeCatalog{
		genres:  map[uuid.UUID]*adminservice.GenreState{},
		created: map[uuid.UUID]time.Time{},
		samples: map[uuid.UUID][]adminrepository.SampleBook{},
	}
}

func (c *fakeCatalog) add(name, status string, books int64) uuid.UUID {
	id := uuid.New()
	c.genres[id] = &adminservice.GenreState{ID: id, Name: name, Status: status, BooksTotal: books}
	c.created[id] = now0.Add(time.Duration(len(c.genres)) * time.Minute)
	return id
}

func (c *fakeCatalog) ListGenreAnalysisCandidates(ctx context.Context) ([]adminservice.GenreAnalysisCandidate, error) {
	var out []adminservice.GenreAnalysisCandidate
	for id, g := range c.genres {
		if g.MergedIntoID != nil || g.Status == catalogDomain.GenreStatusRejected {
			continue
		}
		out = append(out, adminservice.GenreAnalysisCandidate{
			Genre:      catalogDomain.Genre{ID: id, Name: g.Name, Status: g.Status, WikidataQID: g.WikidataQID, CreatedAt: c.created[id]},
			BooksTotal: g.BooksTotal,
		})
	}
	return out, nil
}

func (c *fakeCatalog) ListGenreSampleBooks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]adminrepository.SampleBook, error) {
	out := map[uuid.UUID][]adminrepository.SampleBook{}
	for _, id := range ids {
		if s, ok := c.samples[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func (c *fakeCatalog) GenreHintPairs(ctx context.Context, genres []catalogDomain.Genre) []adminservice.GenreHintPair {
	return c.pairs
}

func (c *fakeCatalog) GenresByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]adminservice.GenreState, error) {
	out := map[uuid.UUID]adminservice.GenreState{}
	for _, id := range ids {
		if g, ok := c.genres[id]; ok {
			out[id] = *g
		}
	}
	return out, nil
}

func (c *fakeCatalog) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (*adminservice.GenreBatchMergeOutcome, error) {
	c.mergeCalls = append(c.mergeCalls, append([]uuid.UUID(nil), sourceIDs...))
	c.mergeTarget = append(c.mergeTarget, targetID)
	if c.mergeErr != nil {
		return nil, c.mergeErr
	}
	target, ok := c.genres[targetID]
	if !ok || target.MergedIntoID != nil {
		return nil, adminservice.ErrGenreNotFound
	}
	for _, id := range sourceIDs {
		g, ok := c.genres[id]
		if !ok || g.MergedIntoID != nil {
			return nil, adminservice.ErrGenreNotFound
		}
	}
	for _, id := range sourceIDs {
		g := c.genres[id]
		tid := targetID
		g.MergedIntoID = &tid
		g.Status = catalogDomain.GenreStatusRejected
	}
	return &adminservice.GenreBatchMergeOutcome{
		Genre:         &catalogDomain.Genre{ID: targetID, Name: target.Name, Status: target.Status},
		SourcesMerged: len(sourceIDs), BooksReassigned: 3, CollectionsSwitched: 1,
	}, nil
}

func (c *fakeCatalog) BulkSetGenreStatus(ctx context.Context, ids []uuid.UUID, status string) (adminservice.GenreBulkStatusReport, error) {
	c.statusCalls++
	report := adminservice.GenreBulkStatusReport{Processed: len(ids), Errors: []adminservice.GenreBulkStatusRowError{}}
	for _, id := range ids {
		g, ok := c.genres[id]
		if !ok || g.MergedIntoID != nil {
			report.Errors = append(report.Errors, adminservice.GenreBulkStatusRowError{GenreID: id.String(), Message: "genre not found"})
			continue
		}
		g.Status = status
		report.Updated++
	}
	return report, nil
}

func (c *fakeCatalog) writes() int { return len(c.mergeCalls) + c.statusCalls }

// scriptedLLM answers with queued responses and records the requests.
type scriptedLLM struct {
	mu        sync.Mutex
	responses []scripted
	requests  []llm.StructuredRequest
}

type scripted struct {
	resp llm.StructuredResponse
	err  error
}

func (s *scriptedLLM) Structured(ctx context.Context, req llm.StructuredRequest) (llm.StructuredResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return llm.StructuredResponse{}, errors.New("scriptedLLM: no response left")
	}
	r := s.responses[0]
	s.responses = s.responses[1:]
	return r.resp, r.err
}

func jsonResp(v any) scripted {
	raw, _ := json.Marshal(v)
	return scripted{resp: llm.StructuredResponse{JSON: raw, StopReason: llm.StopReasonEndTurn, Model: "fake-model", InputTokens: 100, OutputTokens: 20}}
}

type item map[string]any

func answer(items ...item) map[string]any {
	list := make([]item, 0, len(items))
	list = append(list, items...)
	return map[string]any{"suggestions": list}
}

func dup(target string, genres []string, conf float64) item {
	return item{"kind": "duplicate", "target": target, "genres": genres, "reason_codes": []string{"spelling"}, "reason": "одно и то же", "confidence": conf}
}

func junkItem(ref string, conf float64) item {
	return item{"kind": "junk", "target": nil, "genres": []string{ref}, "reason_codes": []string{"date"}, "reason": "не жанр", "confidence": conf}
}

func newTestAnalyzer(client llm.Client, repo Repository, catalog GenreCatalog, batch int) *Analyzer {
	a := NewAnalyzer(client, repo, catalog, "cfg-model", batch, nil)
	a.now = func() time.Time { return now0 }
	a.sleep = func(ctx context.Context, d time.Duration) error { return nil }
	return a
}

func queuedRun(t *testing.T, repo Repository, trigger string) uuid.UUID {
	t.Helper()
	run := &domain.Run{Trigger: trigger, Status: domain.RunQueued, PromptVersion: PromptVersion, CreatedAt: now0}
	if err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	return run.ID
}

func nameOf(i int) string { return fmt.Sprintf("Жанр %d", i) }

func errTransient() error { return fmt.Errorf("overloaded: %w", llm.ErrTransient) }
