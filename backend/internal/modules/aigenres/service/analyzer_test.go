package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/modules/aigenres/repository"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

// refInRequest finds the ref the request gave to the genre with this name.
func refInRequest(t *testing.T, req llm.StructuredRequest, name string) string {
	t.Helper()
	for _, line := range strings.Split(req.User, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var l genreLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			t.Fatalf("bad genre line %q: %v", line, err)
		}
		if l.Name == name {
			return l.Ref
		}
	}
	t.Fatalf("genre %q not in the request", name)
	return ""
}

func requestLines(req llm.StructuredRequest) []genreLine {
	var out []genreLine
	for _, line := range strings.Split(req.User, "\n") {
		if strings.HasPrefix(line, "{") {
			var l genreLine
			_ = json.Unmarshal([]byte(line), &l)
			out = append(out, l)
		}
	}
	return out
}

func TestAnalyzer_StoresSuggestionsAndNeverWrites(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	fantasy := cat.add("Фэнтези", "published", 120)
	fentezi := cat.add("Фентези", "pending", 3)
	cat.add("Книги 2020", "pending", 5)
	cat.samples[fentezi] = []adminrepository.SampleBook{
		{ID: uuid.New(), Title: strings.Repeat("я", 120)}, {ID: uuid.New(), Title: "Б"}, {ID: uuid.New(), Title: "В"},
	}

	// The model answers with the refs it was given.
	client := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		return jsonResp(answer(
			dup(refInRequest(t, req, "Фэнтези"), []string{refInRequest(t, req, "Фентези")}, 0.95),
			junkItem(refInRequest(t, req, "Книги 2020"), 0.9),
		))
	}}
	an := newTestAnalyzer(client, repo, cat, 300)
	runID := queuedRun(t, repo, domain.TriggerManual)

	if err := an.Run(context.Background(), runID); err != nil {
		t.Fatalf("run: %v", err)
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.Status != domain.RunDone || run.SuggestionsCreated != 2 || run.BatchesTotal != 1 || run.BatchesDone != 1 ||
		run.InputTokens != 100 || run.OutputTokens != 20 || run.Model != "cfg-model" || run.Fingerprint == "" || run.ErrorCode != nil {
		t.Fatalf("run: %+v", run)
	}
	if cat.writes() != 0 {
		t.Fatal("the analysis must never merge or reject a genre")
	}

	items, total, _ := repo.ListSuggestions(context.Background(), listOpen())
	if total != 2 {
		t.Fatalf("stored %d", total)
	}
	d := items[0]
	if d.Kind != domain.KindDuplicate || d.Genres[0].GenreID != fantasy || d.Genres[1].GenreID != fentezi || d.Model != "fake-model" {
		t.Fatalf("duplicate: %+v", d)
	}

	// The request: refs, no UUIDs, samples capped at 2 titles of 80 runes,
	// the system prompt carries the untrusted-data rule.
	req := client.requests[0]
	if strings.Contains(req.User, fantasy.String()) || strings.Contains(req.User, fentezi.String()) {
		t.Fatal("UUIDs must not be sent to the model")
	}
	if !strings.Contains(req.System, "недоверенные данные") {
		t.Fatal("the system prompt must state the untrusted-data rule")
	}
	for _, l := range requestLines(req) {
		if l.Name == "Фентези" {
			if len(l.Samples) != 2 || len([]rune(l.Samples[0])) != 80 {
				t.Fatalf("samples: %v", l.Samples)
			}
		}
		if l.Name == "Фэнтези" && !l.Anchor {
			t.Fatal("the published genre is an anchor")
		}
	}
}

// dynamicLLM builds each answer from the request it receives.
type dynamicLLM struct {
	answer   func(req llm.StructuredRequest) scripted
	requests []llm.StructuredRequest
}

func (d *dynamicLLM) Structured(ctx context.Context, req llm.StructuredRequest) (llm.StructuredResponse, error) {
	d.requests = append(d.requests, req)
	s := d.answer(req)
	return s.resp, s.err
}

func TestAnalyzer_BatchesSendAnchorsEverywhereAndMapRefsBack(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	var anchors []uuid.UUID
	for i := 0; i < 100; i++ {
		anchors = append(anchors, cat.add(fmt.Sprintf("Опубликованный %03d", i), "published", int64(1000-i)))
	}
	var pending []uuid.UUID
	for i := 0; i < 25; i++ {
		pending = append(pending, cat.add(fmt.Sprintf("Новый %02d", i), "pending", 0))
	}

	calls := 0
	client := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		// Propose merging the first pending genre of this request into the
		// anchor numbered like the request.
		calls++
		return jsonResp(answer(dup(refInRequest(t, req, fmt.Sprintf("Опубликованный %03d", calls-1)), []string{firstPendingRef(req)}, 0.8)))
	}}
	an := newTestAnalyzer(client, repo, cat, 10)
	runID := queuedRun(t, repo, domain.TriggerManual)
	if err := an.Run(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("expected 3 requests (10+10+5 pending), got %d", len(client.requests))
	}
	for i, req := range client.requests {
		na := 0
		for _, l := range requestLines(req) {
			if l.Anchor {
				na++
				if len(l.Samples) != 0 {
					t.Fatal("anchors are sent without samples")
				}
			}
		}
		if na != 100 {
			t.Fatalf("request %d has %d anchors", i, na)
		}
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.BatchesTotal != 3 || run.BatchesDone != 3 || run.SuggestionsCreated != 3 || run.Status != domain.RunDone {
		t.Fatalf("run: %+v", run)
	}
	items, _, _ := repo.ListSuggestions(context.Background(), listOpen())
	targets := map[uuid.UUID]bool{}
	for _, it := range items {
		targets[it.Genres[0].GenreID] = true
		if it.Genres[1].GenreID == it.Genres[0].GenreID {
			t.Fatal("source equals target")
		}
	}
	for i := 0; i < 3; i++ {
		if !targets[anchors[i]] {
			t.Fatalf("refs must map back to the catalog genre of anchor %d", i)
		}
	}
	sources := map[uuid.UUID]bool{}
	for _, it := range items {
		sources[it.Genres[1].GenreID] = true
	}
	if len(sources) != 3 {
		t.Fatalf("each batch contributes its own pending genre, got %d distinct", len(sources))
	}
	for id := range sources {
		found := false
		for _, p := range pending {
			found = found || p == id
		}
		if !found {
			t.Fatal("a source must be a pending catalog genre")
		}
	}
}

func firstPendingRef(req llm.StructuredRequest) string {
	for _, l := range requestLines(req) {
		if !l.Anchor {
			return l.Ref
		}
	}
	return ""
}

func TestAnalyzer_TransientFailureStopsRunAndKeepsEarlierBatches(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	for i := 0; i < 6; i++ {
		cat.add(nameOf(i), "pending", 0)
	}
	calls := 0
	client := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		calls++
		if calls == 2 {
			return scripted{err: fmt.Errorf("overloaded: %w", llm.ErrTransient)}
		}
		return jsonResp(answer(junkItem(firstPendingRef(req), 0.9)))
	}}
	an := newTestAnalyzer(client, repo, cat, 2)
	runID := queuedRun(t, repo, domain.TriggerManual)
	if err := an.Run(context.Background(), runID); err != nil {
		t.Fatalf("a transient failure must not be returned: %v", err)
	}
	if calls != 2 {
		t.Fatalf("the run must stop at the failed batch, calls=%d", calls)
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.Status != domain.RunFailed || run.ErrorCode == nil || *run.ErrorCode != domain.ErrorTransient || run.BatchesDone != 1 {
		t.Fatalf("run: %+v", run)
	}
	_, total, _ := repo.ListSuggestions(context.Background(), listOpen())
	if total != 1 {
		t.Fatalf("the first batch must stay, got %d", total)
	}
}

func TestAnalyzer_NonTransientFailureStopsWithModelError(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	cat.add("А", "pending", 0)
	client := &scriptedLLM{responses: []scripted{{err: errors.New("400 bad request")}}}
	an := newTestAnalyzer(client, repo, cat, 10)
	runID := queuedRun(t, repo, domain.TriggerManual)
	if err := an.Run(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.Status != domain.RunFailed || *run.ErrorCode != domain.ErrorModelError {
		t.Fatalf("run: %+v", run)
	}
}

func TestAnalyzer_RefusalAndInvalidOutputContinueWithNextBatch(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	for i := 0; i < 6; i++ {
		cat.add(nameOf(i), "pending", 0)
	}
	calls := 0
	client := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		calls++
		switch calls {
		case 1:
			return scripted{resp: llm.StructuredResponse{StopReason: llm.StopReasonRefusal, InputTokens: 5}}
		case 2:
			return scripted{resp: llm.StructuredResponse{StopReason: llm.StopReasonEndTurn, JSON: []byte("not json")}}
		}
		return jsonResp(answer(junkItem(firstPendingRef(req), 0.9)))
	}}
	an := newTestAnalyzer(client, repo, cat, 2)
	runID := queuedRun(t, repo, domain.TriggerManual)
	if err := an.Run(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("every batch must be sent, calls=%d", calls)
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.Status != domain.RunDone || run.BatchesDone != 3 || run.SuggestionsCreated != 1 ||
		run.ErrorCode == nil || *run.ErrorCode != domain.ErrorInvalidOutput {
		t.Fatalf("run: %+v", run)
	}
}

func TestAnalyzer_MarksStaleAtStartAndSkipsDismissedSets(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()
	cat := newFakeCatalog()
	a := cat.add("Фэнтези", "published", 50)
	b := cat.add("Фентези", "pending", 1)
	gone := cat.add("Старый", "pending", 0)

	run0 := queuedRun(t, repo, domain.TriggerManual)
	if _, err := repo.InsertSuggestions(ctx, []domain.NewSuggestion{
		{Kind: domain.KindDuplicate, ReasonCodes: []string{"spelling"}, Reason: "r", Confidence: 0.9, Model: "m", PromptVersion: 1,
			Genres: []domain.SuggestionGenre{{GenreID: a, Role: domain.RoleTarget, NameSnapshot: "Фэнтези"}, {GenreID: gone, Role: domain.RoleSource, NameSnapshot: "Старый"}}},
	}, run0, now0); err != nil {
		t.Fatal(err)
	}
	// "gone" is merged by hand after the suggestion was made.
	tid := a
	cat.genres[gone].MergedIntoID = &tid
	cat.genres[gone].Status = "rejected"

	client := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		return jsonResp(answer(dup(refInRequest(t, req, "Фентези"), []string{refInRequest(t, req, "Фэнтези")}, 0.9)))
	}}
	an := newTestAnalyzer(client, repo, cat, 10)
	if err := an.Run(ctx, queuedRun(t, repo, domain.TriggerManual)); err != nil {
		t.Fatal(err)
	}
	stale, total, _ := repo.ListSuggestions(ctx, listWithStatus(domain.StatusStale))
	if total != 1 || stale[0].Genres[1].GenreID != gone {
		t.Fatalf("the suggestion about a merged genre must be marked stale: %+v", stale)
	}
	open, _, _ := repo.ListSuggestions(ctx, listOpen())
	if len(open) != 1 || open[0].Genres[0].GenreID != b {
		t.Fatalf("the new suggestion must be open: %+v", open)
	}

	// The admin dismisses it; the next run proposes the same pair again, now
	// with the other target: nothing is stored.
	user := insertUser(t, db)
	if ok, err := repo.MarkDismissed(ctx, open[0].ID, user, now0); err != nil || !ok {
		t.Fatalf("dismiss: %v %v", ok, err)
	}
	client2 := &dynamicLLM{answer: func(req llm.StructuredRequest) scripted {
		return jsonResp(answer(dup(refInRequest(t, req, "Фэнтези"), []string{refInRequest(t, req, "Фентези")}, 0.9)))
	}}
	an2 := newTestAnalyzer(client2, repo, cat, 10)
	runID := queuedRun(t, repo, domain.TriggerManual)
	if err := an2.Run(ctx, runID); err != nil {
		t.Fatal(err)
	}
	run, _ := repo.GetRun(ctx, runID)
	if run.Status != domain.RunDone || run.SuggestionsCreated != 0 {
		t.Fatalf("a dismissed set must not be proposed again: %+v", run)
	}
	if _, total, _ := repo.ListSuggestions(ctx, listOpen()); total != 0 {
		t.Fatalf("open = %d", total)
	}
}

func TestAnalyzer_AlreadyHandledRunIsSkipped(t *testing.T) {
	repo, _ := newRepo(t)
	cat := newFakeCatalog()
	cat.add("А", "pending", 0)
	client := &scriptedLLM{}
	an := newTestAnalyzer(client, repo, cat, 10)
	runID := queuedRun(t, repo, domain.TriggerManual)
	_ = repo.FinishRun(context.Background(), runID, domain.RunDone, nil, now0)
	if err := an.Run(context.Background(), runID); err != nil || len(client.requests) != 0 {
		t.Fatalf("a finished run must be skipped: %v", err)
	}
	if err := an.Run(context.Background(), uuid.New()); err != nil {
		t.Fatalf("an unknown run must be skipped: %v", err)
	}
}

func listOpen() repository.ListFilter { return listWithStatus(domain.StatusOpen) }

func listWithStatus(status string) repository.ListFilter {
	return repository.ListFilter{Status: status, Limit: 100}
}
