package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Book moderation queue (docs/features/book-moderation-queue.md).

func (f *fakeRepository) ListBooksForModeration(ctx context.Context, filter repository.BookModerationFilter) ([]catalogDomain.Book, int64, error) {
	f.lastBookModFilter = filter
	return f.bookModRows, int64(len(f.bookModRows)), nil
}

// FindBooksByNormalizedISBNs mirrors the repository: normalized ISBN in
// isbns, id not excluded.
func (f *fakeRepository) FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	f.isbnHintCalls++
	f.lastHintISBNs = isbns
	f.lastHintExclude = excludeIDs
	out := []repository.BookHintCandidate{}
	for _, c := range f.isbnCandidates {
		if c.ISBN == nil || containsUUID(excludeIDs, c.ID) {
			continue
		}
		norm := normalizeISBN(*c.ISBN)
		for _, v := range isbns {
			if v == norm {
				out = append(out, c)
				break
			}
		}
	}
	return out, nil
}

// FindBooksByAuthorIDs mirrors the repository: candidates sharing an
// author id, AuthorIDs narrowed to the requested ones.
func (f *fakeRepository) FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]repository.BookHintCandidate, error) {
	f.authorHintCalls++
	f.lastHintAuthorIDs = authorIDs
	out := []repository.BookHintCandidate{}
	for _, c := range f.authorCandidates {
		if containsUUID(excludeIDs, c.ID) {
			continue
		}
		var matched []uuid.UUID
		for _, id := range c.AuthorIDs {
			if containsUUID(authorIDs, id) {
				matched = append(matched, id)
			}
		}
		if len(matched) > 0 {
			c.AuthorIDs = matched
			out = append(out, c)
		}
	}
	return out, nil
}

func queueBook(title, status string, isbn *string, authors ...uuid.UUID) catalogDomain.Book {
	b := catalogDomain.Book{ID: uuid.New(), Title: title, Slug: uuid.NewString(), Status: status, ISBN: isbn}
	for _, id := range authors {
		b.Authors = append(b.Authors, catalogDomain.Author{ID: id, Name: "Автор", Status: catalogDomain.AuthorStatusPending})
	}
	return b
}

func hintCandidate(title, status string, isbn *string, authors ...uuid.UUID) repository.BookHintCandidate {
	return repository.BookHintCandidate{ID: uuid.New(), Title: title, Slug: uuid.NewString(), Status: status, ISBN: isbn, AuthorIDs: authors}
}

func TestNormalizeISBN(t *testing.T) {
	cases := map[string]string{
		"978-0-306-40615-7": "9780306406157",
		"0 306 40615 2":     "0306406152",
		"080442957x":        "080442957X",
		"12345":             "",
		"978-0-306-40615":   "",
		"":                  "",
	}
	for in, want := range cases {
		if got := normalizeISBN(in); got != want {
			t.Errorf("normalizeISBN(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestISBNVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"0306406152", []string{"0306406152", "9780306406157"}},
		{"9780306406157", []string{"9780306406157", "0306406152"}},
		{"080442957X", []string{"080442957X", "9780804429573"}},
		{"9780804429573", []string{"9780804429573", "080442957X"}},
		{"9791090636071", []string{"9791090636071"}},
		{"", nil},
	}
	for _, c := range cases {
		if got := isbnVariants(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("isbnVariants(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeBookTitle(t *testing.T) {
	cases := map[string]string{
		"Война и мир":        "война и мир",
		"  ВОЙНА  и  МИР! ":  "война и мир",
		"Ёжик в тумане":      "ежик в тумане",
		"Мастер и Маргарита": "мастер и маргарита",
		"...":                "",
	}
	for in, want := range cases {
		if got := normalizeBookTitle(in); got != want {
			t.Errorf("normalizeBookTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBookMatchKind(t *testing.T) {
	a1, a2 := uuid.New(), uuid.New()
	book := hintCandidate("Война и мир", "pending", strPtr("978-0-306-40615-7"), a1)
	cases := []struct {
		name string
		c    repository.BookHintCandidate
		want string
	}{
		{"same isbn plain", hintCandidate("Другое", "published", strPtr("9780306406157")), BookMatchSameISBN},
		{"isbn10 vs 978-13", hintCandidate("Другое", "published", strPtr("0-306-40615-2")), BookMatchSameISBN},
		{"title and shared author", hintCandidate("ВОЙНА И МИР", "published", nil, a2, a1), BookMatchSameTitleAuthor},
		{"title without shared author", hintCandidate("Война и мир", "published", nil, a2), ""},
		{"title, no authors", hintCandidate("Война и мир", "published", nil), ""},
		{"different isbn and title", hintCandidate("Анна Каренина", "published", strPtr("9791090636071"), a1), ""},
		{"both kinds: isbn wins", hintCandidate("Война и мир", "published", strPtr("9780306406157"), a1), BookMatchSameISBN},
	}
	for _, c := range cases {
		if got := bookMatchKind(book, c.c); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	noAuthors := hintCandidate("Война и мир", "pending", nil)
	if got := bookMatchKind(noAuthors, hintCandidate("Война и мир", "published", nil, a1)); got != "" {
		t.Errorf("book without authors must not get a title hint, got %q", got)
	}
	emptyTitle := hintCandidate("!!!", "pending", nil, a1)
	if got := bookMatchKind(emptyTitle, hintCandidate("???", "published", nil, a1)); got != "" {
		t.Errorf("empty normalized title must not match, got %q", got)
	}
}

func TestBuildBookHints_OrderingCapAndDedupe(t *testing.T) {
	a := uuid.New()
	isbn := strPtr("9780306406157")
	book := hintCandidate("Война и мир", "pending", isbn, a)

	rejectedISBN := hintCandidate("Б", "rejected", isbn)
	publishedISBN := hintCandidate("Я", "published", isbn)
	pendingISBN := hintCandidate("А", "pending", isbn)
	titleA := hintCandidate("война и мир", "published", nil, a)
	titleB := hintCandidate("Война и мир", "pending", nil, a)
	titleC := hintCandidate("Война и мир", "rejected", nil, a)
	unrelated := hintCandidate("Анна Каренина", "published", nil, a)

	// publishedISBN appears twice (once without the ISBN, as an author
	// candidate) and is reported once as same_isbn.
	dupAsTitle := publishedISBN
	dupAsTitle.ISBN = nil
	dupAsTitle.Title = "Война и мир"
	dupAsTitle.AuthorIDs = []uuid.UUID{a}

	hints := buildBookHints(book, []repository.BookHintCandidate{
		book, titleC, dupAsTitle, rejectedISBN, titleB, unrelated, publishedISBN, pendingISBN, titleA,
	})
	if len(hints) != maxBookHints {
		t.Fatalf("expected %d hints, got %d: %+v", maxBookHints, len(hints), hints)
	}
	want := []struct {
		id    uuid.UUID
		match string
	}{
		{publishedISBN.ID, BookMatchSameISBN},
		{pendingISBN.ID, BookMatchSameISBN},
		{rejectedISBN.ID, BookMatchSameISBN},
		{titleA.ID, BookMatchSameTitleAuthor},
		{titleB.ID, BookMatchSameTitleAuthor},
	}
	for i, w := range want {
		if hints[i].Book.ID != w.id || hints[i].Match != w.match {
			t.Fatalf("hint %d: got %s/%s, want %s/%s", i, hints[i].Book.Title, hints[i].Match, w.id, w.match)
		}
	}

	none := buildBookHints(book, []repository.BookHintCandidate{book, unrelated})
	if none == nil || len(none) != 0 {
		t.Fatalf("expected empty non-nil hints, got %#v", none)
	}
}

func TestListBookModerationQueue_DefaultsAndValidation(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	items, total, err := svc.ListBookModerationQueue(ctx, repository.BookModerationFilter{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastBookModFilter.Status != catalogDomain.BookStatusPending || repo.lastBookModFilter.Limit != bookQueueDefaultLimit || repo.lastBookModFilter.Offset != 0 {
		t.Fatalf("unexpected defaults: %+v", repo.lastBookModFilter)
	}
	if items == nil || len(items) != 0 || total != 0 {
		t.Fatalf("expected empty non-nil page, got %#v total=%d", items, total)
	}
	if repo.isbnHintCalls != 0 || repo.authorHintCalls != 0 {
		t.Fatalf("empty page must not query hint candidates: isbn=%d author=%d", repo.isbnHintCalls, repo.authorHintCalls)
	}

	if _, _, err := svc.ListBookModerationQueue(ctx, repository.BookModerationFilter{Status: "rejected", Source: "harvested"}, 3, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := repo.lastBookModFilter
	if f.Status != "rejected" || f.Source != "harvested" || f.Limit != bookQueueMaxLimit || f.Offset != 2*bookQueueMaxLimit {
		t.Fatalf("unexpected filter: %+v", f)
	}

	for _, bad := range []repository.BookModerationFilter{{Status: "bogus"}, {Source: "bogus"}} {
		if _, _, err := svc.ListBookModerationQueue(ctx, bad, 1, 20); !errors.Is(err, ErrInvalidModerationFilter) {
			t.Fatalf("filter %+v: expected ErrInvalidModerationFilter, got %v", bad, err)
		}
	}
}

func TestListBookModerationQueue_Hints(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()
	author, other := uuid.New(), uuid.New()

	row := queueBook("Война и мир", catalogDomain.BookStatusPending, strPtr("0-306-40615-2"), author)
	pageDup := queueBook("ВОЙНА И МИР", catalogDomain.BookStatusPending, nil, author)
	lonely := queueBook("Анна Каренина", catalogDomain.BookStatusPending, nil)
	repo.bookModRows = []catalogDomain.Book{row, pageDup, lonely}

	rejectedISBN := hintCandidate("Война и мир (старое издание)", catalogDomain.BookStatusRejected, strPtr("978 0 306 40615 7"))
	titleOther := hintCandidate("Война и мир", catalogDomain.BookStatusPublished, nil, other)
	both := hintCandidate("Война и мир", catalogDomain.BookStatusPublished, strPtr("9780306406157"), author)
	repo.isbnCandidates = []repository.BookHintCandidate{rejectedISBN, both}
	repo.authorCandidates = []repository.BookHintCandidate{titleOther, both}

	items, total, err := svc.ListBookModerationQueue(ctx, repository.BookModerationFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("unexpected page: total=%d items=%d", total, len(items))
	}
	if repo.isbnHintCalls != 1 || repo.authorHintCalls != 1 {
		t.Fatalf("expected one query each, got isbn=%d author=%d", repo.isbnHintCalls, repo.authorHintCalls)
	}
	if !reflect.DeepEqual(repo.lastHintISBNs, []string{"0306406152", "9780306406157"}) {
		t.Fatalf("unexpected isbn set: %v", repo.lastHintISBNs)
	}
	if !reflect.DeepEqual(repo.lastHintAuthorIDs, []uuid.UUID{author}) {
		t.Fatalf("unexpected author set: %v", repo.lastHintAuthorIDs)
	}
	if !reflect.DeepEqual(repo.lastHintExclude, []uuid.UUID{row.ID, pageDup.ID, lonely.ID}) {
		t.Fatalf("unexpected exclude ids: %v", repo.lastHintExclude)
	}

	got := items[0].Similar
	want := []struct {
		id    uuid.UUID
		match string
	}{
		{both.ID, BookMatchSameISBN},
		{rejectedISBN.ID, BookMatchSameISBN},
		{pageDup.ID, BookMatchSameTitleAuthor},
	}
	if len(got) != len(want) {
		t.Fatalf("row hints: expected %d, got %+v", len(want), got)
	}
	for i, w := range want {
		if got[i].Book.ID != w.id || got[i].Match != w.match {
			t.Fatalf("row hint %d: got %s/%s, want %s/%s", i, got[i].Book.ID, got[i].Match, w.id, w.match)
		}
	}

	// The page duplicate hints back at the row and at the shared-author
	// candidate; titleOther (no shared author) is never a hint.
	dupHints := items[1].Similar
	if len(dupHints) != 2 || dupHints[0].Book.ID != both.ID || dupHints[1].Book.ID != row.ID ||
		dupHints[0].Match != BookMatchSameTitleAuthor || dupHints[1].Match != BookMatchSameTitleAuthor {
		t.Fatalf("unexpected page-duplicate hints: %+v", dupHints)
	}

	if items[2].Similar == nil || len(items[2].Similar) != 0 {
		t.Fatalf("expected [] for a row without hints, got %#v", items[2].Similar)
	}
	if items[0].Book.ID != row.ID || len(items[0].Book.Authors) != 1 {
		t.Fatalf("book not passed through: %+v", items[0].Book)
	}
}

func TestListBookModerationQueue_NoISBNsNoAuthors(t *testing.T) {
	svc, repo := newService()
	repo.bookModRows = []catalogDomain.Book{queueBook("Стихи", catalogDomain.BookStatusPending, strPtr("12-34"))}
	if _, _, err := svc.ListBookModerationQueue(context.Background(), repository.BookModerationFilter{}, 1, 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.lastHintISBNs) != 0 || len(repo.lastHintAuthorIDs) != 0 {
		t.Fatalf("expected empty candidate sets, got isbns=%v authors=%v", repo.lastHintISBNs, repo.lastHintAuthorIDs)
	}
}
