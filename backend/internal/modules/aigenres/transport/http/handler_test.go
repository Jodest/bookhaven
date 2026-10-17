package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	aigenresservice "github.com/bookhaven/backend/internal/modules/aigenres/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type fakeEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	f.opts = append(f.opts, opts)
	return &asynq.TaskInfo{ID: "task-1"}, nil
}

type fakeService struct {
	active bool

	status      *aigenresservice.Status
	startRun    *domain.Run
	startErr    error
	markFailed  []uuid.UUID
	listFilter  aigenresservice.ListFilter
	listErr     error
	applyErr    error
	applyUser   uuid.UUID
	applyID     uuid.UUID
	applyTarget *uuid.UUID
	bulkItems   []aigenresservice.ApplyItem
	dismissErr  error
	dismissIDs  []uuid.UUID
}

func (f *fakeService) Active() bool { return f.active }
func (f *fakeService) Status(ctx context.Context) (*aigenresservice.Status, error) {
	return f.status, nil
}
func (f *fakeService) StartAnalysis(ctx context.Context, userID uuid.UUID) (*domain.Run, error) {
	return f.startRun, f.startErr
}
func (f *fakeService) MarkRunEnqueueFailed(ctx context.Context, id uuid.UUID) error {
	f.markFailed = append(f.markFailed, id)
	return nil
}
func (f *fakeService) List(ctx context.Context, flt aigenresservice.ListFilter) ([]aigenresservice.SuggestionView, int64, error) {
	f.listFilter = flt
	return []aigenresservice.SuggestionView{sampleView()}, 1, f.listErr
}
func (f *fakeService) Apply(ctx context.Context, userID, id uuid.UUID, target *uuid.UUID) (*aigenresservice.ApplyResult, error) {
	f.applyUser, f.applyID, f.applyTarget = userID, id, target
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	v := sampleView()
	return &aigenresservice.ApplyResult{
		View: &v,
		Merge: &adminservice.GenreBatchMergeOutcome{
			Genre:         &catalogDomain.Genre{ID: uuid.New(), Name: "Фэнтези", Slug: "fentezi", Status: "published"},
			SourcesMerged: 2, BooksReassigned: 5, CollectionsSwitched: 1,
		},
	}, nil
}
func (f *fakeService) ApplyBulk(ctx context.Context, userID uuid.UUID, items []aigenresservice.ApplyItem) *aigenresservice.BulkApplyReport {
	f.bulkItems = items
	return &aigenresservice.BulkApplyReport{Processed: len(items), Applied: 1, Stale: 1, Errors: []aigenresservice.ItemError{}}
}
func (f *fakeService) Dismiss(ctx context.Context, userID, id uuid.UUID) (*aigenresservice.SuggestionView, error) {
	f.dismissIDs = append(f.dismissIDs, id)
	if f.dismissErr != nil {
		return nil, f.dismissErr
	}
	v := sampleView()
	return &v, nil
}
func (f *fakeService) DismissBulk(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) *aigenresservice.BulkDismissReport {
	f.dismissIDs = append(f.dismissIDs, ids...)
	return &aigenresservice.BulkDismissReport{Processed: len(ids), Dismissed: len(ids), Errors: []aigenresservice.ItemError{}}
}

func sampleView() aigenresservice.SuggestionView {
	target, src := uuid.New(), uuid.New()
	return aigenresservice.SuggestionView{
		Suggestion: domain.Suggestion{
			ID: uuid.New(), Kind: domain.KindDuplicate, Status: domain.StatusOpen, ReasonCodes: []string{"spelling"},
			Reason: "одно и то же", Confidence: 0.93, Model: "m", PromptVersion: 1, CreatedAt: time.Now(),
		},
		Target: &aigenresservice.GenreRef{ID: target, Name: "Фэнтези"},
		Genres: []aigenresservice.GenreView{
			{ID: target, Name: "Фэнтези", Status: "published", Exists: true, BooksTotal: 120},
			{ID: src, Name: "Фентези", Exists: false},
		},
	}
}

func newCtx(method, target, body string, role usersDomain.Role) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if role != "" {
		uid := uuid.New()
		c.Set("user", &usersDomain.User{ID: uid, Role: role})
		c.Set("user_id", uid.String())
	}
	return c, rec
}

func call(t *testing.T, fn echo.HandlerFunc, method, body string, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	c, rec := newCtx(method, "/api/v1/admin/genres/ai-suggestions", body, usersDomain.RoleAdmin)
	for k, v := range params {
		c.SetParamNames(k)
		c.SetParamValues(v)
	}
	if err := fn(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return rec
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad error body %q: %v", rec.Body.String(), err)
	}
	return body["error"]
}

func TestAnalyze(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}

	enq := &fakeEnqueuer{}
	h := NewHandler(enq, &fakeService{active: true, startRun: run})
	rec := call(t, h.Analyze, http.MethodPost, "", nil)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), run.ID.String()) {
		t.Fatalf("accepted: %d %s", rec.Code, rec.Body.String())
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != aigenresservice.TaskTypeAnalyze {
		t.Fatalf("tasks: %v", enq.tasks)
	}
	if rec := call(t, h.Analyze, http.MethodPost, "{}", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("{} body: %d", rec.Code)
	}

	// Disabled: 503 with the exact text, nothing queued.
	enq = &fakeEnqueuer{}
	h = NewHandler(enq, &fakeService{active: false})
	rec = call(t, h.Analyze, http.MethodPost, "", nil)
	if rec.Code != http.StatusServiceUnavailable || errorOf(t, rec) != "AI genre analysis is disabled" || len(enq.tasks) != 0 {
		t.Fatalf("disabled: %d %s", rec.Code, rec.Body.String())
	}

	// A run is already active: 409, nothing queued.
	svc := &fakeService{active: true, startErr: aigenresservice.ErrAlreadyRunning}
	enq = &fakeEnqueuer{}
	rec = call(t, NewHandler(enq, svc).Analyze, http.MethodPost, "", nil)
	if rec.Code != http.StatusConflict || errorOf(t, rec) != "AI genre analysis already running" || len(enq.tasks) != 0 {
		t.Fatalf("conflict: %d %s", rec.Code, rec.Body.String())
	}

	// A queue failure: 502 and the run is marked failed.
	svc = &fakeService{active: true, startRun: run}
	rec = call(t, NewHandler(&fakeEnqueuer{err: errors.New("redis down")}, svc).Analyze, http.MethodPost, "", nil)
	if rec.Code != http.StatusBadGateway || len(svc.markFailed) != 1 || svc.markFailed[0] != run.ID {
		t.Fatalf("enqueue failure: %d %v", rec.Code, svc.markFailed)
	}
	// A duplicate task in the queue maps to 409.
	svc = &fakeService{active: true, startRun: run}
	rec = call(t, NewHandler(&fakeEnqueuer{err: asynq.ErrDuplicateTask}, svc).Analyze, http.MethodPost, "", nil)
	if rec.Code != http.StatusConflict || len(svc.markFailed) != 1 {
		t.Fatalf("duplicate: %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeService{status: &aigenresservice.Status{Enabled: true, Model: "m", PromptVersion: 1, OpenCount: 4}})
	rec := call(t, h.SuggestionsStatus, http.MethodGet, "", nil)
	var body map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("status: %d", rec.Code)
	}
	if body["enabled"] != true || body["open_count"] != float64(4) || body["last_run"] != nil || body["model"] != "m" {
		t.Fatalf("body: %v", body)
	}
}

func TestList(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(&fakeEnqueuer{}, svc)
	c, rec := newCtx(http.MethodGet, "/api/v1/admin/genres/ai-suggestions?status=stale&kind=junk&page=2&limit=5", "", usersDomain.RoleAdmin)
	if err := h.ListSuggestions(c); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list: %v %d", err, rec.Code)
	}
	if svc.listFilter.Status != "stale" || svc.listFilter.Kind != "junk" || svc.listFilter.Page != 2 || svc.listFilter.Limit != 5 {
		t.Fatalf("filter: %+v", svc.listFilter)
	}
	var body struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("body: %s", rec.Body.String())
	}
	item := body.Items[0]
	for _, k := range []string{"id", "kind", "status", "reason_codes", "reason", "confidence", "model", "prompt_version", "created_at", "decided_at", "stale", "target", "genres", "applied_target"} {
		if _, ok := item[k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}
	genres := item["genres"].([]any)
	if len(genres) != 2 || genres[1].(map[string]any)["exists"] != false {
		t.Fatalf("genres: %v", genres)
	}

	// Validation errors from the service and from the query.
	svc = &fakeService{listErr: aigenresservice.ErrInvalidFilter}
	c, rec = newCtx(http.MethodGet, "/api/v1/admin/genres/ai-suggestions?kind=other", "", usersDomain.RoleAdmin)
	if err := NewHandler(&fakeEnqueuer{}, svc).ListSuggestions(c); err != nil || rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind: %v %d", err, rec.Code)
	}
	c, rec = newCtx(http.MethodGet, "/api/v1/admin/genres/ai-suggestions?page=abc", "", usersDomain.RoleAdmin)
	if err := h.ListSuggestions(c); err != nil || rec.Code != http.StatusBadRequest {
		t.Fatalf("bad page: %v %d", err, rec.Code)
	}
}

func TestApplySuggestion(t *testing.T) {
	svc := &fakeService{}
	h := NewHandler(&fakeEnqueuer{}, svc)
	id := uuid.New()

	rec := call(t, h.ApplySuggestion, http.MethodPost, "", map[string]string{"id": id.String()})
	if rec.Code != http.StatusOK || svc.applyID != id || svc.applyTarget != nil {
		t.Fatalf("empty body: %d target=%v", rec.Code, svc.applyTarget)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	merge, _ := body["merge"].(map[string]any)
	if merge == nil || merge["sources_merged"] != float64(2) || merge["collections_switched"] != float64(1) || body["status_report"] != nil {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if body["suggestion"] == nil {
		t.Fatal("suggestion missing")
	}

	target := uuid.New()
	rec = call(t, h.ApplySuggestion, http.MethodPost, `{"target_id":"`+target.String()+`"}`, map[string]string{"id": id.String()})
	if rec.Code != http.StatusOK || svc.applyTarget == nil || *svc.applyTarget != target {
		t.Fatalf("with target: %d %v", rec.Code, svc.applyTarget)
	}

	// 400: bad ids, bad body, input errors from the service.
	for name, tc := range map[string]struct {
		id, body string
	}{
		"bad id":        {"nope", ""},
		"bad target":    {id.String(), `{"target_id":"x"}`},
		"malformed":     {id.String(), `{`},
		"non-uuid body": {id.String(), `{"target_id":5}`},
	} {
		if rec := call(t, h.ApplySuggestion, http.MethodPost, tc.body, map[string]string{"id": tc.id}); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}

	for name, tc := range map[string]struct {
		err  error
		code int
		msg  string
	}{
		"not found":     {aigenresservice.ErrNotFound, http.StatusNotFound, "suggestion not found"},
		"target 404":    {adminservice.ErrGenreNotFound, http.StatusNotFound, "genre not found"},
		"not open":      {aigenresservice.ErrNotOpen, http.StatusConflict, "suggestion is not open"},
		"stale":         {aigenresservice.ErrStale, http.StatusConflict, "suggestion is stale"},
		"input":         {&aigenresservice.InputError{Msg: "target_id is not allowed for a junk suggestion"}, http.StatusBadRequest, "target_id is not allowed for a junk suggestion"},
		"internal":      {errors.New("db down"), http.StatusInternalServerError, "failed to apply suggestion"},
		"wrapped stale": {errors.Join(errors.New("x"), aigenresservice.ErrStale), http.StatusConflict, ""},
	} {
		svc := &fakeService{applyErr: tc.err}
		rec := call(t, NewHandler(&fakeEnqueuer{}, svc).ApplySuggestion, http.MethodPost, "", map[string]string{"id": id.String()})
		if rec.Code != tc.code || (tc.msg != "" && errorOf(t, rec) != tc.msg) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestApplySuggestions_Validation(t *testing.T) {
	a, b := uuid.New().String(), uuid.New().String()
	var many []string
	for i := 0; i < 51; i++ {
		many = append(many, `{"id":"`+uuid.NewString()+`"}`)
	}
	for name, body := range map[string]string{
		"malformed":   `{`,
		"empty items": `{"items":[]}`,
		"no items":    `{}`,
		"too many":    `{"items":[` + strings.Join(many, ",") + `]}`,
		"duplicate":   `{"items":[{"id":"` + a + `"},{"id":"` + a + `"}]}`,
		"bad id":      `{"items":[{"id":"` + a + `"},{"id":"nope"}]}`,
		"bad target":  `{"items":[{"id":"` + a + `","target_id":"nope"}]}`,
	} {
		svc := &fakeService{}
		rec := call(t, NewHandler(&fakeEnqueuer{}, svc).ApplySuggestions, http.MethodPost, body, nil)
		if rec.Code != http.StatusBadRequest || svc.bulkItems != nil {
			t.Errorf("%s: %d (items %v)", name, rec.Code, svc.bulkItems)
		}
	}

	svc := &fakeService{}
	target := uuid.NewString()
	rec := call(t, NewHandler(&fakeEnqueuer{}, svc).ApplySuggestions, http.MethodPost,
		`{"items":[{"id":"`+a+`","target_id":"`+target+`"},{"id":"`+b+`"}]}`, nil)
	if rec.Code != http.StatusOK || len(svc.bulkItems) != 2 || svc.bulkItems[0].TargetID == nil || svc.bulkItems[1].TargetID != nil {
		t.Fatalf("bulk apply: %d %v", rec.Code, svc.bulkItems)
	}
	var rep map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rep)
	if rep["processed"] != float64(2) || rep["applied"] != float64(1) || rep["stale"] != float64(1) || rep["errors"] == nil {
		t.Fatalf("report: %v", rep)
	}
}

func TestDismiss(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{}
	h := NewHandler(&fakeEnqueuer{}, svc)
	if rec := call(t, h.DismissSuggestion, http.MethodPost, "", map[string]string{"id": id.String()}); rec.Code != http.StatusOK || svc.dismissIDs[0] != id {
		t.Fatalf("dismiss: %d", rec.Code)
	}
	if rec := call(t, h.DismissSuggestion, http.MethodPost, "", map[string]string{"id": "x"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
	for err, code := range map[error]int{aigenresservice.ErrNotFound: 404, aigenresservice.ErrNotOpen: 409, errors.New("x"): 500} {
		rec := call(t, NewHandler(&fakeEnqueuer{}, &fakeService{dismissErr: err}).DismissSuggestion, http.MethodPost, "", map[string]string{"id": id.String()})
		if rec.Code != code {
			t.Errorf("%v: %d", err, rec.Code)
		}
	}

	// Bulk: duplicates collapse, bad bodies are 400.
	svc = &fakeService{}
	h = NewHandler(&fakeEnqueuer{}, svc)
	a, b := uuid.NewString(), uuid.NewString()
	rec := call(t, h.DismissSuggestions, http.MethodPost, `{"ids":["`+a+`","`+b+`","`+a+`"]}`, nil)
	if rec.Code != http.StatusOK || len(svc.dismissIDs) != 2 {
		t.Fatalf("bulk dismiss: %d %v", rec.Code, svc.dismissIDs)
	}
	var many []string
	for i := 0; i < 51; i++ {
		many = append(many, `"`+uuid.NewString()+`"`)
	}
	for name, body := range map[string]string{
		"malformed": `{`, "empty": `{"ids":[]}`, "bad uuid": `{"ids":["nope"]}`, "too many": `{"ids":[` + strings.Join(many, ",") + `]}`,
	} {
		svc := &fakeService{}
		if rec := call(t, NewHandler(&fakeEnqueuer{}, svc).DismissSuggestions, http.MethodPost, body, nil); rec.Code != http.StatusBadRequest || len(svc.dismissIDs) != 0 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
}

// Every route is registered behind RequireRole(admin) (router.go); a
// moderator, a user and an anonymous caller never reach the service.
func TestRoutes_AdminOnly(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	id := uuid.NewString()
	for _, tc := range []struct {
		name string
		role usersDomain.Role
		want int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"user", usersDomain.RoleUser, http.StatusForbidden},
		{"moderator", usersDomain.RoleModerator, http.StatusForbidden},
		{"admin", usersDomain.RoleAdmin, http.StatusOK},
	} {
		svc := &fakeService{active: true, status: &aigenresservice.Status{}, startRun: &domain.Run{ID: uuid.New()}}
		h := NewHandler(&fakeEnqueuer{}, svc)
		routes := []struct {
			name   string
			method string
			body   string
			fn     echo.HandlerFunc
			ok     int
		}{
			{"list", http.MethodGet, "", h.ListSuggestions, http.StatusOK},
			{"status", http.MethodGet, "", h.SuggestionsStatus, http.StatusOK},
			{"analyze", http.MethodPost, "", h.Analyze, http.StatusAccepted},
			{"apply", http.MethodPost, `{"items":[{"id":"` + id + `"}]}`, h.ApplySuggestions, http.StatusOK},
			{"dismiss", http.MethodPost, `{"ids":["` + id + `"]}`, h.DismissSuggestions, http.StatusOK},
			{"apply one", http.MethodPost, "", h.ApplySuggestion, http.StatusOK},
			{"dismiss one", http.MethodPost, "", h.DismissSuggestion, http.StatusOK},
		}
		for _, r := range routes {
			c, rec := newCtx(r.method, "/api/v1/admin/genres/ai-suggestions", r.body, tc.role)
			c.SetParamNames("id")
			c.SetParamValues(id)
			if err := adminRole(r.fn)(c); err != nil {
				t.Fatalf("%s %s: %v", tc.name, r.name, err)
			}
			want := tc.want
			if tc.want == http.StatusOK {
				want = r.ok
			}
			if rec.Code != want {
				t.Errorf("%s %s: expected %d, got %d", tc.name, r.name, want, rec.Code)
			}
		}
		if tc.want != http.StatusOK && (svc.applyID != uuid.Nil || len(svc.bulkItems) != 0 || len(svc.dismissIDs) != 0) {
			t.Errorf("%s: the service must not be reached", tc.name)
		}
	}
}
