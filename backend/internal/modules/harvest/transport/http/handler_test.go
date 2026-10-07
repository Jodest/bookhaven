package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// fakeEnqueuer is a hand-rolled TaskEnqueuer fake (no mocking library, per
// repo convention).
type fakeEnqueuer struct {
	err        error
	lastTask   *asynq.Task
	returnInfo *asynq.TaskInfo
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.lastTask = task
	if f.err != nil {
		return nil, f.err
	}
	if f.returnInfo != nil {
		return f.returnInfo, nil
	}
	return &asynq.TaskInfo{
		ID:    "task-1",
		Queue: harvestservice.QueueName,
		State: asynq.TaskStatePending,
	}, nil
}

// fakeInspector is a hand-rolled TaskInspector fake.
type fakeInspector struct {
	pending   []*asynq.TaskInfo
	active    []*asynq.TaskInfo
	retry     []*asynq.TaskInfo
	completed []*asynq.TaskInfo
	archived  []*asynq.TaskInfo

	getByID map[string]*asynq.TaskInfo
	getErr  error
	listErr error
}

func (f *fakeInspector) GetTaskInfo(qname, id string) (*asynq.TaskInfo, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if info, ok := f.getByID[id]; ok {
		return info, nil
	}
	return nil, asynq.ErrTaskNotFound
}

func (f *fakeInspector) ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pending, nil
}

func (f *fakeInspector) ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.active, nil
}

func (f *fakeInspector) ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.retry, nil
}

func (f *fakeInspector) ListArchivedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.archived, nil
}

func (f *fakeInspector) ListCompletedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.completed, nil
}

// fakeSearcher is a hand-rolled AuthorSearcher fake (no mocking library, per
// repo convention).
type fakeSearcher struct {
	candidates  []bookdata.AuthorCandidate
	err         error
	lastQuery   string
	capturedCtx context.Context
}

func (f *fakeSearcher) SearchAuthors(ctx context.Context, query string) ([]bookdata.AuthorCandidate, error) {
	f.lastQuery = query
	f.capturedCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

func newTestContext(method, path string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestTriggerHarvest_ValidAuthorQIDs_ReturnsCreated(t *testing.T) {
	enqueuer := &fakeEnqueuer{}
	h := NewHandler(enqueuer, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"Q7200", "Q1183296"}, Limit: 100})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp triggerHarvestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-1" || resp.Queue != harvestservice.QueueName || resp.State != "pending" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if enqueuer.lastTask == nil {
		t.Fatal("expected enqueuer to receive a task")
	}
	var payload harvestservice.HarvestPayload
	if err := json.Unmarshal(enqueuer.lastTask.Payload(), &payload); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}
	if payload.SPARQLQuery == "" {
		t.Fatal("expected non-empty sparql query built from author QIDs")
	}
}

func TestTriggerHarvest_EmptyAuthorQIDs_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("at least one author QID is required")) {
		t.Fatalf("expected ErrInvalidQuery message in body, got %s", rec.Body.String())
	}
}

func TestTriggerHarvest_MalformedQID_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"not-a-qid"}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("invalid author QID")) {
		t.Fatalf("expected invalid QID message in body, got %s", rec.Body.String())
	}
}

func TestTriggerHarvest_EnqueueError_ReturnsBadGateway(t *testing.T) {
	enqueuer := &fakeEnqueuer{err: errors.New("redis unavailable")}
	h := NewHandler(enqueuer, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"Q7200"}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestListHarvestJobs_MergesAllStatesInDocumentedOrder(t *testing.T) {
	inspector := &fakeInspector{
		active:    []*asynq.TaskInfo{{ID: "a1", State: asynq.TaskStateActive}},
		retry:     []*asynq.TaskInfo{{ID: "r1", State: asynq.TaskStateRetry}},
		pending:   []*asynq.TaskInfo{{ID: "p1", State: asynq.TaskStatePending}},
		completed: []*asynq.TaskInfo{{ID: "c1", State: asynq.TaskStateCompleted}},
		archived:  []*asynq.TaskInfo{{ID: "ar1", State: asynq.TaskStateArchived}},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs", nil)
	if err := h.ListHarvestJobs(c); err != nil {
		t.Fatalf("ListHarvestJobs returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp listHarvestJobsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	wantOrder := []string{"a1", "r1", "p1", "c1", "ar1"}
	if len(resp.Items) != len(wantOrder) {
		t.Fatalf("items = %d, want %d", len(resp.Items), len(wantOrder))
	}
	for i, id := range wantOrder {
		if resp.Items[i].ID != id {
			t.Fatalf("items[%d].ID = %q, want %q", i, resp.Items[i].ID, id)
		}
	}
}

func TestGetHarvestJob_Found_ReturnsMappedFields(t *testing.T) {
	result := harvestservice.HarvestResult{Processed: 5, Created: 3, Skipped: 2}
	resultBytes, _ := json.Marshal(result)
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-1": {
				ID:       "job-1",
				Queue:    harvestservice.QueueName,
				State:    asynq.TaskStateCompleted,
				Type:     harvestservice.TaskTypeHarvestWikidataQuery,
				MaxRetry: 3,
				Retried:  1,
				Result:   resultBytes,
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.State != "completed" || resp.MaxRetry != 3 || resp.Retried != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Result == nil || resp.Result.Created != 3 {
		t.Fatalf("expected result to be populated, got %+v", resp.Result)
	}
}

func TestGetHarvestJob_CompletedWithMalformedResult_OmitsResultNotError(t *testing.T) {
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-2": {
				ID:     "job-2",
				State:  asynq.TaskStateCompleted,
				Result: []byte("not json"),
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-2", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-2")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Result != nil {
		t.Fatalf("expected result to be nil for malformed bytes, got %+v", resp.Result)
	}
}

// TestGetHarvestJob_ActiveWithPartialResult_ResultSurfaced proves the claim
// in docs/features/harvest-progress.md Acceptance Criteria that
// toHarvestJobResponse needs no shape/gating change to surface a Result
// written mid-task (State == active, Processed < Total) - it already
// unconditionally maps Result whenever TaskInfo.Result decodes cleanly,
// regardless of State.
func TestGetHarvestJob_ActiveWithPartialResult_ResultSurfaced(t *testing.T) {
	partial := harvestservice.HarvestResult{Processed: 37, Total: 200, Errors: []harvestservice.HarvestRecordError{}}
	resultBytes, _ := json.Marshal(partial)
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-active": {
				ID:     "job-active",
				Queue:  harvestservice.QueueName,
				State:  asynq.TaskStateActive,
				Result: resultBytes,
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-active", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-active")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.State != "active" {
		t.Fatalf("State = %q, want active", resp.State)
	}
	if resp.Result == nil {
		t.Fatal("expected partial result to be surfaced for an active job")
	}
	if resp.Result.Processed != 37 || resp.Result.Total != 200 {
		t.Fatalf("unexpected partial result: %+v", resp.Result)
	}
}

// TestGetHarvestJob_SkippedRecordsSurfacedForActiveAndCompleted proves
// docs/features/harvest-job-report.md's claim that toHarvestJobResponse
// needs no code change to carry the new HarvestResult.SkippedRecords field
// through GET /jobs/:id, for both a still-running (active) job and a
// finished (completed) one - the same unconditional Result mapping already
// established by harvest-progress.md for Errors/Processed.
func TestGetHarvestJob_SkippedRecordsSurfacedForActiveAndCompleted(t *testing.T) {
	for _, state := range []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStateCompleted} {
		result := harvestservice.HarvestResult{
			Processed: 10, Total: 10, Created: 7, Skipped: 3,
			Errors: []harvestservice.HarvestRecordError{},
			SkippedRecords: []harvestservice.HarvestSkippedRecord{
				{Index: 1, Title: "Книга 2"},
				{Index: 5, Title: "Книга 6"},
				{Index: 8, Title: "Книга 9"},
			},
		}
		resultBytes, _ := json.Marshal(result)
		id := "job-" + state.String()
		inspector := &fakeInspector{
			getByID: map[string]*asynq.TaskInfo{
				id: {ID: id, Queue: harvestservice.QueueName, State: state, Result: resultBytes},
			},
		}
		h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

		c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/"+id, nil)
		c.SetParamNames("id")
		c.SetParamValues(id)

		if err := h.GetHarvestJob(c); err != nil {
			t.Fatalf("state=%s: GetHarvestJob returned error: %v", state, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("state=%s: status = %d, want %d, body=%s", state, rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp harvestJobResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("state=%s: unmarshal response: %v", state, err)
		}
		if resp.Result == nil {
			t.Fatalf("state=%s: expected result to be populated", state)
		}
		if len(resp.Result.SkippedRecords) != 3 {
			t.Fatalf("state=%s: SkippedRecords = %+v, want 3 entries", state, resp.Result.SkippedRecords)
		}
		if resp.Result.SkippedRecords[0] != (harvestservice.HarvestSkippedRecord{Index: 1, Title: "Книга 2"}) {
			t.Errorf("state=%s: SkippedRecords[0] = %+v, want {1 Книга 2}", state, resp.Result.SkippedRecords[0])
		}
	}

	// Also confirm the same field flows through GET /jobs (list), not just
	// GET /jobs/:id.
	result := harvestservice.HarvestResult{
		SkippedRecords: []harvestservice.HarvestSkippedRecord{{Index: 0, Title: "Книга 1"}},
	}
	resultBytes, _ := json.Marshal(result)
	inspector := &fakeInspector{
		completed: []*asynq.TaskInfo{{ID: "c1", State: asynq.TaskStateCompleted, Result: resultBytes}},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs", nil)
	if err := h.ListHarvestJobs(c); err != nil {
		t.Fatalf("ListHarvestJobs returned error: %v", err)
	}
	var listResp listHarvestJobsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Result == nil || len(listResp.Items[0].Result.SkippedRecords) != 1 {
		t.Fatalf("expected SkippedRecords to flow through GET /jobs, got %+v", listResp.Items)
	}
}

// TestGetHarvestJob_ActiveWithNoResultYet_ResultOmitted covers the window
// right after a task is dequeued but before its first reportProgress write
// has landed - Result stays nil/omitted, exactly like the pending/retry case.
func TestGetHarvestJob_ActiveWithNoResultYet_ResultOmitted(t *testing.T) {
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-active-2": {
				ID:    "job-active-2",
				Queue: harvestservice.QueueName,
				State: asynq.TaskStateActive,
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-active-2", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-active-2")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Result != nil {
		t.Fatalf("expected nil result before first progress write, got %+v", resp.Result)
	}
}

func TestGetHarvestJob_NotFound_ReturnsNotFound(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/missing", nil)
	c.SetParamNames("id")
	c.SetParamValues("missing")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSearchAuthors_ValidQuery_ReturnsMappedItems(t *testing.T) {
	searcher := &fakeSearcher{
		candidates: []bookdata.AuthorCandidate{
			{QID: "Q7200", Label: "Лев Толстой", Description: "русский писатель"},
			{QID: "Q171976", Label: "Алексей Николаевич Толстой"},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, searcher, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=%D0%A2%D0%BE%D0%BB%D1%81%D1%82%D0%BE%D0%B9", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if searcher.lastQuery != "Толстой" {
		t.Fatalf("lastQuery = %q, want Толстой", searcher.lastQuery)
	}

	var resp listAuthorCandidatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].QID != "Q7200" || resp.Items[0].Label != "Лев Толстой" || resp.Items[0].Description != "русский писатель" {
		t.Fatalf("unexpected items[0]: %+v", resp.Items[0])
	}
}

func TestSearchAuthors_ZeroResults_ReturnsEmptyItems(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{candidates: []bookdata.AuthorCandidate{}}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=xyzzy", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp listAuthorCandidatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("items = %v, want empty", resp.Items)
	}
}

func TestSearchAuthors_MissingQuery_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSearchAuthors_BlankQuery_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=%20%20", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSearchAuthors_InvalidQueryFromSearcher_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{err: bookdata.ErrInvalidQuery}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=bad", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSearchAuthors_RateLimitedOrUnavailable_ReturnsBadGateway(t *testing.T) {
	for _, err := range []error{bookdata.ErrRateLimited, bookdata.ErrSourceUnavailable} {
		h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{err: err}, &fakeFinder{})

		c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=Толстой", nil)

		if callErr := h.SearchAuthors(c); callErr != nil {
			t.Fatalf("SearchAuthors returned error: %v", callErr)
		}
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("err=%v: status = %d, want %d", err, rec.Code, http.StatusBadGateway)
		}
	}
}

func TestSearchAuthors_UnmappedError_ReturnsInternalServerError(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{err: errors.New("boom")}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=Толстой", nil)

	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// fakeFinder is a hand-rolled ExistingAuthorFinder fake.
type fakeFinder struct {
	authors  map[string]catalogDomain.Author
	err      error
	calls    int
	lastQIDs []string
}

func (f *fakeFinder) FindAuthorsByWikidataQIDs(ctx context.Context, qids []string) (map[string]catalogDomain.Author, error) {
	f.calls++
	f.lastQIDs = qids
	if f.err != nil {
		return nil, f.err
	}
	return f.authors, nil
}

func TestSearchAuthors_ExistingMarkers(t *testing.T) {
	tolstoy := catalogDomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: "rejected"}
	searcher := &fakeSearcher{candidates: []bookdata.AuthorCandidate{
		{QID: "Q7200", Label: "Лев Толстой"},
		{QID: "Q171976", Label: "Алексей Николаевич Толстой"},
	}}
	finder := &fakeFinder{authors: map[string]catalogDomain.Author{"Q7200": tolstoy}}
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, searcher, finder)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=x", nil)
	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if finder.calls != 1 || len(finder.lastQIDs) != 2 || finder.lastQIDs[0] != "Q7200" || finder.lastQIDs[1] != "Q171976" {
		t.Fatalf("finder calls=%d qids=%v, want one call with both QIDs", finder.calls, finder.lastQIDs)
	}

	var raw struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ex, _ := raw.Items[0]["existing"].(map[string]interface{})
	if ex["id"] != tolstoy.ID.String() || ex["name"] != "Лев Толстой" || ex["slug"] != "lev-tolstoy" || ex["status"] != "rejected" {
		t.Fatalf("unexpected existing marker: %v", raw.Items[0])
	}
	if v, ok := raw.Items[1]["existing"]; !ok || v != nil {
		t.Fatalf("expected existing:null on items[1], got %v", raw.Items[1])
	}
}

func TestSearchAuthors_FinderError_ReturnsCandidatesWithNullExisting(t *testing.T) {
	searcher := &fakeSearcher{candidates: []bookdata.AuthorCandidate{{QID: "Q7200", Label: "Лев Толстой"}}}
	finder := &fakeFinder{err: errors.New("db down")}
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, searcher, finder)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=x", nil)
	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (search must survive a lookup failure)", rec.Code)
	}
	var raw struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Items) != 1 {
		t.Fatalf("items = %v", raw.Items)
	}
	if v, ok := raw.Items[0]["existing"]; !ok || v != nil {
		t.Fatalf("expected existing:null, got %v", raw.Items[0])
	}
}

func TestSearchAuthors_ZeroResults_DoesNotCallFinder(t *testing.T) {
	finder := &fakeFinder{}
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{}, &fakeSearcher{candidates: []bookdata.AuthorCandidate{}}, finder)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/authors/search?q=xyzzy", nil)
	if err := h.SearchAuthors(c); err != nil {
		t.Fatalf("SearchAuthors returned error: %v", err)
	}
	if rec.Code != http.StatusOK || finder.calls != 0 {
		t.Fatalf("status=%d finder calls=%d, want 200 and 0", rec.Code, finder.calls)
	}
}
