package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	screenworksservice "github.com/bookhaven/backend/internal/modules/screenworks/service"
	"github.com/hibiken/asynq"
)

func TestTriggerHarvest_RecordsManualTriggerAndNormalizedQIDs(t *testing.T) {
	enqueuer := &fakeEnqueuer{}
	h := NewHandler(enqueuer, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{" Q7200 ", "", "Q1183296", "Q7200"}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)
	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload harvestservice.HarvestPayload
	if err := json.Unmarshal(enqueuer.lastTask.Payload(), &payload); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}
	if payload.Trigger != harvestservice.TriggerManual {
		t.Errorf("trigger = %q, want manual", payload.Trigger)
	}
	if !reflect.DeepEqual(payload.AuthorQIDs, []string{"Q7200", "Q1183296"}) {
		t.Errorf("author_qids = %v", payload.AuthorQIDs)
	}
}

func payloadBytes(t *testing.T, p harvestservice.HarvestPayload) []byte {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestListHarvestJobs_TriggerAndAuthorQIDs(t *testing.T) {
	inspector := &fakeInspector{
		active: []*asynq.TaskInfo{{ID: "sched", State: asynq.TaskStateActive, Payload: payloadBytes(t, harvestservice.HarvestPayload{
			SPARQLQuery: "SELECT", Trigger: harvestservice.TriggerScheduled, AuthorQIDs: []string{"Q7200", "Q1183296"},
		})}},
		pending: []*asynq.TaskInfo{{ID: "manual", State: asynq.TaskStatePending, Payload: payloadBytes(t, harvestservice.HarvestPayload{
			SPARQLQuery: "SELECT", Trigger: harvestservice.TriggerManual, AuthorQIDs: []string{"Q7200"},
		})}},
		completed: []*asynq.TaskInfo{{ID: "legacy", State: asynq.TaskStateCompleted, Payload: []byte(`{"sparql_query":"SELECT"}`)}},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs", nil)
	if err := h.ListHarvestJobs(c); err != nil {
		t.Fatalf("ListHarvestJobs returned error: %v", err)
	}
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	want := map[string]struct {
		trigger string
		qids    []any
	}{
		"sched":  {"scheduled", []any{"Q7200", "Q1183296"}},
		"manual": {"manual", []any{"Q7200"}},
		"legacy": {"manual", []any{}},
	}
	if len(resp.Items) != len(want) {
		t.Fatalf("items = %d, want %d", len(resp.Items), len(want))
	}
	for _, item := range resp.Items {
		w := want[item["id"].(string)]
		if item["trigger"] != w.trigger {
			t.Errorf("%v: trigger = %v, want %s", item["id"], item["trigger"], w.trigger)
		}
		qids, ok := item["author_qids"].([]any)
		if !ok || !reflect.DeepEqual(qids, w.qids) {
			t.Errorf("%v: author_qids = %#v, want %v (never null)", item["id"], item["author_qids"], w.qids)
		}
	}
}

func TestTriggerCatalogAdaptationSync(t *testing.T) {
	enqueuer := &fakeEnqueuer{}
	h := NewHandler(enqueuer, &fakeInspector{}, &fakeSearcher{}, &fakeFinder{})
	c, rec := newTestContext(http.MethodPost, "/admin/books/sync-adaptations/catalog", nil)
	if err := h.TriggerCatalogAdaptationSync(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated || enqueuer.lastTask == nil || enqueuer.lastTask.Type() != screenworksservice.TaskTypeCatalogSync {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload screenworksservice.CatalogSyncPayload
	_ = json.Unmarshal(enqueuer.lastTask.Payload(), &payload)
	if payload.Trigger != "manual" {
		t.Errorf("trigger = %q", payload.Trigger)
	}
}

func TestTriggerCatalogAdaptationSync_ConflictAndFailures(t *testing.T) {
	running := &asynq.TaskInfo{Type: screenworksservice.TaskTypeCatalogSync}
	cases := []struct {
		name      string
		inspector *fakeInspector
		enqErr    error
		want      int
	}{
		{"active", &fakeInspector{active: []*asynq.TaskInfo{running}}, nil, http.StatusConflict},
		{"pending", &fakeInspector{pending: []*asynq.TaskInfo{running}}, nil, http.StatusConflict},
		{"retry", &fakeInspector{retry: []*asynq.TaskInfo{running}}, nil, http.StatusConflict},
		{"inspector error", &fakeInspector{listErr: errors.New("redis")}, nil, http.StatusBadGateway},
		{"enqueue error", &fakeInspector{}, errors.New("redis"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		enqueuer := &fakeEnqueuer{err: tc.enqErr}
		h := NewHandler(enqueuer, tc.inspector, &fakeSearcher{}, &fakeFinder{})
		c, rec := newTestContext(http.MethodPost, "/admin/books/sync-adaptations/catalog", nil)
		if err := h.TriggerCatalogAdaptationSync(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, rec.Code, tc.want)
		}
		if tc.want == http.StatusConflict && enqueuer.lastTask != nil {
			t.Errorf("%s: a job was queued", tc.name)
		}
	}
}

func TestListHarvestJobs_Kind(t *testing.T) {
	report, _ := json.Marshal(screenworksservice.CatalogSyncResult{Total: 683, Processed: 100, Added: 5, Errors: []screenworksservice.CatalogSyncError{}})
	inspector := &fakeInspector{
		active: []*asynq.TaskInfo{{ID: "sync", Type: screenworksservice.TaskTypeCatalogSync, State: asynq.TaskStateActive,
			Payload: []byte(`{"trigger":"scheduled"}`), Result: report}},
		pending: []*asynq.TaskInfo{{ID: "harvest", Type: harvestservice.TaskTypeHarvestWikidataQuery, State: asynq.TaskStatePending,
			Payload: payloadBytes(t, harvestservice.HarvestPayload{SPARQLQuery: "SELECT", Trigger: harvestservice.TriggerManual, AuthorQIDs: []string{"Q1"}})}},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector, &fakeSearcher{}, &fakeFinder{})
	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs", nil)
	if err := h.ListHarvestJobs(c); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, item := range resp.Items {
		switch item["id"] {
		case "sync":
			rep, _ := item["adaptation_sync"].(map[string]any)
			if item["kind"] != "adaptation_sync" || item["result"] != nil || item["trigger"] != "scheduled" ||
				!reflect.DeepEqual(item["author_qids"], []any{}) || rep["total"] != float64(683) || rep["processed"] != float64(100) {
				t.Errorf("sync item = %#v", item)
			}
		case "harvest":
			if item["kind"] != "harvest" {
				t.Errorf("harvest kind = %v", item["kind"])
			}
			if v, ok := item["adaptation_sync"]; !ok || v != nil {
				t.Errorf("harvest adaptation_sync = %#v (want explicit null)", v)
			}
		}
	}
}
