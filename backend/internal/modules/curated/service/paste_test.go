package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/modules/curated/repository"
	"github.com/bookhaven/backend/internal/pkg/textnorm"
	"github.com/google/uuid"
)

// fakeRepo serves paste tests from memory; any other method panics (nil
// embedded interface), which keeps the tests honest about what they use.
type fakeRepo struct {
	repository.Repository
	collection *domain.Collection
	catalog    []catalogDomain.Book
	held       []uuid.UUID
	queries    []string
	addCalls   int
	addResult  repository.BulkAdded
	addErr     error
	addIDs     []uuid.UUID
	addPos     *int
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Collection, error) {
	if f.collection == nil || f.collection.ID != id {
		return nil, domain.ErrCollectionNotFound
	}
	return f.collection, nil
}

func (f *fakeRepo) CollectionBookIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return f.held, nil
}

// TitleCandidates mimics the word query: books containing every key word.
func (f *fakeRepo) TitleCandidates(_ context.Context, key string, limit int) ([]catalogDomain.Book, error) {
	f.queries = append(f.queries, key)
	var out []catalogDomain.Book
	for _, b := range f.catalog {
		tk := " " + textnorm.Key(b.Title) + " "
		ok := true
		for _, w := range strings.Fields(key) {
			if !strings.Contains(tk, " "+w+" ") {
				ok = false
			}
		}
		if ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeRepo) BooksByIDs(_ context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	var out []catalogDomain.Book
	for _, id := range ids {
		for _, b := range f.catalog {
			if b.ID == id {
				out = append(out, b)
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) AddBooks(_ context.Context, _ uuid.UUID, ids []uuid.UUID, pos *int) (repository.BulkAdded, error) {
	f.addCalls++
	f.addIDs, f.addPos = ids, pos
	if f.addErr != nil {
		return repository.BulkAdded{}, f.addErr
	}
	if f.addResult.Added == nil && len(ids) > 0 && len(f.addResult.AlreadyIn) == 0 {
		var res repository.BulkAdded
		for i, id := range ids {
			res.Added = append(res.Added, repository.AddedBook{BookID: id, Position: len(f.held) + i + 1})
		}
		res.Total = len(f.held) + len(ids)
		return res, nil
	}
	return f.addResult, nil
}

func cbook(title, status string, authors ...string) catalogDomain.Book {
	b := catalogDomain.Book{ID: uuid.New(), Title: title, Slug: uuid.NewString(), Status: status}
	for _, a := range authors {
		b.Authors = append(b.Authors, catalogDomain.Author{ID: uuid.New(), Name: a})
	}
	return b
}

func newFake(books ...catalogDomain.Book) (*Service, *fakeRepo) {
	f := &fakeRepo{collection: &domain.Collection{ID: uuid.New()}, catalog: books}
	return NewService(f), f
}

const (
	pub  = catalogDomain.BookStatusPublished
	pend = catalogDomain.BookStatusPending
	rej  = catalogDomain.BookStatusRejected
)

func TestPreviewPaste_ThreeFoundWritesNothing(t *testing.T) {
	a, b, c := cbook("Мать", pub, "Максим Горький"), cbook("Война и мир", pub, "Лев Толстой"), cbook("1984", pend, "Джордж Оруэлл")
	s, f := newFake(a, b, c)
	p, err := s.PreviewPaste(context.Background(), f.collection.ID, "1. Мать — Горький\n«Война и мир»\n1984 — Оруэлл")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lines) != 3 || p.Counts[domain.PasteFound] != 3 || p.FreeSlots != domain.MaxBooks {
		t.Fatalf("preview: %+v", p)
	}
	if p.Lines[2].Book == nil || p.Lines[2].Book.Book.Status != pend {
		t.Fatalf("pending book must be found: %+v", p.Lines[2])
	}
	if f.addCalls != 0 {
		t.Fatal("preview must not write")
	}
}

func TestPreviewPaste_BlankLinesKeepNumbering(t *testing.T) {
	s, f := newFake(cbook("Мать", pub), cbook("Война и мир", pub))
	p, err := s.PreviewPaste(context.Background(), f.collection.ID, "Мать\n\n  \nВойна и мир\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lines) != 2 || p.Lines[0].Line != 1 || p.Lines[1].Line != 4 {
		t.Fatalf("lines: %+v", p.Lines)
	}
}

func TestPreviewPaste_Limits(t *testing.T) {
	s, f := newFake()
	id := f.collection.ID
	if _, err := s.PreviewPaste(context.Background(), id, strings.Repeat("Мать\n", 501)); !errors.Is(err, domain.ErrTooManyLines) {
		t.Fatalf("501 lines: %v", err)
	}
	if _, err := s.PreviewPaste(context.Background(), id, strings.Repeat("Мать\n", 500)); err != nil {
		t.Fatalf("500 lines: %v", err)
	}
	if _, err := s.PreviewPaste(context.Background(), id, strings.Repeat("я", domain.MaxPasteChars+1)); !isValidation(err) {
		t.Fatalf("long text: %v", err)
	}
	if _, err := s.PreviewPaste(context.Background(), id, " \n\n "); !isValidation(err) {
		t.Fatalf("blank text: %v", err)
	}
	if _, err := s.PreviewPaste(context.Background(), uuid.New(), "Мать"); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("unknown collection: %v", err)
	}
}

func TestPreviewPaste_Statuses(t *testing.T) {
	gorky, capek := cbook("Мать", pub, "Максим Горький"), cbook("Мать", pub, "Карел Чапек")
	elka := cbook("Ёлка: повесть", pub, "А")
	moscow := cbook("Москва - Петушки", pub, "Венедикт Ерофеев")
	rejected := cbook("Запрещённая", rej, "Б")
	held := cbook("Удержанная", pub, "В")
	s, f := newFake(gorky, capek, elka, moscow, rejected, held)
	f.held = []uuid.UUID{held.ID}
	text := strings.Join([]string{
		"Мать",                   // 1 ambiguous
		"Мать — М. Горький",      // 2 found gorky
		"Мать — Толстой",         // 3 ambiguous, author mismatch
		"ЕЛКА ПОВЕСТЬ",           // 4 found
		"Москва - Петушки",       // 5 found by whole-line retry
		"Нет такой книги",        // 6 not found
		"Запрещённая",            // 7 rejected
		"Удержанная",             // 8 in collection
		"7.",                     // 9 invalid
		"Мать — Горький",         // 10 duplicate of 2 (same book)
		"Нет такой книги",        // 11 duplicate of 6 (same text)
		"«Ёлка: повесть» — А. Б", // 12 elka again by title, author mismatch -> ambiguous
	}, "\n")
	p, err := s.PreviewPaste(context.Background(), f.collection.ID, text)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.PasteStatus{
		domain.PasteAmbiguous, domain.PasteFound, domain.PasteAmbiguous, domain.PasteFound,
		domain.PasteFound, domain.PasteNotFound, domain.PasteRejected, domain.PasteInCollection,
		domain.PasteInvalid, domain.PasteDuplicate, domain.PasteDuplicate, domain.PasteAmbiguous,
	}
	for i, w := range want {
		if p.Lines[i].Status != w {
			t.Errorf("line %d: status %s, want %s", i+1, p.Lines[i].Status, w)
		}
	}
	if len(p.Lines[0].Candidates) != 2 || len(p.Lines[2].Candidates) != 2 {
		t.Errorf("ambiguous candidates: %d %d", len(p.Lines[0].Candidates), len(p.Lines[2].Candidates))
	}
	if p.Lines[1].Book.Book.ID != gorky.ID || p.Lines[1].Author == nil || *p.Lines[1].Author != "М. Горький" {
		t.Errorf("line 2: %+v", p.Lines[1])
	}
	if p.Lines[4].Book.Book.ID != moscow.ID || p.Lines[4].Author != nil || p.Lines[4].Title != "Москва - Петушки" {
		t.Errorf("line 5: %+v", p.Lines[4])
	}
	if l := p.Lines[6]; len(l.Candidates) != 1 || l.Candidates[0].Book.Status != rej {
		t.Errorf("line 7: %+v", l)
	}
	if l := p.Lines[7]; l.Book == nil || !l.Book.InCollection {
		t.Errorf("line 8: %+v", l)
	}
	if d := p.Lines[9].DuplicateOf; d == nil || *d != 2 {
		t.Errorf("line 10 duplicate_of: %v", d)
	}
	if d := p.Lines[10].DuplicateOf; d == nil || *d != 6 {
		t.Errorf("line 11 duplicate_of: %v", d)
	}
	if p.Lines[9].Book != nil || len(p.Lines[9].Candidates) != 0 {
		t.Errorf("duplicate keeps no book: %+v", p.Lines[9])
	}
	if p.Counts[domain.PasteFound] != 3 || p.Counts[domain.PasteDuplicate] != 2 || p.Counts[domain.PasteInvalid] != 1 || p.BooksCount != 1 {
		t.Errorf("counts: %+v", p)
	}
}

func TestPreviewPaste_OneQueryPerDistinctKey(t *testing.T) {
	s, f := newFake(cbook("Мать", pub))
	if _, err := s.PreviewPaste(context.Background(), f.collection.ID, "Мать\nмать!\n«Мать» — Горький"); err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 1 || f.queries[0] != "мать" {
		t.Fatalf("queries: %v", f.queries)
	}
}

func TestPreviewPaste_ExactTitleOnly(t *testing.T) {
	s, f := newFake(cbook("Мать и дочь", pub))
	p, err := s.PreviewPaste(context.Background(), f.collection.ID, "Мать")
	if err != nil {
		t.Fatal(err)
	}
	if p.Lines[0].Status != domain.PasteNotFound {
		t.Fatalf("a longer title must not match: %+v", p.Lines[0])
	}
}

func TestPreviewPaste_FreeSlots(t *testing.T) {
	s, f := newFake()
	for i := 0; i < 990; i++ {
		f.held = append(f.held, uuid.New())
	}
	p, err := s.PreviewPaste(context.Background(), f.collection.ID, "Мать")
	if err != nil || p.BooksCount != 990 || p.FreeSlots != 10 {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestBulkAdd_MixedRequest(t *testing.T) {
	good, rejected, held := cbook("A", pub), cbook("B", rej), cbook("C", pub)
	s, f := newFake(good, rejected, held)
	f.addResult = repository.BulkAdded{
		Added:     []repository.AddedBook{{BookID: good.ID, Position: 3}},
		AlreadyIn: []uuid.UUID{held.ID},
		Total:     3,
	}
	missing := uuid.New()
	res, err := s.BulkAdd(context.Background(), f.collection.ID, []uuid.UUID{good.ID, rejected.ID, held.ID, missing, good.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0].BookID != good.ID || res.BooksCount != 3 {
		t.Fatalf("added: %+v", res)
	}
	got := map[uuid.UUID]string{}
	for _, sk := range res.Skipped {
		got[sk.BookID] = sk.Reason
	}
	if got[rejected.ID] != SkipRejected || got[held.ID] != SkipAlreadyIn || got[missing] != SkipNotFound || len(res.Skipped) != 4 {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
	if res.Skipped[len(res.Skipped)-1].Reason != SkipDuplicate {
		t.Fatalf("repeat must be reported as duplicate: %+v", res.Skipped)
	}
	if len(f.addIDs) != 2 || f.addIDs[0] != good.ID || f.addIDs[1] != held.ID {
		t.Fatalf("repo must get the addable ids in order: %v", f.addIDs)
	}
}

func TestBulkAdd_PassesPositionAndPending(t *testing.T) {
	a, b := cbook("A", pend), cbook("B", pub)
	s, f := newFake(a, b)
	pos := 2
	res, err := s.BulkAdd(context.Background(), f.collection.ID, []uuid.UUID{a.ID, b.ID}, &pos)
	if err != nil || len(res.Added) != 2 || len(res.Skipped) != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if f.addPos == nil || *f.addPos != 2 {
		t.Fatal("position not passed")
	}
}

func TestBulkAdd_Validation(t *testing.T) {
	s, f := newFake()
	ctx, id := context.Background(), f.collection.ID
	if _, err := s.BulkAdd(ctx, id, nil, nil); !isValidation(err) {
		t.Errorf("empty: %v", err)
	}
	many := make([]uuid.UUID, 501)
	for i := range many {
		many[i] = uuid.New()
	}
	if _, err := s.BulkAdd(ctx, id, many, nil); !isValidation(err) {
		t.Errorf("501 ids: %v", err)
	}
	zero := 0
	if _, err := s.BulkAdd(ctx, id, many[:1], &zero); !errors.Is(err, domain.ErrInvalidPosition) {
		t.Errorf("position 0: %v", err)
	}
	if _, err := s.BulkAdd(ctx, uuid.New(), many[:1], nil); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Errorf("unknown collection: %v", err)
	}
}

func TestBulkAdd_FullPassesThrough(t *testing.T) {
	a := cbook("A", pub)
	s, f := newFake(a)
	f.addErr = domain.ErrCollectionFull
	if _, err := s.BulkAdd(context.Background(), f.collection.ID, []uuid.UUID{a.ID}, nil); !errors.Is(err, domain.ErrCollectionFull) {
		t.Fatalf("got %v", err)
	}
}
