package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// fakeModerationRepository overrides the author moderation methods of
// fakeRepository with in-memory behavior.
type fakeModerationRepository struct {
	*fakeRepository

	rows        []repository.AuthorModerationRow
	samples     map[uuid.UUID][]repository.SampleBook
	candidates  []catalogDomain.Author
	statuses    map[uuid.UUID]string
	setErr      map[uuid.UUID]error
	setCalls    []uuid.UUID
	lastFilter  repository.AuthorModerationFilter
	lastSurname []string
	lastExclude []uuid.UUID
	findCalls   int
	sampleCalls int
}

func newModerationService() (*Service, *fakeModerationRepository) {
	repo := &fakeModerationRepository{
		fakeRepository: &fakeRepository{authorsExist: true, genresExist: true},
		statuses:       map[uuid.UUID]string{},
		setErr:         map[uuid.UUID]error{},
	}
	return NewService(repo), repo
}

func (f *fakeModerationRepository) ListAuthorsForModeration(ctx context.Context, filter repository.AuthorModerationFilter) ([]repository.AuthorModerationRow, int64, error) {
	f.lastFilter = filter
	return f.rows, int64(len(f.rows)), nil
}

func (f *fakeModerationRepository) ListAuthorSampleBooks(ctx context.Context, authorIDs []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	f.sampleCalls++
	return f.samples, nil
}

func (f *fakeModerationRepository) FindAuthorsBySurnames(ctx context.Context, surnames []string, excludeIDs []uuid.UUID) ([]catalogDomain.Author, error) {
	f.findCalls++
	f.lastSurname = surnames
	f.lastExclude = excludeIDs
	return f.candidates, nil
}

func (f *fakeModerationRepository) SetAuthorStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	f.setCalls = append(f.setCalls, id)
	if err := f.setErr[id]; err != nil {
		return false, err
	}
	if _, ok := f.statuses[id]; !ok {
		return false, nil
	}
	f.statuses[id] = status
	return true, nil
}

func queueAuthor(name, status string) catalogDomain.Author {
	return catalogDomain.Author{ID: uuid.New(), Name: name, Status: status}
}

func TestNormalizeAuthorName(t *testing.T) {
	cases := map[string]string{
		"Лев Толстой":      "лев толстой",
		"  Лев   ТОЛСТОЙ ": "лев толстой",
		"Пётр Ёлкин":       "петр елкин",
		"Л. Н. Толстой":    "л н толстой",
		"Толстой, Лев":     "толстой лев",
		"Салтыков-Щедрин":  "салтыков щедрин",
		"O'Henry (pseud.)": "o henry pseud",
		"Агата Кристи 2":   "агата кристи 2",
		"...":              "",
	}
	for in, want := range cases {
		if got := normalizeAuthorName(in); got != want {
			t.Errorf("normalizeAuthorName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAuthorSurname(t *testing.T) {
	cases := map[string]string{
		"Лев Толстой":       "толстой",
		"Толстой, Лев":      "толстой",
		"Толстой , Лев":     "толстой",
		"де Ла Фонтен, Жан": "де ла фонтен",
		"Л. Н. Толстой":     "толстой",
		"Гомер":             "гомер",
		"Пётр Ёлкин":        "елкин",
		"Ли Бо":             "",
		"Ли, Бо":            "",
		"":                  "",
		", Лев":             "",
	}
	for in, want := range cases {
		if got := authorSurname(in); got != want {
			t.Errorf("authorSurname(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFirstInitial(t *testing.T) {
	cases := map[string]rune{
		"Лев Толстой":   'л',
		"Л. Н. Толстой": 'л',
		"Толстой, Лев":  'л',
		"Гомер":         0,
		"Толстой,":      0,
	}
	for in, want := range cases {
		if got := firstInitial(in); got != want {
			t.Errorf("firstInitial(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchKind(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"Лев Толстой", "лев  толстой", MatchExactName},
		{"Пётр Ёлкин", "Петр Елкин", MatchExactName},
		{"Л. Н. Толстой", "Лев Толстой", MatchSurnameInitial},
		{"Толстой, Лев", "Лев Николаевич Толстой", MatchSurnameInitial},
		{"Лев Толстой", "Алексей Толстой", MatchSurname},
		{"Толстой", "Лев Толстой", MatchSurname},
		{"Лев Толстой", "Лев Тургенев", ""},
		{"Ли Бо", "Ли Бо", ""},
	}
	for _, tc := range cases {
		if got := matchKind(tc.a, tc.b); got != tc.want {
			t.Errorf("matchKind(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBuildAuthorHints_OrderingExclusionsAndCap(t *testing.T) {
	row := queueAuthor("Л. Н. Толстой", catalogDomain.AuthorStatusPending)
	exactPending := queueAuthor("Л Н Толстой", catalogDomain.AuthorStatusPending)
	initialPublished := queueAuthor("Лев Толстой", catalogDomain.AuthorStatusPublished)
	initialPending := queueAuthor("Лев Николаевич Толстой", catalogDomain.AuthorStatusPending)
	surnameB := queueAuthor("Борис Толстой", catalogDomain.AuthorStatusPublished)
	surnameA := queueAuthor("Алексей Толстой", catalogDomain.AuthorStatusPublished)
	surnameC := queueAuthor("Вадим Толстой", catalogDomain.AuthorStatusPublished)
	rejected := queueAuthor("Лев Толстой", catalogDomain.AuthorStatusRejected)
	other := queueAuthor("Иван Тургенев", catalogDomain.AuthorStatusPublished)

	candidates := []catalogDomain.Author{surnameC, surnameB, rejected, other, initialPending, row, surnameA, initialPublished, exactPending}
	hints := buildAuthorHints(row, candidates)

	want := []struct {
		id    uuid.UUID
		match string
	}{
		{exactPending.ID, MatchExactName},
		{initialPublished.ID, MatchSurnameInitial},
		{initialPending.ID, MatchSurnameInitial},
		{surnameA.ID, MatchSurname},
		{surnameB.ID, MatchSurname},
	}
	if len(hints) != len(want) {
		t.Fatalf("expected %d hints (cap), got %d: %+v", len(want), len(hints), hints)
	}
	for i, w := range want {
		if hints[i].Author.ID != w.id || hints[i].Match != w.match {
			t.Fatalf("hint %d = %s/%s, want %s/%s", i, hints[i].Author.Name, hints[i].Match, w.id, w.match)
		}
	}
}

func TestBuildAuthorHints_NeverNil(t *testing.T) {
	hints := buildAuthorHints(queueAuthor("Ли Бо", catalogDomain.AuthorStatusPending), []catalogDomain.Author{queueAuthor("Ли Бо", catalogDomain.AuthorStatusPublished)})
	if hints == nil || len(hints) != 0 {
		t.Fatalf("expected empty non-nil hints, got %#v", hints)
	}
}

func TestListAuthorModerationQueue_InvalidFilters(t *testing.T) {
	cases := []repository.AuthorModerationFilter{
		{Status: "deleted"},
		{QID: "maybe"},
		{Books: "some"},
	}
	for _, f := range cases {
		svc, _ := newModerationService()
		_, _, err := svc.ListAuthorModerationQueue(context.Background(), f, 1, 20)
		if !errors.Is(err, ErrInvalidModerationFilter) {
			t.Fatalf("filter %+v: expected ErrInvalidModerationFilter, got %v", f, err)
		}
	}
}

func TestListAuthorModerationQueue_DefaultsAndLimitClamp(t *testing.T) {
	svc, repo := newModerationService()
	if _, _, err := svc.ListAuthorModerationQueue(context.Background(), repository.AuthorModerationFilter{QID: "with", Books: "none", Search: "x"}, 3, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := repo.lastFilter
	if f.Status != catalogDomain.AuthorStatusPending || f.QID != "with" || f.Books != "none" || f.Search != "x" {
		t.Fatalf("unexpected filter passed through: %+v", f)
	}
	if f.Limit != 50 || f.Offset != 100 {
		t.Fatalf("expected limit 50 offset 100, got %d/%d", f.Limit, f.Offset)
	}
	if repo.sampleCalls != 0 || repo.findCalls != 0 {
		t.Fatal("empty page must not run extra queries")
	}
}

func TestListAuthorModerationQueue_AssemblesContextAndHints(t *testing.T) {
	svc, repo := newModerationService()
	pendingTolstoy := queueAuthor("Л. Н. Толстой", catalogDomain.AuthorStatusPending)
	pageMate := queueAuthor("Лев Толстой", catalogDomain.AuthorStatusPending)
	shortName := queueAuthor("Ли Бо", catalogDomain.AuthorStatusPending)
	published := queueAuthor("Алексей Толстой", catalogDomain.AuthorStatusPublished)
	bookID := uuid.New()

	repo.rows = []repository.AuthorModerationRow{
		{Author: pendingTolstoy, BooksTotal: 3, BooksVisible: 1},
		{Author: pageMate},
		{Author: shortName},
	}
	repo.samples = map[uuid.UUID][]repository.SampleBook{
		pendingTolstoy.ID: {{ID: bookID, Title: "Война и мир", IsHidden: true}},
	}
	repo.candidates = []catalogDomain.Author{published}

	items, total, err := svc.ListAuthorModerationQueue(context.Background(), repository.AuthorModerationFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected 3 items, got %d (total %d)", len(items), total)
	}
	if repo.sampleCalls != 1 || repo.findCalls != 1 {
		t.Fatalf("expected one sample and one candidate query, got %d/%d", repo.sampleCalls, repo.findCalls)
	}
	if len(repo.lastSurname) != 1 || repo.lastSurname[0] != "толстой" {
		t.Fatalf("expected distinct surnames [толстой], got %v", repo.lastSurname)
	}
	if len(repo.lastExclude) != 3 {
		t.Fatalf("expected page ids excluded from the prefilter, got %v", repo.lastExclude)
	}

	first := items[0]
	if first.BooksTotal != 3 || first.BooksVisible != 1 || len(first.SampleBooks) != 1 || first.SampleBooks[0].ID != bookID {
		t.Fatalf("unexpected book context: %+v", first)
	}
	if len(first.Similar) != 2 ||
		first.Similar[0].Author.ID != pageMate.ID || first.Similar[0].Match != MatchSurnameInitial ||
		first.Similar[1].Author.ID != published.ID || first.Similar[1].Match != MatchSurname {
		t.Fatalf("unexpected hints: %+v", first.Similar)
	}
	if items[1].SampleBooks == nil || len(items[1].SampleBooks) != 0 {
		t.Fatalf("expected empty non-nil sample books, got %#v", items[1].SampleBooks)
	}
	if items[2].Similar == nil || len(items[2].Similar) != 0 {
		t.Fatalf("expected empty non-nil hints for a short surname, got %#v", items[2].Similar)
	}
}

func TestBulkSetAuthorStatus_AllSucceed(t *testing.T) {
	svc, repo := newModerationService()
	a, b := uuid.New(), uuid.New()
	repo.statuses[a] = catalogDomain.AuthorStatusPending
	repo.statuses[b] = catalogDomain.AuthorStatusPublished // no-op still counts

	report, err := svc.BulkSetAuthorStatus(context.Background(), []uuid.UUID{a, b}, catalogDomain.AuthorStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 2 || report.Updated != 2 || len(report.Errors) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if repo.statuses[a] != catalogDomain.AuthorStatusPublished {
		t.Fatalf("expected a published, got %q", repo.statuses[a])
	}
}

func TestBulkSetAuthorStatus_UnknownAndFailingRowsContinue(t *testing.T) {
	svc, repo := newModerationService()
	known, unknown, failing := uuid.New(), uuid.New(), uuid.New()
	repo.statuses[known] = catalogDomain.AuthorStatusPending
	repo.statuses[failing] = catalogDomain.AuthorStatusPending
	repo.setErr[failing] = errors.New("boom")

	report, err := svc.BulkSetAuthorStatus(context.Background(), []uuid.UUID{unknown, failing, known}, catalogDomain.AuthorStatusRejected)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 3 || report.Updated != 1 || len(report.Errors) != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Errors[0].AuthorID != unknown.String() || report.Errors[0].Message != "author not found" {
		t.Fatalf("unexpected not-found row error: %+v", report.Errors[0])
	}
	if report.Errors[1].AuthorID != failing.String() || report.Errors[1].Message != "boom" {
		t.Fatalf("unexpected failing row error: %+v", report.Errors[1])
	}
	if repo.statuses[known] != catalogDomain.AuthorStatusRejected {
		t.Fatalf("expected the known author rejected, got %q", repo.statuses[known])
	}
}

func TestBulkSetAuthorStatus_InvalidStatus(t *testing.T) {
	svc, repo := newModerationService()
	_, err := svc.BulkSetAuthorStatus(context.Background(), []uuid.UUID{uuid.New()}, "deleted")
	if !errors.Is(err, ErrInvalidAuthorStatus) {
		t.Fatalf("expected ErrInvalidAuthorStatus, got %v", err)
	}
	if len(repo.setCalls) != 0 {
		t.Fatal("nothing must be updated")
	}
}

func TestBulkSetAuthorStatus_TooLargeChangesNothing(t *testing.T) {
	svc, repo := newModerationService()
	ids := make([]uuid.UUID, bulkAuthorStatusCap+1)
	for i := range ids {
		ids[i] = uuid.New()
		repo.statuses[ids[i]] = catalogDomain.AuthorStatusPending
	}
	_, err := svc.BulkSetAuthorStatus(context.Background(), ids, catalogDomain.AuthorStatusPublished)
	if !errors.Is(err, ErrBulkStatusTooLarge) {
		t.Fatalf("expected ErrBulkStatusTooLarge, got %v", err)
	}
	if len(repo.setCalls) != 0 {
		t.Fatal("nothing must be updated")
	}
}

func TestBulkSetAuthorStatus_DedupesBeforeCap(t *testing.T) {
	svc, repo := newModerationService()
	id := uuid.New()
	repo.statuses[id] = catalogDomain.AuthorStatusPending
	ids := make([]uuid.UUID, bulkAuthorStatusCap+50)
	for i := range ids {
		ids[i] = id
	}
	report, err := svc.BulkSetAuthorStatus(context.Background(), ids, catalogDomain.AuthorStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 1 || report.Updated != 1 || len(repo.setCalls) != 1 {
		t.Fatalf("expected one deduped update, got %+v (%d calls)", report, len(repo.setCalls))
	}
}

func TestBulkSetAuthorStatus_EmptyList(t *testing.T) {
	svc, _ := newModerationService()
	report, err := svc.BulkSetAuthorStatus(context.Background(), nil, catalogDomain.AuthorStatusPublished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 0 || report.Updated != 0 || report.Errors == nil {
		t.Fatalf("unexpected report: %#v", report)
	}
}
