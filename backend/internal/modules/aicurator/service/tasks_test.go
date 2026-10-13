package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type harness struct {
	repo  *fakeRepo
	model *fakeLLM
	wd    *fakeWikidata
	final bool
	h     *Handlers
}

func newHarness(resp string) *harness {
	hs := &harness{
		repo:  newFakeRepo(),
		model: &fakeLLM{resp: answer(resp)},
		wd: &fakeWikidata{
			hits: map[string][]bookdata.AuthorCandidate{
				"Иван Гончаров": {{QID: "Q192279", Description: "русский писатель"}},
				"Лев Толстой":   {{QID: "Q7243", Description: "писатель"}},
				"Антон Чехов":   {{QID: "Q5685", Description: "писатель"}},
			},
			records: map[string]*bookdata.ExternalAuthorRecord{
				"Q192279": person("Q192279", "Иван Александрович Гончаров"),
				"Q7243":   person("Q7243", "Лев Николаевич Толстой"),
				"Q5685":   person("Q5685", "Антон Павлович Чехов"),
			},
			searchErr: map[string]error{},
		},
	}
	hs.h = NewHandlers(hs.repo, hs.model, "claude-haiku-5-5", NewResolver(hs.wd, hs.wd, eligible), nil,
		WithHandlerClock(fixedClock), WithFinalAttempt(func(context.Context) bool { return hs.final }))
	return hs
}

const threeAuthors = `{"authors":[{"name":"Иван Гончаров","note":"«Обломов»"},{"name":"Лев Толстой","note":""},{"name":"Антон Чехов","note":""}]}`

func (hs *harness) run(t *testing.T, id uuid.UUID) error {
	t.Helper()
	task, err := NewCurateTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return hs.h.HandleCurate(context.Background(), task)
}

func (hs *harness) request(status string) uuid.UUID {
	return seed(hs.repo, uuid.New(), status, now0.Add(-time.Minute), now0.Add(-time.Minute))
}

func TestCurateTaskAndOptions(t *testing.T) {
	id := uuid.New()
	task, _ := NewCurateTask(id)
	var p CuratePayload
	if task.Type() != "ai_curator:curate" || json.Unmarshal(task.Payload(), &p) != nil || p.RequestID != id.String() {
		t.Fatalf("task = %s %s", task.Type(), task.Payload())
	}
	if QueueName != "ai_curator" || len(CurateOptions()) != 3 {
		t.Fatalf("options = %v", CurateOptions())
	}
	opts := map[asynq.OptionType]any{}
	for _, o := range CurateOptions() {
		opts[o.Type()] = o.Value()
	}
	if opts[asynq.QueueOpt] != "ai_curator" || opts[asynq.MaxRetryOpt] != 2 || opts[asynq.TimeoutOpt] != 15*time.Minute {
		t.Errorf("options = %v", opts)
	}
}

func TestHandleCurate_HappyPath(t *testing.T) {
	hs := newHarness(threeAuthors)
	id := hs.request(domain.StatusQueued)
	if err := hs.run(t, id); err != nil {
		t.Fatalf("HandleCurate: %v", err)
	}
	req := hs.repo.requests[id]
	if req.Status != domain.StatusDone || req.FinishedAt == nil || req.Model != "claude-haiku-5-5" || req.InputTokens != 900 || req.OutputTokens != 2400 {
		t.Errorf("request = %+v", req)
	}
	cands := hs.repo.cands[id]
	if len(cands) != 3 || cands[0].MatchStatus != domain.MatchMatched || cands[0].Options[0].QID != "Q192279" {
		t.Errorf("candidates = %+v", cands)
	}
	if len(hs.model.requests) != 1 {
		t.Fatalf("model calls = %d", len(hs.model.requests))
	}
	mr := hs.model.requests[0]
	if mr.System != SystemPrompt || mr.User != UserMessage("p", 20) || mr.MaxTokens != 4096 || mr.Schema == nil {
		t.Errorf("model request = %+v", mr)
	}
	if len(hs.repo.purgedAt) != 1 || !hs.repo.purgedAt[0].Equal(now0.AddDate(0, 0, -30)) {
		t.Errorf("purge = %v", hs.repo.purgedAt)
	}
}

func TestHandleCurate_WikidataOutageResumesWithoutModelCall(t *testing.T) {
	hs := newHarness(threeAuthors)
	id := hs.request(domain.StatusQueued)
	hs.wd.searchErr["Лев Толстой"] = bookdata.ErrSourceUnavailable

	if err := hs.run(t, id); err == nil {
		t.Fatal("expected a retryable error on a non-final attempt")
	}
	cands := hs.repo.cands[id]
	if hs.repo.requests[id].Status != domain.StatusRunning || cands[0].MatchStatus != domain.MatchMatched || cands[1].MatchStatus != domain.MatchPending {
		t.Fatalf("after outage: %+v %+v", hs.repo.requests[id], cands)
	}

	// The retry resumes at the failed candidate and never calls the model.
	delete(hs.wd.searchErr, "Лев Толстой")
	hs.wd.searches = nil
	if err := hs.run(t, id); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(hs.model.requests) != 1 {
		t.Errorf("model calls = %d, want 1", len(hs.model.requests))
	}
	if len(hs.wd.searches) != 2 || hs.wd.searches[0] != "Лев Толстой" {
		t.Errorf("searches on retry = %v", hs.wd.searches)
	}
	if hs.repo.requests[id].Status != domain.StatusDone {
		t.Errorf("status = %s", hs.repo.requests[id].Status)
	}
}

func TestHandleCurate_WikidataOutageOnFinalAttempt(t *testing.T) {
	hs := newHarness(threeAuthors)
	id := hs.request(domain.StatusQueued)
	hs.wd.searchErr["Лев Толстой"] = bookdata.ErrRateLimited
	hs.final = true
	if err := hs.run(t, id); err != nil {
		t.Fatalf("final attempt: %v", err)
	}
	cands := hs.repo.cands[id]
	if cands[0].MatchStatus != domain.MatchMatched || cands[1].MatchStatus != domain.MatchError || cands[2].MatchStatus != domain.MatchError {
		t.Errorf("candidates = %+v", cands)
	}
	if hs.repo.requests[id].Status != domain.StatusDone {
		t.Errorf("status = %s", hs.repo.requests[id].Status)
	}
}

func TestHandleCurate_ModelErrors(t *testing.T) {
	// Transient, non-final: returned for a retry, request still running.
	hs := newHarness(threeAuthors)
	id := hs.request(domain.StatusQueued)
	hs.model.err = fmt.Errorf("%w: 529", llm.ErrTransient)
	err := hs.run(t, id)
	if err == nil || errors.Is(err, asynq.SkipRetry) || hs.repo.requests[id].Status != domain.StatusRunning {
		t.Fatalf("transient non-final: err=%v status=%s", err, hs.repo.requests[id].Status)
	}
	// Final attempt: failed / model_error.
	hs.final = true
	err = hs.run(t, id)
	req := hs.repo.requests[id]
	if !errors.Is(err, asynq.SkipRetry) || req.Status != domain.StatusFailed || *req.ErrorCode != domain.ErrorModelError {
		t.Fatalf("transient final: err=%v req=%+v", err, req)
	}

	// Non-transient: SkipRetry at once.
	hs2 := newHarness(threeAuthors)
	id2 := hs2.request(domain.StatusQueued)
	hs2.model.err = errors.New("llm: anthropic status 400")
	err = hs2.run(t, id2)
	req2 := hs2.repo.requests[id2]
	if !errors.Is(err, asynq.SkipRetry) || req2.Status != domain.StatusFailed || *req2.ErrorCode != domain.ErrorModelError || len(hs2.repo.cands[id2]) != 0 {
		t.Fatalf("non-transient: err=%v req=%+v", err, req2)
	}
}

func TestHandleCurate_OutputFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		resp llm.StructuredResponse
		code string
	}{
		"refusal":   {llm.StructuredResponse{StopReason: llm.StopReasonRefusal}, domain.ErrorModelRefusal},
		"truncated": {llm.StructuredResponse{StopReason: llm.StopReasonMaxTokens}, domain.ErrorModelTruncated},
		"invalid":   {answer(`{"names":[]}`), domain.ErrorInvalidOutput},
	} {
		hs := newHarness("")
		hs.model.resp = tc.resp
		id := hs.request(domain.StatusQueued)
		if err := hs.run(t, id); err != nil {
			t.Errorf("%s: err = %v", name, err)
		}
		req := hs.repo.requests[id]
		if req.Status != domain.StatusFailed || *req.ErrorCode != tc.code || len(hs.repo.cands[id]) != 0 {
			t.Errorf("%s: req = %+v", name, req)
		}
	}

	hs := newHarness(`{"authors":[]}`)
	id := hs.request(domain.StatusQueued)
	if err := hs.run(t, id); err != nil || hs.repo.requests[id].Status != domain.StatusDone || len(hs.repo.cands[id]) != 0 {
		t.Errorf("empty list: err=%v req=%+v", err, hs.repo.requests[id])
	}
}

func TestHandleCurate_TerminalAndMissingAreNoOps(t *testing.T) {
	for _, status := range []string{domain.StatusDone, domain.StatusFailed} {
		hs := newHarness(threeAuthors)
		id := hs.request(status)
		if err := hs.run(t, id); err != nil || len(hs.model.requests) != 0 || hs.repo.requests[id].Status != status {
			t.Errorf("%s: err=%v calls=%d", status, err, len(hs.model.requests))
		}
		if len(hs.repo.purgedAt) != 1 {
			t.Errorf("%s: purge must still run", status)
		}
	}
	hs := newHarness(threeAuthors)
	if err := hs.run(t, uuid.New()); err != nil || len(hs.model.requests) != 0 {
		t.Errorf("missing: err=%v", err)
	}
	// A purge failure is not fatal.
	hs.repo.purgeErr = errors.New("db")
	id := hs.request(domain.StatusQueued)
	if err := hs.run(t, id); err != nil || hs.repo.requests[id].Status != domain.StatusDone {
		t.Errorf("purge failure: err=%v", err)
	}
	// A bad payload is skipped.
	if err := hs.h.HandleCurate(context.Background(), asynq.NewTask(TaskTypeCurate, []byte(`{"request_id":"x"}`))); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("bad payload: %v", err)
	}
}
