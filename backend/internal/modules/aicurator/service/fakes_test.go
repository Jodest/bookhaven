package service

import (
	"context"
	"sort"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

// fakeLLM never reaches the network; it counts calls.
type fakeLLM struct {
	resp     llm.StructuredResponse
	err      error
	requests []llm.StructuredRequest
}

func (f *fakeLLM) Structured(ctx context.Context, req llm.StructuredRequest) (llm.StructuredResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return llm.StructuredResponse{}, f.err
	}
	return f.resp, nil
}

func answer(json string) llm.StructuredResponse {
	return llm.StructuredResponse{JSON: []byte(json), StopReason: llm.StopReasonEndTurn, Model: "claude-haiku-5-5", InputTokens: 900, OutputTokens: 2400}
}

// fakeWikidata serves search hits by name and records by QID.
type fakeWikidata struct {
	hits       map[string][]bookdata.AuthorCandidate
	records    map[string]*bookdata.ExternalAuthorRecord
	searchErr  map[string]error
	fetchErr   error
	searches   []string
	fetchCalls int
}

func (f *fakeWikidata) SearchAuthors(ctx context.Context, q string) ([]bookdata.AuthorCandidate, error) {
	f.searches = append(f.searches, q)
	if err := f.searchErr[q]; err != nil {
		return nil, err
	}
	return f.hits[q], nil
}

func (f *fakeWikidata) FetchAuthors(ctx context.Context, qids []string) (map[string]*bookdata.ExternalAuthorRecord, error) {
	f.fetchCalls++
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	out := map[string]*bookdata.ExternalAuthorRecord{}
	for _, q := range qids {
		if r, ok := f.records[q]; ok {
			out[q] = r
		}
	}
	return out, nil
}

func person(qid, name string) *bookdata.ExternalAuthorRecord {
	return &bookdata.ExternalAuthorRecord{QID: qid, Name: name, InstanceOf: []string{"Q5"}}
}

// eligible mirrors adminservice.IsImportableAuthorRecord closely enough
// for these tests (person type and a non-empty label).
func eligible(r *bookdata.ExternalAuthorRecord) bool {
	if r.Name == "" {
		return false
	}
	for _, t := range r.InstanceOf {
		if t == "Q5" || t == "Q61002" || t == "Q16017119" {
			return true
		}
	}
	return false
}

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	requests  map[uuid.UUID]*domain.Request
	cands     map[uuid.UUID][]domain.Candidate
	purgedAt  []time.Time
	purgeErr  error
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{requests: map[uuid.UUID]*domain.Request{}, cands: map[uuid.UUID][]domain.Candidate{}}
}

func (r *fakeRepo) CreateRequest(ctx context.Context, req *domain.Request) error {
	if r.createErr != nil {
		return r.createErr
	}
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	cp := *req
	r.requests[req.ID] = &cp
	return nil
}

func (r *fakeRepo) withCounts(req domain.Request) domain.Request {
	cands := r.cands[req.ID]
	req.CandidatesTotal = len(cands)
	req.CandidatesResolved = 0
	for _, c := range cands {
		if c.MatchStatus != domain.MatchPending {
			req.CandidatesResolved++
		}
	}
	return req
}

func (r *fakeRepo) GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	req, ok := r.requests[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := r.withCounts(*req)
	out.Candidates = append([]domain.Candidate{}, r.cands[id]...)
	return &out, nil
}

func (r *fakeRepo) ListRecentRequests(ctx context.Context, limit int) ([]domain.Request, error) {
	out := make([]domain.Request, 0)
	for _, req := range r.requests {
		out = append(out, r.withCounts(*req))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) FindActiveRequest(ctx context.Context, userID uuid.UUID, staleBefore time.Time) (*domain.Request, error) {
	for _, req := range r.requests {
		if req.CreatedBy != nil && *req.CreatedBy == userID && domain.IsActiveStatus(req.Status) && req.UpdatedAt.After(staleBefore) {
			cp := *req
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *fakeRepo) CountRequestsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	for _, req := range r.requests {
		if req.CreatedBy != nil && *req.CreatedBy == userID && req.CreatedAt.After(since) {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error {
	if req, ok := r.requests[id]; ok && domain.IsActiveStatus(req.Status) {
		req.Status, req.UpdatedAt = domain.StatusRunning, now
	}
	return nil
}

func (r *fakeRepo) MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error {
	if req, ok := r.requests[id]; ok {
		c := code
		req.Status, req.ErrorCode, req.UpdatedAt, req.FinishedAt = domain.StatusFailed, &c, now, &now
	}
	return nil
}

func (r *fakeRepo) MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error {
	if req, ok := r.requests[id]; ok {
		req.Status, req.UpdatedAt, req.FinishedAt = domain.StatusDone, now, &now
	}
	return nil
}

func (r *fakeRepo) SaveModelResult(ctx context.Context, id uuid.UUID, model string, in, out int, cands []domain.NewCandidate, now time.Time) error {
	req := r.requests[id]
	req.Model, req.InputTokens, req.OutputTokens, req.UpdatedAt = model, in, out, now
	for i, c := range cands {
		r.cands[id] = append(r.cands[id], domain.Candidate{
			ID: uuid.New(), RequestID: id, Position: i, Name: c.Name, Note: c.Note,
			MatchStatus: domain.MatchPending, Options: []domain.Option{}, UpdatedAt: now,
		})
	}
	return nil
}

func (r *fakeRepo) ListPendingCandidates(ctx context.Context, id uuid.UUID) ([]domain.Candidate, error) {
	var out []domain.Candidate
	for _, c := range r.cands[id] {
		if c.MatchStatus == domain.MatchPending {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *fakeRepo) SaveCandidateResolution(ctx context.Context, reqID, candID uuid.UUID, status string, options []domain.Option, now time.Time) error {
	if options == nil {
		options = []domain.Option{}
	}
	for i := range r.cands[reqID] {
		if r.cands[reqID][i].ID == candID {
			r.cands[reqID][i].MatchStatus = status
			r.cands[reqID][i].Options = options
		}
	}
	r.requests[reqID].UpdatedAt = now
	return nil
}

func (r *fakeRepo) PurgeOlderThan(ctx context.Context, t time.Time) (int64, error) {
	r.purgedAt = append(r.purgedAt, t)
	if r.purgeErr != nil {
		return 0, r.purgeErr
	}
	var n int64
	for id, req := range r.requests {
		if req.CreatedAt.Before(t) {
			delete(r.requests, id)
			delete(r.cands, id)
			n++
		}
	}
	return n, nil
}
