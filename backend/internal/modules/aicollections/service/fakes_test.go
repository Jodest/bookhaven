package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	curateddomain "github.com/bookhaven/backend/internal/modules/curated/domain"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

var now0 = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return now0 }

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	drafts    map[uuid.UUID]*domain.Draft
	proposals map[uuid.UUID][]domain.Proposal
	purgedAt  []time.Time
	createErr error
	appends   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{drafts: map[uuid.UUID]*domain.Draft{}, proposals: map[uuid.UUID][]domain.Proposal{}}
}

func (r *fakeRepo) CreateDraft(ctx context.Context, d *domain.Draft) error {
	if r.createErr != nil {
		return r.createErr
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	cp := *d
	r.drafts[d.ID] = &cp
	return nil
}

func (r *fakeRepo) GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, error) {
	d, ok := r.drafts[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	cp.ProposalsCount = len(r.proposals[id])
	return &cp, nil
}

func (r *fakeRepo) ListDraftsByCollection(ctx context.Context, collectionID uuid.UUID, limit int) ([]domain.Draft, error) {
	var out []domain.Draft
	for id, d := range r.drafts {
		if d.CollectionID == collectionID {
			cp := *d
			cp.ProposalsCount = len(r.proposals[id])
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) FindActiveDraft(ctx context.Context, collectionID uuid.UUID, staleBefore time.Time) (*domain.Draft, error) {
	for _, d := range r.drafts {
		if d.CollectionID == collectionID && domain.IsActiveStatus(d.Status) && d.UpdatedAt.After(staleBefore) {
			cp := *d
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *fakeRepo) CountDraftsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	for _, d := range r.drafts {
		if d.CreatedBy != nil && *d.CreatedBy == userID && d.CreatedAt.After(since) {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error {
	if d := r.drafts[id]; d != nil && domain.IsActiveStatus(d.Status) {
		d.Status, d.UpdatedAt = domain.StatusRunning, now
	}
	return nil
}

func (r *fakeRepo) MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error {
	if d := r.drafts[id]; d != nil {
		d.Status, d.ErrorCode, d.UpdatedAt, d.FinishedAt = domain.StatusFailed, &code, now, &now
	}
	return nil
}

func (r *fakeRepo) MarkDone(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error {
	if d := r.drafts[id]; d != nil {
		d.Status, d.ErrorCode, d.UpdatedAt, d.FinishedAt = domain.StatusDone, errorCode, now, &now
	}
	return nil
}

func (r *fakeRepo) ListProposals(ctx context.Context, draftID uuid.UUID) ([]domain.Proposal, error) {
	return append([]domain.Proposal(nil), r.proposals[draftID]...), nil
}

func (r *fakeRepo) AppendProposals(ctx context.Context, draftID uuid.UUID, startRank int, ps []domain.NewProposal, model string, in, out int, now time.Time) error {
	r.appends++
	d := r.drafts[draftID]
	if d == nil {
		return ErrNotFound
	}
	d.Model, d.InputTokens, d.OutputTokens, d.UpdatedAt = model, d.InputTokens+in, d.OutputTokens+out, now
	for i, p := range ps {
		r.proposals[draftID] = append(r.proposals[draftID], domain.Proposal{
			DraftID: draftID, Rank: startRank + i, Title: p.Title, Author: p.Author, OriginalTitle: p.OriginalTitle, Note: p.Note,
		})
	}
	return nil
}

func (r *fakeRepo) PurgeOlderThan(ctx context.Context, t time.Time) (int64, error) {
	r.purgedAt = append(r.purgedAt, t)
	return 0, nil
}

// seed stores a draft directly.
func seed(r *fakeRepo, col uuid.UUID, status string, count int, created, updated time.Time) uuid.UUID {
	uid := uuid.New()
	d := &domain.Draft{
		ID: uuid.New(), CollectionID: col, Brief: "научная фантастика", Count: count, Status: status,
		PromptVersion: PromptVersion, CreatedBy: &uid, CreatedAt: created, UpdatedAt: updated,
	}
	r.drafts[d.ID] = d
	return d.ID
}

// fakeLookup serves collections by id.
type fakeLookup struct {
	infos map[uuid.UUID]*curatedservice.ManualInfo
	autos map[uuid.UUID]bool
	calls int
}

func newFakeLookup() *fakeLookup {
	return &fakeLookup{infos: map[uuid.UUID]*curatedservice.ManualInfo{}, autos: map[uuid.UUID]bool{}}
}

func (l *fakeLookup) ManualInfo(ctx context.Context, id uuid.UUID) (*curatedservice.ManualInfo, error) {
	l.calls++
	if l.autos[id] {
		return nil, curateddomain.ErrCollectionAutomatic
	}
	info, ok := l.infos[id]
	if !ok {
		return nil, curateddomain.ErrCollectionNotFound
	}
	return info, nil
}

// fakeMatcher records what it was asked and returns a canned preview.
type fakeMatcher struct {
	got     []curatedservice.Proposal
	preview *curatedservice.PastePreview
	err     error
	calls   int
}

func (m *fakeMatcher) MatchProposals(ctx context.Context, id uuid.UUID, ps []curatedservice.Proposal) (*curatedservice.PastePreview, error) {
	m.calls++
	m.got = ps
	if m.err != nil {
		return nil, m.err
	}
	return m.preview, nil
}

// scriptedLLM answers each call with the next scripted step.
type step struct {
	resp llm.StructuredResponse
	err  error
}

type scriptedLLM struct {
	steps    []step
	requests []llm.StructuredRequest
}

func (f *scriptedLLM) Structured(ctx context.Context, req llm.StructuredRequest) (llm.StructuredResponse, error) {
	f.requests = append(f.requests, req)
	i := len(f.requests) - 1
	if i >= len(f.steps) {
		return llm.StructuredResponse{}, fmt.Errorf("unexpected model call %d", i+1)
	}
	return f.steps[i].resp, f.steps[i].err
}

func books(n int, prefix string) llm.StructuredResponse {
	return booksFrom(1, n, prefix)
}

// booksFrom builds an answer with books numbered from..from+n-1.
func booksFrom(from, n int, prefix string) llm.StructuredResponse {
	type b struct {
		Title         string `json:"title"`
		Author        string `json:"author"`
		OriginalTitle string `json:"original_title"`
		Note          string `json:"note"`
	}
	list := make([]b, 0, n)
	for i := 0; i < n; i++ {
		list = append(list, b{Title: fmt.Sprintf("%s %d", prefix, from+i), Author: fmt.Sprintf("Автор %d", from+i)})
	}
	raw, _ := json.Marshal(map[string]any{"books": list})
	return llm.StructuredResponse{JSON: raw, StopReason: llm.StopReasonEndTurn, Model: "claude-sonnet-5-5", InputTokens: 1000, OutputTokens: 500}
}

func stop(reason string) llm.StructuredResponse {
	return llm.StructuredResponse{StopReason: reason, InputTokens: 100, OutputTokens: 10}
}
