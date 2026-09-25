// Package service implements the async catalog-harvest job handler: it
// decodes a HarvestPayload (a caller-supplied SPARQL query), executes it
// against Wikidata via bookdata.WikidataClient.Query, optionally enriches
// each returned record via an OpenLibrary-shaped bookdata.Client, resolves
// authors/genres by name (autocreate on miss, see
// adminservice.ResolveOrCreateAuthorIDsByNames/ResolveOrCreateGenreIDsByNames),
// and creates books via adminservice.Service.CreateBook.
//
// Books created by this handler are always is_hidden = true: harvested data
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
	"github.com/hibiken/asynq"
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
type HarvestPayload struct {
	SPARQLQuery string `json:"sparql_query"`
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
	Processed int                  `json:"processed"`
	Total     int                  `json:"total"`
	Created   int                  `json:"created"`
	Skipped   int                  `json:"skipped_existing"`
	Errors    []HarvestRecordError `json:"errors"`
}

// HarvestRecordError describes a single record's failure. It does not abort
// the rest of the job - the handler processes every record returned by the
// query and collects per-record failures here.
type HarvestRecordError struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Message string `json:"message"`
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
}

// NewHandler builds a Handler. enrich may be nil to disable enrichment
// entirely (records are then created using only Wikidata's own fields).
func NewHandler(wikidata WikidataQuerier, enrich bookdata.Client, admin *adminservice.Service) *Handler {
	return &Handler{wikidata: wikidata, enrich: enrich, admin: admin}
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

	result := HarvestResult{Total: len(records), Errors: []HarvestRecordError{}}
	reportProgress(progress, result)

	for i, rec := range records {
		outcome, procErr := processRecord(ctx, rec, h.enrich, h.admin)
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
			}
		}
		result.Processed++
		reportProgress(progress, result)
	}

	return result, nil
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
	sourceID := strings.TrimSpace(rec.SourceID)
	if sourceID == "" {
		return 0, errors.New("record has no source id")
	}

	_, err := admin.FindBookByExternalID(ctx, wikidataSourceName, sourceID)
	if err == nil {
		return outcomeSkipped, nil
	}
	if !errors.Is(err, adminservice.ErrBookNotFound) {
		return 0, fmt.Errorf("check existing book: %w", err)
	}

	enriched := rec
	if enrich != nil {
		enrichRecord(ctx, &enriched, enrich)
	}

	authorIDs, err := admin.ResolveOrCreateAuthorIDsByNames(ctx, enriched.Authors)
	if err != nil {
		return 0, fmt.Errorf("resolve authors: %w", err)
	}
	genreIDs, err := admin.ResolveOrCreateGenreIDsByNames(ctx, enriched.Genres)
	if err != nil {
		return 0, fmt.Errorf("resolve genres: %w", err)
	}

	externalSource := wikidataSourceName
	input := adminservice.BookInput{
		Title:          enriched.Title,
		Description:    enriched.Description,
		CoverURL:       enriched.CoverURL,
		ISBN:           enriched.ISBN,
		PublishedYear:  enriched.PublishedYear,
		PageCount:      enriched.PageCount,
		Language:       enriched.Language,
		AuthorIDs:      authorIDs,
		GenreIDs:       genreIDs,
		IsHidden:       true,
		ExternalSource: &externalSource,
		ExternalID:     &sourceID,
	}

	if _, err := admin.CreateBook(ctx, input); err != nil {
		return 0, fmt.Errorf("create book: %w", err)
	}
	return outcomeCreated, nil
}

// enrichRecord fills CoverURL, Description, PageCount and ISBN from enrich
// (Open Library, in production) but only where rec's own value is nil -
// Wikidata's own values are never overwritten. Enrichment lookup failures
// (not found, rate limited, source unavailable) are not fatal to the
// record: the book is still created using whatever Wikidata provided.
func enrichRecord(ctx context.Context, rec *bookdata.ExternalBookRecord, enrich bookdata.Client) {
	if rec.CoverURL != nil && rec.Description != nil && rec.PageCount != nil && rec.ISBN != nil {
		return
	}

	var enrichedRec *bookdata.ExternalBookRecord
	if rec.ISBN != nil {
		if found, err := enrich.FindByISBN(ctx, *rec.ISBN); err == nil {
			enrichedRec = found
		}
	}
	if enrichedRec == nil && strings.TrimSpace(rec.Title) != "" {
		author := ""
		if len(rec.Authors) > 0 {
			author = rec.Authors[0]
		}
		if found, err := enrich.FindByTitleAuthor(ctx, rec.Title, author); err == nil {
			enrichedRec = found
		}
	}
	if enrichedRec == nil {
		return
	}

	if rec.CoverURL == nil {
		rec.CoverURL = enrichedRec.CoverURL
	}
	if rec.Description == nil {
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
func NewHarvestTask(sparqlQuery string) (*asynq.Task, error) {
	payload, err := json.Marshal(HarvestPayload{SPARQLQuery: sparqlQuery})
	if err != nil {
		return nil, fmt.Errorf("harvest: marshal payload: %w", err)
	}
	return asynq.NewTask(TaskTypeHarvestWikidataQuery, payload,
		asynq.Queue(QueueName),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
		asynq.Retention(24*time.Hour),
	), nil
}
