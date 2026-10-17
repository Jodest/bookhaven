package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/modules/aigenres/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// minCallInterval is the minimum pause between two model calls.
	minCallInterval = time.Second
	// maxOutputTokens caps one answer (up to 50 items).
	maxOutputTokens = 8192
)

// Analyzer runs one analysis: it reads the genre catalog, asks the model one
// batch at a time and stores the validated suggestions. It never writes to
// genres, books or collections (it has no GenreApplier).
type Analyzer struct {
	client    llm.Client
	repo      Repository
	catalog   GenreCatalog
	model     string
	batchSize int
	log       *zap.Logger

	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	lastCall time.Time
}

// NewAnalyzer builds an Analyzer. batchSize is the number of non-anchor
// genres per request (config AI_GENRES_BATCH). A nil logger is replaced by a
// no-op logger.
func NewAnalyzer(client llm.Client, repo Repository, catalog GenreCatalog, model string, batchSize int, log *zap.Logger) *Analyzer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Analyzer{
		client: client, repo: repo, catalog: catalog, model: model, batchSize: batchSize, log: log,
		now: time.Now, sleep: sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (a *Analyzer) pace(ctx context.Context) error {
	if a.lastCall.IsZero() {
		return nil
	}
	if wait := minCallInterval - a.now().Sub(a.lastCall); wait > 0 {
		return a.sleep(ctx, wait)
	}
	return nil
}

// fail ends the run as failed with code. It uses a context that survives the
// cancellation of ctx, so a timed-out run is still closed.
func (a *Analyzer) fail(ctx context.Context, runID uuid.UUID, code string) {
	c := code
	if err := a.repo.FinishRun(context.WithoutCancel(ctx), runID, domain.RunFailed, &c, a.now().UTC()); err != nil {
		a.log.Error("AI genres: mark run failed", zap.String("run_id", runID.String()), zap.Error(err))
	}
}

// Run computes the queued run. A temporary or other model failure ends the
// run as failed (suggestions of earlier batches stay) and returns nil, so
// asynq does not retry; the next scheduled tick or a manual run continues.
// A database failure is returned after the run is marked failed.
func (a *Analyzer) Run(ctx context.Context, runID uuid.UUID) error {
	run, err := a.repo.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("aigenres run: %w", err)
	}
	if run.Status != domain.RunQueued {
		return nil
	}
	now := a.now().UTC()

	if n, err := a.repo.PurgeRunsOlderThan(ctx, now.Add(-domain.RunRetention)); err != nil {
		a.log.Warn("AI genres: purge old runs", zap.Error(err))
	} else if n > 0 {
		a.log.Info("AI genres: purged old runs", zap.Int64("runs", n))
	}

	if err := a.sweepStale(ctx); err != nil {
		a.fail(ctx, runID, domain.ErrorInternal)
		return fmt.Errorf("aigenres run: stale sweep: %w", err)
	}

	cands, err := a.catalog.ListGenreAnalysisCandidates(ctx)
	if err != nil {
		a.fail(ctx, runID, domain.ErrorInternal)
		return fmt.Errorf("aigenres run: candidates: %w", err)
	}
	genres := make([]catalogDomain.Genre, 0, len(cands))
	for _, c := range cands {
		genres = append(genres, c.Genre)
	}
	allPairs := a.catalog.GenreHintPairs(ctx, genres)
	plan := planBatches(cands, allPairs, a.batchSize, domain.MaxBatches)

	if err := a.repo.StartRun(ctx, runID, a.model, Fingerprint(cands), len(plan.Batches), now); err != nil {
		a.fail(ctx, runID, domain.ErrorInternal)
		return fmt.Errorf("aigenres run: %w", err)
	}

	for i, batch := range plan.Batches {
		res, stop, err := a.runBatch(ctx, runID, plan.Anchors, batch, allPairs)
		if err != nil {
			a.fail(ctx, runID, domain.ErrorInternal)
			return fmt.Errorf("aigenres run: batch %d: %w", i+1, err)
		}
		if stop != "" {
			a.fail(ctx, runID, stop)
			return nil
		}
		if err := a.repo.AddBatchResult(ctx, runID, res); err != nil {
			a.fail(ctx, runID, domain.ErrorInternal)
			return fmt.Errorf("aigenres run: %w", err)
		}
	}
	if err := a.repo.FinishRun(ctx, runID, domain.RunDone, nil, a.now().UTC()); err != nil {
		return fmt.Errorf("aigenres run: %w", err)
	}
	a.log.Info("AI genres run finished", zap.String("run_id", runID.String()),
		zap.Int("candidates", len(cands)), zap.Int("batches", len(plan.Batches)))
	return nil
}

// sweepStale marks open suggestions whose genres changed as stale.
func (a *Analyzer) sweepStale(ctx context.Context) error {
	open, err := a.repo.ListOpenSuggestions(ctx)
	if err != nil || len(open) == 0 {
		return err
	}
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	for _, s := range open {
		for _, g := range s.Genres {
			if _, ok := seen[g.GenreID]; !ok {
				seen[g.GenreID] = struct{}{}
				ids = append(ids, g.GenreID)
			}
		}
	}
	states, err := a.catalog.GenresByIDs(ctx, ids)
	if err != nil {
		return err
	}
	var staleIDs []uuid.UUID
	for i := range open {
		if suggestionStale(&open[i], states) {
			staleIDs = append(staleIDs, open[i].ID)
		}
	}
	if n, err := a.repo.MarkStale(ctx, staleIDs); err != nil {
		return err
	} else if n > 0 {
		a.log.Info("AI genres: suggestions marked stale", zap.Int64("count", n))
	}
	return nil
}

// runBatch sends one request and stores its validated suggestions. stop is a
// failure code that ends the whole run (a model call failure); a refusal, a
// truncated or an invalid answer is recorded in the result and the run goes
// on. A non-nil err is a database failure.
func (a *Analyzer) runBatch(ctx context.Context, runID uuid.UUID, anchors, batch []candidate, allPairs []adminservice.GenreHintPair) (res repository.BatchResult, stop string, err error) {
	// Samples for the non-anchor genres of the batch only.
	ids := make([]uuid.UUID, 0, len(batch))
	for _, c := range batch {
		ids = append(ids, c.Genre.ID)
	}
	samples, err := a.catalog.ListGenreSampleBooks(ctx, ids)
	if err != nil {
		return res, "", err
	}

	genres := make([]requestGenre, 0, len(anchors)+len(batch))
	refs := make(map[string]refGenre, len(anchors)+len(batch))
	refOf := make(map[uuid.UUID]string, len(anchors)+len(batch))
	add := func(c candidate, anchor bool) {
		ref := fmt.Sprintf("g%d", len(genres)+1)
		g := requestGenre{Ref: ref, Candidate: c, Anchor: anchor}
		if !anchor {
			for _, b := range samples[c.Genre.ID] {
				if len(g.SampleText) >= domain.SampleBooksPerGenre {
					break
				}
				g.SampleText = append(g.SampleText, truncateRunes(b.Title, domain.SampleTitleRunes))
			}
		}
		genres = append(genres, g)
		refs[ref] = refGenre{ID: c.Genre.ID, Name: c.Genre.Name, Books: c.BooksTotal}
		refOf[c.Genre.ID] = ref
	}
	for _, c := range anchors {
		add(c, true)
	}
	for _, c := range batch {
		add(c, false)
	}

	var hints [][3]string
	for _, p := range allPairs {
		ra, okA := refOf[p.A]
		rb, okB := refOf[p.B]
		if !okA || !okB {
			continue
		}
		hints = append(hints, [3]string{ra, rb, p.Match})
		if len(hints) >= maxHintPairs {
			break
		}
	}

	user, err := buildUserMessage(genres, hints)
	if err != nil {
		return res, "", err
	}
	if err := a.pace(ctx); err != nil {
		a.log.Warn("AI genres: stopped while waiting", zap.Error(err))
		return res, domain.ErrorTransient, nil
	}
	a.lastCall = a.now()
	resp, callErr := a.client.Structured(ctx, llm.StructuredRequest{
		System:    SystemPrompt,
		User:      user,
		Schema:    OutputSchema(),
		MaxTokens: maxOutputTokens,
	})
	if callErr != nil {
		if errors.Is(callErr, llm.ErrTransient) {
			a.log.Warn("AI genres: transient model failure, run stopped", zap.String("run_id", runID.String()), zap.Error(callErr))
			return res, domain.ErrorTransient, nil
		}
		// Not transient: a config or code bug (e.g. a 400). Nothing stored.
		a.log.Error("AI genres: model call failed, run stopped", zap.String("run_id", runID.String()), zap.Error(callErr))
		return res, domain.ErrorModelError, nil
	}

	model := resp.Model
	if model == "" {
		model = a.model
	}
	res.InputTokens, res.OutputTokens = resp.InputTokens, resp.OutputTokens
	out := Validate(resp, refs, model)
	if out.Code != "" {
		code := out.Code
		res.ErrorCode = &code
		a.log.Warn("AI genres: batch stored nothing", zap.String("run_id", runID.String()), zap.String("code", code),
			zap.String("stop_reason", resp.StopReason))
		return res, "", nil
	}

	created, err := a.repo.InsertSuggestions(ctx, out.Items, runID, a.now().UTC())
	if err != nil {
		return res, "", err
	}
	res.SuggestionsCreated = created
	// Counts only, never names or titles.
	a.log.Info("AI genres batch stored",
		zap.String("run_id", runID.String()),
		zap.Int("genres", len(genres)),
		zap.Int("proposed", len(out.Items)),
		zap.Int("dropped", out.Dropped),
		zap.Int("created", created),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
		zap.Int("cache_read_tokens", resp.CacheReadTokens))
	return res, "", nil
}
