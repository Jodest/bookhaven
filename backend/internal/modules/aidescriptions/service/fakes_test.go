package service

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

// fakeLLM never reaches the network; it answers from a queue (the last
// answer repeats) and records requests.
type fakeLLM struct {
	resps    []llm.StructuredResponse
	errs     []error
	requests []llm.StructuredRequest
}

func (f *fakeLLM) Structured(ctx context.Context, req llm.StructuredRequest) (llm.StructuredResponse, error) {
	i := len(f.requests)
	f.requests = append(f.requests, req)
	if i < len(f.errs) && f.errs[i] != nil {
		return llm.StructuredResponse{}, f.errs[i]
	}
	if len(f.resps) == 0 {
		return answer(`{"known":true,"description":"` + validText + `"}`), nil
	}
	if i >= len(f.resps) {
		i = len(f.resps) - 1
	}
	return f.resps[i], nil
}

const validText = "Роман рассказывает о молодом враче, который приезжает в далёкую уездную больницу и учится работать в одиночку."

func answer(json string) llm.StructuredResponse {
	return llm.StructuredResponse{JSON: []byte(json), StopReason: llm.StopReasonEndTurn, Model: "claude-sonnet-5-5", InputTokens: 1050, OutputTokens: 600}
}

// fakeBooks serves books by id.
type fakeBooks struct {
	books map[uuid.UUID]catalogDomain.Book
	err   error
	calls int
}

func (f *fakeBooks) LoadBooksWithRelations(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := []catalogDomain.Book{}
	for _, id := range ids {
		if b, ok := f.books[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func booksOf(bs ...catalogDomain.Book) *fakeBooks {
	f := &fakeBooks{books: map[uuid.UUID]catalogDomain.Book{}}
	for _, b := range bs {
		f.books[b.ID] = b
	}
	return f
}

// fakeRepo keeps records in memory.
type fakeRepo struct {
	records     map[uuid.UUID]domain.Item
	missing     []uuid.UUID
	upserts     []domain.Description
	approveErr  error
	approveText *string
	discardErr  error
	listItems   []domain.Item
	listArgs    []any
}

func newFakeRepo() *fakeRepo { return &fakeRepo{records: map[uuid.UUID]domain.Item{}} }

func (f *fakeRepo) Upsert(ctx context.Context, d domain.Description) error {
	f.upserts = append(f.upserts, d)
	f.records[d.BookID] = domain.Item{Description: d}
	return nil
}

func (f *fakeRepo) Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error) {
	it, ok := f.records[bookID]
	if !ok {
		return nil, nil
	}
	return &it, nil
}

func (f *fakeRepo) List(ctx context.Context, status string, page, limit int) ([]domain.Item, int64, error) {
	f.listArgs = []any{status, page, limit}
	return f.listItems, int64(len(f.listItems)), nil
}

func (f *fakeRepo) CountDrafts(ctx context.Context) (int64, error) { return 6, nil }
func (f *fakeRepo) CountMissing(ctx context.Context) (int64, error) {
	return int64(len(f.missing)), nil
}

func (f *fakeRepo) ListMissingCandidateIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit < len(f.missing) {
		return f.missing[:limit], nil
	}
	return f.missing, nil
}

func (f *fakeRepo) Approve(ctx context.Context, bookID, userID uuid.UUID, text *string, now time.Time) error {
	f.approveText = text
	if f.approveErr != nil {
		return f.approveErr
	}
	it := f.records[bookID]
	it.Status = domain.StatusApproved
	f.records[bookID] = it
	return nil
}

func (f *fakeRepo) Discard(ctx context.Context, bookID, userID uuid.UUID, now time.Time) error {
	if f.discardErr != nil {
		return f.discardErr
	}
	it := f.records[bookID]
	it.Status = domain.StatusDiscarded
	f.records[bookID] = it
	return nil
}

func strp(s string) *string { return &s }

func book(status string, description *string) catalogDomain.Book {
	year := 1925
	return catalogDomain.Book{
		ID: uuid.New(), Title: "Записки юного врача", Status: status, Language: "ru", PublishedYear: &year,
		Description: description,
		Authors: []catalogDomain.Author{
			{ID: uuid.New(), Name: "Михаил Булгаков"},
			{ID: uuid.New(), Name: "Аноним"},
		},
		Genres: []catalogDomain.Genre{
			{ID: uuid.New(), Name: "повесть"},
			{ID: uuid.New(), Name: "автобиография"},
		},
	}
}
