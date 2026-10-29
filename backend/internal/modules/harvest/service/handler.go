// Package service implements the async catalog-harvest job handler: it
// decodes a HarvestPayload (a caller-supplied SPARQL query), executes it
// against Wikidata via bookdata.WikidataClient.Query, optionally enriches
// each returned record via an OpenLibrary-shaped bookdata.Client, resolves
// authors by Wikidata QID then name (new authors are pending; rejected
// authors are tombstones and are skipped - see
// adminservice.ResolveOrCreateHarvestAuthors) and genres the same way
// (Wikidata QID first, then name; new genres are pending; rejected genres
// are tombstones and are skipped - see
// adminservice.ResolveOrCreateHarvestGenres and
// docs/features/genre-status.md), and creates books via adminservice.Service.CreateBook.
//
// Books created by this handler are always status = pending: harvested data
// is crowdsourced (Wikidata/Open Library) and needs human review before it
// is shown on the public catalog. This is a hardcoded rule for this
// increment (see business analysis OQ-002), not a per-job option.
//
// The task type this handler is registered under (see
// TaskTypeHarvestWikidataQuery) runs on a dedicated asynq queue (QueueName)
// configured with concurrency 1 on the worker server. bookdata's
// WikidataClient/OpenLibraryClient each throttle outbound requests
// per-client-instance (~1 req/sec); running more than one harvest task at a
// time would multiply the effective outbound request rate beyond that
// design intent (each task constructs/uses its own client instances).
// Serializing harvest-queue processing keeps that throttling guarantee true
// without adding a second, duplicate throttling layer in this package.
// The worker sets Concurrency 1 explicitly (asynq defaults to NumCPU, #191).
//
// Harvest jobs are started manually (POST /admin/harvest, cmd/harvest) or by
// the scheduled harvest (see schedule.go): the worker's scheduler enqueues a
// TaskTypeHarvestScheduleTick task on its own queue (ScheduleQueueName),
// whose handler picks a batch of flagged authors and queues one ordinary
// harvest job for them. HarvestPayload records the trigger and author QIDs
// for the admin jobs list; processing is identical for both triggers, so
// scheduled jobs also create only pending books and never publish anything.
//
// This package does not talk to the admin repository directly - it only
// calls exported adminservice.Service methods, so it does not need to know
// how author/genre/book validation or persistence works internally.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/pkg/bookdata"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// TaskTypeHarvestWikidataQuery is the asynq task type for a Wikidata bulk
// harvest job. Defined once here and imported by both cmd/harvest (enqueue)
// and cmd/worker (register handler) so the two can't drift.
const TaskTypeHarvestWikidataQuery = "harvest:wikidata_query"

// QueueName is the dedicated asynq queue harvest tasks are enqueued/
// processed on. See the package doc comment for why this queue must be
// configured with concurrency 1 on the worker server.
const QueueName = "harvest"

// wikidataSourceName is the external_source value stamped on books created
// from Wikidata records, and the source scope for the idempotency
// (external_source, external_id) lookup.
const wikidataSourceName = "wikidata"

// HarvestPayload is the JSON task payload for TaskTypeHarvestWikidataQuery.
// Trigger and AuthorQIDs are job metadata for the admin jobs list (how the
// job was started and which author QIDs it covers); HandleTask ignores
// them. Both are absent in jobs queued before they existed and in
// cmd/harvest jobs (reported as manual with no authors).
type HarvestPayload struct {
	SPARQLQuery string   `json:"sparql_query"`
	Trigger     string   `json:"trigger,omitempty"`
	AuthorQIDs  []string `json:"author_qids,omitempty"`
}

// Harvest job triggers (HarvestPayload.Trigger).
const (
	TriggerManual    = "manual"
	TriggerScheduled = "scheduled"
)

// HarvestMeta is the optional job metadata NewHarvestTask stores in the
// payload.
type HarvestMeta struct {
	Trigger    string
	AuthorQIDs []string
}

// HarvestResult is the JSON result written via task.ResultWriter().Write.
// It is written incrementally while the task is still running (see
// reportProgress/runHarvest) as well as once more, finally, after the
// handler returns - retrievable at any point via asynq.Inspector.GetTaskInfo
// (TaskInfo.Result), including while TaskInfo.State == asynq.TaskStateActive,
// not only once State == asynq.TaskStateCompleted.
//
// Confirmed against asynq v0.25.1 source (internal/rdb/rdb.go's WriteResult,
// internal/rdb/inspect.go's GetTaskInfo Lua script): both operate on the
// same per-task Redis hash key regardless of task state, and
// ResultWriter.Write has no restriction on being called more than once per
// task - so writing progress mid-task and reading it back via GetTaskInfo
// while the task is still active is safe.
type HarvestResult struct {
	Processed      int                    `json:"processed"`
	Total          int                    `json:"total"`
	Created        int                    `json:"created"`
	Skipped        int                    `json:"skipped_existing"`
	Errors         []HarvestRecordError   `json:"errors"`
	SkippedRecords []HarvestSkippedRecord `json:"skipped_records"`
	// AuthorsEnriched / AuthorEnrichmentErrors report the end-of-job
	// Wikidata enrichment of the authors this job created
	// (docs/features/author-wikidata-enrichment.md rule 4b). Set only once
	// that step ran (nil while the record loop is running, in results of
	// older jobs, and when no enrichment source is configured).
	AuthorsEnriched        *int `json:"authors_enriched,omitempty"`
	AuthorEnrichmentErrors *int `json:"author_enrichment_errors,omitempty"`
	// AdaptationsAdded / AdaptationSyncErrors report the end-of-job sync of
	// screen adaptations of the books this job created (openspec
	// catalog-import/wikidata-book-harvest). Set only once that step ran
	// (nil without a configured syncer and in results of older jobs).
	AdaptationsAdded     *int `json:"adaptations_added,omitempty"`
	AdaptationSyncErrors *int `json:"adaptation_sync_errors,omitempty"`
	// AuthorFilmsAdded / AuthorFilmSyncErrors report the end-of-job sync of
	// films about the authors this job created (openspec
	// catalog-import/wikidata-book-harvest). Set only once that step ran
	// (nil without a configured syncer and in results of older jobs).
	AuthorFilmsAdded     *int `json:"author_films_added,omitempty"`
	AuthorFilmSyncErrors *int `json:"author_film_sync_errors,omitempty"`
}

// HarvestRecordError describes a single record's failure. It does not abort
// the rest of the job - the handler processes every record returned by the
// query and collects per-record failures here.
type HarvestRecordError struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// HarvestSkippedRecord describes a single record the harvest run skipped
// because a book with the same (external_source, external_id) already
// exists in the catalog - today the only possible skip cause (see
// processRecord's idempotency check). No Message field: unlike
// HarvestRecordError, there is exactly one reason, so the frontend renders
// a fixed label rather than this type carrying a per-record message.
type HarvestSkippedRecord struct {
	Index int    `json:"index"`
	Title string `json:"title"`
}

// WikidataQuerier is the subset of bookdata.WikidataClient this handler
// depends on. It is satisfied by *bookdata.WikidataClient; tests supply a
// fake so the handler's core logic is testable without a real network call.
type WikidataQuerier interface {
	Query(ctx context.Context, sparql string) ([]bookdata.ExternalBookRecord, error)
}

// Handler orchestrates a Wikidata harvest job: query, per-record enrichment
// + author/genre resolution + book creation, result aggregation.
type Handler struct {
	wikidata WikidataQuerier
	enrich   bookdata.Client
	admin    *adminservice.Service
	syncer   AdaptationSyncer
	// filmSyncer syncs films about the authors a job created.
	filmSyncer AuthorFilmSyncer
	// fantlab finds covers on FantLab (nil = step off); log reports the
	// per-job request cap being hit.
	fantlab FantLabCoverFinder
	log     *zap.Logger
}

// MaxFantLabRequestsPerJob caps the FantLab requests one harvest job makes.
const MaxFantLabRequestsPerJob = 250

// FantLabCoverFinder finds a cover link on FantLab by the Wikidata FantLab
// work IDs and the book's title (bookdata.FantLabClient satisfies it).
type FantLabCoverFinder interface {
	FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (coverURL, usedID string, err error)
}

// fantLabBudgeter is implemented by finders with a request budget
// (bookdata.FantLabClient); the handler resets it at the start of every job.
type fantLabBudgeter interface {
	SetRequestBudget(n int)
}

// budgetCountingFantLab counts the works skipped because the job's request
// budget was spent.
type budgetCountingFantLab struct {
	FantLabCoverFinder
	skipped int
}

func (b *budgetCountingFantLab) FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (string, string, error) {
	u, id, err := b.FantLabCoverFinder.FindSelfTitledEditionCover(ctx, title, workIDs)
	if errors.Is(err, bookdata.ErrBudgetExhausted) {
		b.skipped++
	}
	return u, id, err
}

// AdaptationSyncer syncs screen adaptations of the books a job created.
// Declared here so the harvest package does not import the screenworks
// module; satisfied by *screenworks/service.Service.
type AdaptationSyncer interface {
	// SyncCreatedBooks returns the number of links created and the number
	// of books whose sync ended in an error. It never fails the job.
	SyncCreatedBooks(ctx context.Context, bookIDs []uuid.UUID) (added, failed int)
	HasSyncSource() bool
}

// AuthorFilmSyncer syncs films about the authors a job created. Declared here
// so the harvest package does not import the screenworks module; satisfied by
// *screenworks/service.Service.
type AuthorFilmSyncer interface {
	// SyncCreatedAuthors returns the number of links created and the number
	// of authors whose sync ended in an error. It never fails the job.
	SyncCreatedAuthors(ctx context.Context, authorIDs []uuid.UUID) (added, failed int)
	HasAuthorSyncSource() bool
}

// HandlerOption configures optional Handler dependencies.
type HandlerOption func(*Handler)

// WithAdaptationSyncer enables the end-of-job adaptation sync. Without it
// the step is skipped and the result carries no adaptation counters.
func WithAdaptationSyncer(s AdaptationSyncer) HandlerOption {
	return func(h *Handler) { h.syncer = s }
}

// WithAuthorFilmSyncer enables the end-of-job sync of films about the authors
// the job created. Without it the step is skipped and the result carries no
// author film counters.
func WithAuthorFilmSyncer(s AuthorFilmSyncer) HandlerOption {
	return func(h *Handler) { h.filmSyncer = s }
}

// WithFantLabCovers enables the FantLab cover step of enrichment (after the
// Open Library ID cover, before the title search). Without it the step is
// off. Finders with a request budget get MaxFantLabRequestsPerJob per job.
func WithFantLabCovers(f FantLabCoverFinder) HandlerOption {
	return func(h *Handler) { h.fantlab = f }
}

// WithLogger sets the logger for job-level notices (default: none).
func WithLogger(l *zap.Logger) HandlerOption {
	return func(h *Handler) { h.log = l }
}

// NewHandler builds a Handler. enrich may be nil to disable enrichment
// entirely (records are then created using only Wikidata's own fields).
func NewHandler(wikidata WikidataQuerier, enrich bookdata.Client, admin *adminservice.Service, opts ...HandlerOption) *Handler {
	h := &Handler{wikidata: wikidata, enrich: enrich, admin: admin}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HandleTask is the asynq.HandlerFunc-compatible entry point, registered on
// an asynq.ServeMux under TaskTypeHarvestWikidataQuery.
func (h *Handler) HandleTask(ctx context.Context, t *asynq.Task) error {
	var payload HarvestPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("harvest: decode payload: %v: %w", err, asynq.SkipRetry)
	}
	if strings.TrimSpace(payload.SPARQLQuery) == "" {
		return fmt.Errorf("harvest: sparql_query is required: %w", asynq.SkipRetry)
	}

	result, err := h.runHarvest(ctx, payload, t.ResultWriter())
	if err != nil {
		if errors.Is(err, bookdata.ErrInvalidQuery) {
			return fmt.Errorf("harvest: invalid sparql query: %v: %w", err, asynq.SkipRetry)
		}
		// Rate-limited / source-unavailable / transport errors: let asynq
		// retry with backoff per the task's configured MaxRetry.
		return fmt.Errorf("harvest: wikidata query: %w", err)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("harvest: marshal result: %w", err)
	}
	if _, err := t.ResultWriter().Write(resultBytes); err != nil {
		return fmt.Errorf("harvest: write result: %w", err)
	}
	return nil
}

// progressWriter is the subset of *asynq.ResultWriter (t.ResultWriter())
// this package depends on for mid-task progress reporting. Satisfied by
// *asynq.ResultWriter in production; tests supply a fake or nil (a nil
// progressWriter is a safe no-op - see reportProgress).
type progressWriter interface {
	Write(data []byte) (int, error)
}

// reportProgress marshals result to JSON and writes it via progress, for
// mid-task visibility of a still-running job (see HarvestResult's doc
// comment for why this is safe to read back via GetTaskInfo while the task
// is still active). Progress reporting is best-effort: a nil progress (no
// live ResultWriter, e.g. in tests) is a no-op, and any marshal/write error
// is swallowed - it must never fail the harvest job itself (see
// docs/features/harvest-progress.md Business Rules).
func reportProgress(progress progressWriter, result HarvestResult) {
	if progress == nil {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_, _ = progress.Write(data)
}

// runHarvest executes the query and per-record processing loop, returning
// the aggregated result. It is separated from HandleTask so it can be unit
// tested without a real asynq.Task/ResultWriter (only tasks dequeued by a
// live asynq.Server have a usable ResultWriter; asynq.NewTask-constructed
// tasks do not, so exercising the full success path through HandleTask
// itself is not possible in a unit test). progress may be nil (see
// reportProgress).
func (h *Handler) runHarvest(ctx context.Context, payload HarvestPayload, progress progressWriter) (HarvestResult, error) {
	records, err := h.wikidata.Query(ctx, payload.SPARQLQuery)
	if err != nil {
		return HarvestResult{}, err
	}

	result := HarvestResult{Total: len(records), Errors: []HarvestRecordError{}, SkippedRecords: []HarvestSkippedRecord{}}
	reportProgress(progress, result)

	// FantLab: a fresh request budget per job; the counter reports how many
	// works the cap left without a lookup.
	var fantlab FantLabCoverFinder
	var counting *budgetCountingFantLab
	if h.fantlab != nil {
		if bg, ok := h.fantlab.(fantLabBudgeter); ok {
			bg.SetRequestBudget(MaxFantLabRequestsPerJob)
		}
		counting = &budgetCountingFantLab{FantLabCoverFinder: h.fantlab}
		fantlab = counting
	}

	var createdAuthors, createdBooks []uuid.UUID
	for i, rec := range records {
		outcome, created, bookID, procErr := processHarvestRecord(ctx, rec, h.enrich, fantlab, h.admin)
		createdAuthors = append(createdAuthors, created...)
		if bookID != uuid.Nil {
			createdBooks = append(createdBooks, bookID)
		}
		if procErr != nil {
			result.Errors = append(result.Errors, HarvestRecordError{
				Index:   i,
				Title:   rec.Title,
				Message: procErr.Error(),
			})
		} else {
			switch outcome {
			case outcomeCreated:
				result.Created++
			case outcomeSkipped:
				result.Skipped++
				result.SkippedRecords = append(result.SkippedRecords, HarvestSkippedRecord{
					Index: i,
					Title: rec.Title,
				})
			}
		}
		result.Processed++
		reportProgress(progress, result)
	}

	if counting != nil && counting.skipped > 0 && h.log != nil {
		h.log.Info("harvest: FantLab request cap reached, works left without a FantLab cover lookup",
			zap.Int("cap", MaxFantLabRequestsPerJob), zap.Int("skipped_works", counting.skipped))
	}

	if h.admin.HasAuthorEnrichmentSource() {
		enriched, failed := enrichCreatedAuthors(ctx, h.admin, createdAuthors)
		result.AuthorsEnriched = &enriched
		result.AuthorEnrichmentErrors = &failed
		reportProgress(progress, result)
	}

	// Best-effort: after the author enrichment, sync screen adaptations of
	// the books this job created.
	if h.syncer != nil && h.syncer.HasSyncSource() {
		added, failed := 0, 0
		if len(createdBooks) > 0 {
			added, failed = h.syncer.SyncCreatedBooks(ctx, createdBooks)
		}
		result.AdaptationsAdded = &added
		result.AdaptationSyncErrors = &failed
		reportProgress(progress, result)
	}

	// Best-effort: sync films about the authors this job created.
	if h.filmSyncer != nil && h.filmSyncer.HasAuthorSyncSource() {
		added, failed := 0, 0
		if len(createdAuthors) > 0 {
			added, failed = h.filmSyncer.SyncCreatedAuthors(ctx, createdAuthors)
		}
		result.AuthorFilmsAdded = &added
		result.AuthorFilmSyncErrors = &failed
		reportProgress(progress, result)
	}

	return result, nil
}

// enrichCreatedAuthors enriches the authors created by this job once,
// after the record loop. Best-effort: a failure is only counted (the admin
// service logs it) and never changes the task's outcome.
func enrichCreatedAuthors(ctx context.Context, admin *adminservice.Service, ids []uuid.UUID) (enriched, failed int) {
	if len(ids) == 0 {
		return 0, 0
	}
	report, err := admin.EnrichHarvestAuthors(ctx, ids)
	if report == nil {
		if err != nil {
			return 0, len(ids)
		}
		return 0, 0
	}
	for _, item := range report.Items {
		if item.Outcome == adminservice.EnrichOutcomeError {
			failed++
		}
	}
	return report.Enriched, failed
}

type recordOutcome int

const (
	outcomeCreated recordOutcome = iota
	outcomeSkipped
)

// processRecord handles a single Wikidata record: idempotency skip-check,
// optional enrichment, author/genre resolution, and book creation. It
// returns a non-nil error only for a *this record's* failure - callers
// should record it and keep processing the rest of the batch.
//
// It is a package-level function (not a Handler method) so it can be unit
// tested directly with fakes, without constructing a Handler/asynq.Task.
func processRecord(ctx context.Context, rec bookdata.ExternalBookRecord, enrich bookdata.Client, admin *adminservice.Service) (recordOutcome, error) {
	outcome, _, _, err := processHarvestRecord(ctx, rec, enrich, nil, admin)
	return outcome, err
}

// processHarvestRecord is processRecord that also returns the authors
// created with a Wikidata QID while resolving this record's authors -
// reported even when the book creation then fails, so the job still
// enriches them (docs/features/author-wikidata-enrichment.md rule 4b). It
// also returns the id of the created book (uuid.Nil when none was created).
func processHarvestRecord(ctx context.Context, rec bookdata.ExternalBookRecord, enrich bookdata.Client, fantlab FantLabCoverFinder, admin *adminservice.Service) (recordOutcome, []uuid.UUID, uuid.UUID, error) {
	sourceID := strings.TrimSpace(rec.SourceID)
	if sourceID == "" {
		return 0, nil, uuid.Nil, errors.New("record has no source id")
	}

	_, err := admin.FindBookByExternalID(ctx, wikidataSourceName, sourceID)
	if err == nil {
		return outcomeSkipped, nil, uuid.Nil, nil
	}
	if !errors.Is(err, adminservice.ErrBookNotFound) {
		return 0, nil, uuid.Nil, fmt.Errorf("check existing book: %w", err)
	}

	enriched := rec
	if enrich != nil {
		enrichRecord(ctx, &enriched, enrich, fantlab)
	}

	resolution, err := admin.ResolveHarvestAuthorRefs(ctx, harvestAuthorRefs(enriched))
	if err != nil {
		return 0, nil, uuid.Nil, fmt.Errorf("resolve authors: %w", err)
	}
	authorIDs, created := resolution.IDs, resolution.CreatedWithQID
	genreIDs, err := admin.ResolveOrCreateHarvestGenres(ctx, harvestGenreRefs(enriched))
	if err != nil {
		return 0, created, uuid.Nil, fmt.Errorf("resolve genres: %w", err)
	}

	// Language is required by validateBookInput (ErrInvalidLanguage on
	// empty), but Wikidata's P407 "language of work" property is sparsely
	// populated, so enriched.Language is frequently empty. Default to "ru"
	// in that case, mirroring import.go's defaultImportLanguage precedent -
	// without this, most harvested records would fail validation for an
	// unrelated reason. When Wikidata does supply a P407-derived language,
	// it is preserved as-is (e.g. "en" for a Russian-labeled translation of
	// an English-original work).
	language := strings.TrimSpace(enriched.Language)
	if language == "" {
		language = "ru"
	}

	externalSource := wikidataSourceName
	status := catalogDomain.BookStatusPending
	input := adminservice.BookInput{
		Title:          enriched.Title,
		Description:    enriched.Description,
		CoverURL:       enriched.CoverURL,
		ISBN:           enriched.ISBN,
		PublishedYear:  enriched.PublishedYear,
		PageCount:      enriched.PageCount,
		Language:       language,
		AuthorIDs:      authorIDs,
		GenreIDs:       genreIDs,
		Status:         &status,
		ExternalSource: &externalSource,
		ExternalID:     &sourceID,
		// The title comes only from the Wikidata "ru" label
		// (FILTER(LANG(?itemLabel) = "ru") in bookdata/wikidata.go), so it is
		// the official Russian title by definition even when written in
		// Latin script (e.g. «S.N.U.F.F.»). Only the title-language check is
		// lifted; the description check still applies.
		// See docs/features/latin-book-titles.md.
		AllowNonRussianTitle: true,
	}

	book, err := admin.CreateBook(ctx, input)
	if err != nil {
		return 0, created, uuid.Nil, fmt.Errorf("create book: %w", err)
	}
	return outcomeCreated, created, book.ID, nil
}

// harvestAuthorRefs maps a record's authors to resolver refs: the paired
// Wikidata AuthorRefs (name + QID) when present, otherwise the plain
// Authors names as QID-less refs (e.g. records without a parsed pairing).
func harvestAuthorRefs(rec bookdata.ExternalBookRecord) []adminservice.AuthorRef {
	if len(rec.AuthorRefs) > 0 {
		refs := make([]adminservice.AuthorRef, 0, len(rec.AuthorRefs))
		for _, a := range rec.AuthorRefs {
			refs = append(refs, adminservice.AuthorRef{Name: a.Name, QID: a.QID})
		}
		return refs
	}
	refs := make([]adminservice.AuthorRef, 0, len(rec.Authors))
	for _, name := range rec.Authors {
		refs = append(refs, adminservice.AuthorRef{Name: name})
	}
	return refs
}

// harvestGenreRefs maps a record's genres to resolver refs: the paired
// Wikidata GenreRefs (name + QID) when present, otherwise the plain Genres
// names as QID-less refs. Enrichment never touches genres.
func harvestGenreRefs(rec bookdata.ExternalBookRecord) []adminservice.GenreRef {
	if len(rec.GenreRefs) > 0 {
		refs := make([]adminservice.GenreRef, 0, len(rec.GenreRefs))
		for _, g := range rec.GenreRefs {
			refs = append(refs, adminservice.GenreRef{Name: g.Name, QID: g.QID})
		}
		return refs
	}
	refs := make([]adminservice.GenreRef, 0, len(rec.Genres))
	for _, name := range rec.Genres {
		refs = append(refs, adminservice.GenreRef{Name: name})
	}
	return refs
}

// titleAuthorMatcher is implemented by enrichment clients that can do a
// strict title+author search (bookdata.OpenLibraryClient).
type titleAuthorMatcher interface {
	FindMatchByTitleAuthor(ctx context.Context, q bookdata.TitleAuthorQuery) (*bookdata.ExternalBookRecord, error)
}

// buildTitleAuthorQuery builds the strict Open Library query for a harvested
// record: the Russian title and first author are searched; the English labels
// and the other authors are used only for matching.
func buildTitleAuthorQuery(rec *bookdata.ExternalBookRecord) bookdata.TitleAuthorQuery {
	q := bookdata.TitleAuthorQuery{Title: rec.Title}
	if en := strings.TrimSpace(rec.TitleEn); en != "" {
		q.AltTitles = []string{en}
	}
	if len(rec.AuthorRefs) > 0 {
		for _, ref := range rec.AuthorRefs {
			q.Authors = append(q.Authors, ref.Name)
		}
		for _, ref := range rec.AuthorRefs {
			if en := strings.TrimSpace(ref.NameEn); en != "" {
				q.Authors = append(q.Authors, en)
			}
		}
	} else {
		q.Authors = append(q.Authors, rec.Authors...)
	}
	return q
}

// enrichRecord fills CoverURL, Description, PageCount and ISBN from enrich
// (Open Library, in production) but only where rec's own value is nil -
// Wikidata's own values are never overwritten. Enrichment lookup failures
// (not found, rate limited, source unavailable) are not fatal to the
// record: the book is still created using whatever Wikidata provided.
// Description is additionally filtered to (heuristically) Russian text
// before being copied - see the inline comment below for the rationale.
func enrichRecord(ctx context.Context, rec *bookdata.ExternalBookRecord, enrich bookdata.Client, fantlab FantLabCoverFinder) {
	if rec.CoverURL != nil && rec.Description != nil && rec.PageCount != nil && rec.ISBN != nil {
		return
	}

	var isbnFound bool
	if rec.ISBN != nil {
		if found, err := enrich.FindByISBN(ctx, *rec.ISBN); err == nil && found != nil {
			isbnFound = true
			mergeEnrichment(rec, found)
		}
	}

	// Cover by Open Library ID (Wikidata P648): an exact lookup, tried before
	// the title search. Any failure is ignored. The cover may belong to an
	// edition in another language (accepted, issue #688). Only the cover is
	// taken from this step.
	if rec.CoverURL == nil && len(rec.OpenLibraryIDs) > 0 {
		if finder, ok := enrich.(openLibraryCoverFinder); ok {
			if coverURL, _, err := finder.FindCoverByOpenLibraryIDs(ctx, rec.OpenLibraryIDs); err == nil && coverURL != "" {
				rec.CoverURL = &coverURL
			}
		}
	}

	// Cover from a FantLab edition named exactly like the book (Wikidata
	// P7439): tried after the Open Library ID and before the title search.
	// Only the cover is taken; any failure is ignored.
	if rec.CoverURL == nil && len(rec.FantLabWorkIDs) > 0 && fantlab != nil && strings.TrimSpace(rec.Title) != "" {
		if coverURL, _, err := fantlab.FindSelfTitledEditionCover(ctx, rec.Title, rec.FantLabWorkIDs); err == nil && coverURL != "" {
			rec.CoverURL = &coverURL
		}
	}

	if !isbnFound && strings.TrimSpace(rec.Title) != "" {
		// Strict title+author match only: a loose "first search result" is
		// never accepted (it attached unrelated covers/ISBNs, issue #687).
		if matcher, ok := enrich.(titleAuthorMatcher); ok {
			if found, err := matcher.FindMatchByTitleAuthor(ctx, buildTitleAuthorQuery(rec)); err == nil && found != nil {
				mergeEnrichment(rec, found)
			}
		}
	}
}

// openLibraryCoverFinder is implemented by enrichment clients that can look a
// cover up by Open Library work/edition ID (bookdata.OpenLibraryClient).
type openLibraryCoverFinder interface {
	FindCoverByOpenLibraryIDs(ctx context.Context, ids []string) (coverURL string, usedID string, err error)
}

// mergeEnrichment copies fields from an enrichment record into rec, only where
// rec's own value is nil.
func mergeEnrichment(rec, enrichedRec *bookdata.ExternalBookRecord) {
	if rec.CoverURL == nil {
		rec.CoverURL = enrichedRec.CoverURL
	}
	// Only copy Open Library's Description if it is (heuristically) Russian
	// text. This is a silent drop, not a hard rejection of the whole
	// record: Open Library's excerpt has no language filtering of its own
	// today, so a non-Russian excerpt is simply left out (rec.Description
	// stays nil, since Wikidata's own SPARQL parsing never sets Description
	// itself) rather than failing an otherwise-good, correctly-Russian-
	// titled book over an optional enrichment field. Harvest titles are
	// exempt from the title-language check altogether (processRecord sets
	// AllowNonRussianTitle: the title is the Wikidata "ru" label by
	// construction) - see docs/features/latin-book-titles.md and
	// docs/features/russian-only-content.md Business Rules.
	if rec.Description == nil && enrichedRec.Description != nil && adminservice.IsRussianText(*enrichedRec.Description) {
		rec.Description = enrichedRec.Description
	}
	if rec.PageCount == nil {
		rec.PageCount = enrichedRec.PageCount
	}
	if rec.ISBN == nil {
		rec.ISBN = enrichedRec.ISBN
	}
}

// NewHarvestTask builds the asynq.Task for a Wikidata harvest job, with the
// retry/timeout/retention/queue options fixed by the business rules for
// this task type (see docs/features/async-harvest-queue.md Business
// Rules). Both cmd/harvest (enqueue) and tests use this so those options
// can't drift out of sync.
//
// meta is optional (at most the first value is used): it records the
// trigger and author QIDs in the payload; omitted, the payload carries the
// query only (cmd/harvest).
func NewHarvestTask(sparqlQuery string, meta ...HarvestMeta) (*asynq.Task, error) {
	p := HarvestPayload{SPARQLQuery: sparqlQuery}
	if len(meta) > 0 {
		p.Trigger = meta[0].Trigger
		p.AuthorQIDs = meta[0].AuthorQIDs
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("harvest: marshal payload: %w", err)
	}
	return asynq.NewTask(TaskTypeHarvestWikidataQuery, payload, HarvestTaskOptions()...), nil
}

// HarvestTaskOptions are the fixed options of every harvest task (manual,
// scheduled or cmd/harvest), applied by NewHarvestTask.
func HarvestTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(3),
		asynq.Timeout(30 * time.Minute),
		asynq.Retention(24 * time.Hour),
	}
}
