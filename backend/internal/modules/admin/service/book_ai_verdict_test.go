package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	aimodDomain "github.com/bookhaven/backend/internal/modules/aimoderation/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// AI verdict repository methods of the service fakes.

func (f *fakeRepository) UpsertBookAIVerdict(ctx context.Context, v repository.BookAIVerdict) error {
	if f.upsertAIVerdictErr != nil {
		return f.upsertAIVerdictErr
	}
	if f.aiVerdicts == nil {
		f.aiVerdicts = map[uuid.UUID]repository.BookAIVerdict{}
	}
	v.CreatedAt = time.Now()
	f.aiVerdicts[v.BookID] = v
	return nil
}

func (f *fakeRepository) GetBookAIVerdicts(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID]repository.BookAIVerdict, error) {
	out := map[uuid.UUID]repository.BookAIVerdict{}
	for _, id := range bookIDs {
		if v, ok := f.aiVerdicts[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (f *fakeRepository) ListAIReviewCandidateIDs(ctx context.Context, limit, promptVersion int) ([]uuid.UUID, error) {
	f.lastAICandidateLimit, f.lastAIPromptVersion = limit, promptVersion
	return f.aiCandidateIDs, nil
}

func (f *fakeRepository) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	f.lastAIPromptVersion = promptVersion
	return f.aiCandidateCount, nil
}

func (f *fakeRepository) LoadBooksForReview(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	out := []catalogDomain.Book{}
	for _, b := range f.reviewBooks {
		if containsUUID(ids, b.ID) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeImportRepository) UpsertBookAIVerdict(ctx context.Context, v repository.BookAIVerdict) error {
	return nil
}

func (f *fakeImportRepository) GetBookAIVerdicts(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID]repository.BookAIVerdict, error) {
	return map[uuid.UUID]repository.BookAIVerdict{}, nil
}

func (f *fakeImportRepository) ListAIReviewCandidateIDs(ctx context.Context, limit, promptVersion int) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeImportRepository) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	return 0, nil
}

func (f *fakeImportRepository) LoadBooksForReview(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) UpsertBookAIVerdict(ctx context.Context, v repository.BookAIVerdict) error {
	return nil
}

func (f *fakeMergeAuthorsRepository) GetBookAIVerdicts(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID]repository.BookAIVerdict, error) {
	return map[uuid.UUID]repository.BookAIVerdict{}, nil
}

func (f *fakeMergeAuthorsRepository) ListAIReviewCandidateIDs(ctx context.Context, limit, promptVersion int) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeMergeAuthorsRepository) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	return 0, nil
}

func (f *fakeMergeAuthorsRepository) LoadBooksForReview(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	return nil, nil
}

func reviewBook() catalogDomain.Book {
	desc := "Роман-эпопея."
	return catalogDomain.Book{
		ID: uuid.New(), Title: "Война и мир", Slug: "voyna-i-mir", Status: catalogDomain.BookStatusPending,
		Language: "ru", Description: &desc,
		Authors: []catalogDomain.Author{
			{ID: uuid.New(), Name: "Лев Толстой", Status: catalogDomain.AuthorStatusPublished},
			{ID: uuid.New(), Name: "Аноним", Status: catalogDomain.AuthorStatusPending},
		},
		Genres: []catalogDomain.Genre{
			{ID: uuid.New(), Name: "роман", Status: catalogDomain.GenreStatusPublished},
			{ID: uuid.New(), Name: "история", Status: catalogDomain.GenreStatusPublished},
		},
	}
}

func TestInputHash_StableAndSensitive(t *testing.T) {
	b := reviewBook()
	base := InputHash(BookReviewDataOf(b))
	if len(base) != 64 {
		t.Fatalf("hash length = %d", len(base))
	}

	reordered := b
	reordered.Authors = []catalogDomain.Author{b.Authors[1], b.Authors[0]}
	reordered.Genres = []catalogDomain.Genre{b.Genres[1], b.Genres[0]}
	if got := InputHash(BookReviewDataOf(reordered)); got != base {
		t.Error("hash must not depend on author/genre order")
	}

	retitled := b
	retitled.Title = "Война и мир. Том 1"
	if InputHash(BookReviewDataOf(retitled)) == base {
		t.Error("a title edit must change the hash")
	}

	renamed := b
	renamed.Authors = append([]catalogDomain.Author(nil), b.Authors...)
	renamed.Authors[0].Name = "Л. Н. Толстой"
	if InputHash(BookReviewDataOf(renamed)) == base {
		t.Error("an author rename must change the hash")
	}

	data := BookReviewDataOf(b)
	if data.Source != "manual" || !reflect.DeepEqual(data.Genres, []string{"история", "роман"}) || data.Authors[0].Name != "Аноним" {
		t.Errorf("unexpected data: %+v", data)
	}
}

func TestBookReviewDataOf_TruncatesDescriptionAndSource(t *testing.T) {
	b := reviewBook()
	long := strings.Repeat("ж", 2000)
	src, ext := "wikidata", "Q161531"
	b.Description, b.ExternalSource, b.ExternalID = &long, &src, &ext
	data := BookReviewDataOf(b)
	if n := len([]rune(data.Description)); n != reviewDescriptionMaxRunes {
		t.Errorf("description runes = %d", n)
	}
	if data.Source != "wikidata" || data.ExternalID == nil || *data.ExternalID != "Q161531" {
		t.Errorf("source = %q / %v", data.Source, data.ExternalID)
	}
}

func TestBookReviewInputs_OrderHintsAndHash(t *testing.T) {
	svc, repo := newService()
	a := reviewBook()
	b := reviewBook()
	b.Title = "Анна Каренина"
	b.Status = catalogDomain.BookStatusPublished
	isbn := "978-5-17-090593-3"
	a.ISBN = &isbn
	repo.reviewBooks = []catalogDomain.Book{a, b}
	dup := hintCandidate("Война и мир", catalogDomain.BookStatusPublished, strPtr("9785170905933"))
	repo.isbnCandidates = []repository.BookHintCandidate{dup}

	got, err := svc.BookReviewInputs(context.Background(), []uuid.UUID{b.ID, uuid.New(), a.ID, b.ID})
	if err != nil {
		t.Fatalf("BookReviewInputs: %v", err)
	}
	if len(got) != 2 || got[0].BookID != b.ID || got[1].BookID != a.ID {
		t.Fatalf("unexpected order: %+v", got)
	}
	if got[0].Status != catalogDomain.BookStatusPublished {
		t.Errorf("status = %q", got[0].Status)
	}
	if len(got[1].Input.Similar) != 1 || got[1].Input.Similar[0].Match != BookMatchSameISBN || got[0].Input.Similar == nil {
		t.Errorf("hints = %+v / %+v", got[1].Input.Similar, got[0].Input.Similar)
	}
	if got[1].InputHash != InputHash(BookReviewDataOf(a)) {
		t.Error("hash must be the hash of the book's own data")
	}
}

func TestSaveBookAIVerdict(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	err := svc.SaveBookAIVerdict(context.Background(), BookAIVerdictRecord{
		BookID: id, Verdict: aimodDomain.VerdictReject, ReasonCodes: []string{aimodDomain.ReasonCollection},
		Reason: "Сборник.", Confidence: 0.876, Model: "claude-haiku-5-5", PromptVersion: 1, InputHash: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatalf("SaveBookAIVerdict: %v", err)
	}
	row := repo.aiVerdicts[id]
	if row.ReasonCodes != `["collection"]` || row.Confidence != 0.88 || row.Model != "claude-haiku-5-5" {
		t.Errorf("row = %+v", row)
	}
	if err := svc.SaveBookAIVerdict(context.Background(), BookAIVerdictRecord{BookID: id, Verdict: aimodDomain.VerdictPublish, Model: "m", InputHash: "h"}); err != nil {
		t.Fatalf("nil codes: %v", err)
	}
	if repo.aiVerdicts[id].ReasonCodes != "[]" {
		t.Errorf("nil codes stored as %q", repo.aiVerdicts[id].ReasonCodes)
	}
	for _, bad := range []BookAIVerdictRecord{
		{BookID: id, Verdict: "maybe", Model: "m", InputHash: "h"},
		{BookID: id, Verdict: aimodDomain.VerdictPublish, Confidence: 1.5, Model: "m", InputHash: "h"},
		{BookID: id, Verdict: aimodDomain.VerdictReject, ReasonCodes: []string{"bogus"}, Model: "m", InputHash: "h"},
	} {
		if err := svc.SaveBookAIVerdict(context.Background(), bad); !errors.Is(err, ErrInvalidAIVerdict) {
			t.Errorf("%+v: expected ErrInvalidAIVerdict, got %v", bad, err)
		}
	}
}

func TestListBookModerationQueue_AIVerdicts(t *testing.T) {
	svc, repo := newService()
	fresh, stalePrompt, staleHash, none := reviewBook(), reviewBook(), reviewBook(), reviewBook()
	repo.bookModRows = []catalogDomain.Book{fresh, stalePrompt, staleHash, none}
	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	verdict := func(b catalogDomain.Book, version int, hash string) repository.BookAIVerdict {
		return repository.BookAIVerdict{BookID: b.ID, Verdict: aimodDomain.VerdictPublish, ReasonCodes: "[]", Reason: "ok",
			Confidence: 0.93, Model: "claude-haiku-5-5", PromptVersion: version, InputHash: hash, CreatedAt: at}
	}
	edited := staleHash
	edited.Title = "Старое название"
	repo.aiVerdicts = map[uuid.UUID]repository.BookAIVerdict{
		fresh.ID:       verdict(fresh, aimodDomain.PromptVersion, InputHash(BookReviewDataOf(fresh))),
		stalePrompt.ID: verdict(stalePrompt, aimodDomain.PromptVersion+1, InputHash(BookReviewDataOf(stalePrompt))),
		staleHash.ID:   verdict(staleHash, aimodDomain.PromptVersion, InputHash(BookReviewDataOf(edited))),
	}

	items, _, err := svc.ListBookModerationQueue(context.Background(), repository.BookModerationFilter{AI: aimodDomain.VerdictPublish}, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if repo.lastBookModFilter.AI != aimodDomain.VerdictPublish {
		t.Errorf("ai filter not passed: %+v", repo.lastBookModFilter)
	}
	v := items[0].AIVerdict
	if v == nil || v.Stale || v.Verdict != "publish" || v.Confidence != 0.93 || v.ReasonCodes == nil || !v.CreatedAt.Equal(at) {
		t.Fatalf("fresh verdict = %+v", v)
	}
	if items[1].AIVerdict == nil || !items[1].AIVerdict.Stale {
		t.Error("prompt version mismatch must be stale")
	}
	if items[2].AIVerdict == nil || !items[2].AIVerdict.Stale {
		t.Error("input hash mismatch must be stale")
	}
	if items[3].AIVerdict != nil {
		t.Error("a book without a verdict must have nil")
	}

	for _, ok := range []string{"", "none", "publish", "reject", "needs_human"} {
		if _, _, err := svc.ListBookModerationQueue(context.Background(), repository.BookModerationFilter{AI: ok}, 1, 20); err != nil {
			t.Errorf("ai=%q: %v", ok, err)
		}
	}
	if _, _, err := svc.ListBookModerationQueue(context.Background(), repository.BookModerationFilter{AI: "bogus"}, 1, 20); !errors.Is(err, ErrInvalidModerationFilter) {
		t.Errorf("ai=bogus: expected ErrInvalidModerationFilter, got %v", err)
	}
}

func TestAIReviewCandidates(t *testing.T) {
	svc, repo := newService()
	id := uuid.New()
	repo.aiCandidateIDs, repo.aiCandidateCount = []uuid.UUID{id}, 7
	ids, err := svc.ListAIReviewCandidates(context.Background(), 50, 3)
	if err != nil || len(ids) != 1 || repo.lastAICandidateLimit != 50 || repo.lastAIPromptVersion != 3 {
		t.Fatalf("list = %v %v (limit %d, version %d)", ids, err, repo.lastAICandidateLimit, repo.lastAIPromptVersion)
	}
	n, err := svc.CountAIReviewCandidates(context.Background(), 1)
	if err != nil || n != 7 {
		t.Fatalf("count = %d %v", n, err)
	}
}
