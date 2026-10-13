package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// AI verdict repository tests. SQLite-native version of migration 000022.
const bookAIVerdictDDL = `
CREATE TABLE book_ai_verdicts (
	book_id TEXT PRIMARY KEY,
	verdict TEXT NOT NULL,
	reason_codes TEXT NOT NULL DEFAULT '[]',
	reason TEXT NOT NULL DEFAULT '',
	confidence REAL NOT NULL DEFAULT 0,
	model TEXT NOT NULL,
	prompt_version INTEGER NOT NULL,
	input_hash TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

func newAIVerdictRepo(t *testing.T) *bookModFixture {
	t.Helper()
	f := newBookModRepo(t)
	if err := f.db.Exec(bookAIVerdictDDL).Error; err != nil {
		t.Fatalf("create verdict table: %v", err)
	}
	return f
}

// verdictAt inserts a verdict row with an explicit created_at.
func (f *bookModFixture) verdictAt(bookID uuid.UUID, verdict string, promptVersion int, at time.Time) {
	f.t.Helper()
	if err := f.db.Exec("INSERT INTO book_ai_verdicts (book_id, verdict, model, prompt_version, input_hash, created_at) VALUES (?, ?, 'm', ?, 'h', ?)",
		bookID, verdict, promptVersion, at).Error; err != nil {
		f.t.Fatalf("insert verdict: %v", err)
	}
}

func TestUpsertBookAIVerdict_InsertAndReplace(t *testing.T) {
	f := newAIVerdictRepo(t)
	ctx := context.Background()
	book := f.book("Война и мир", catalogDomain.BookStatusPending, nil, nil, false)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	f.verdictAt(book, "needs_human", 1, old)

	err := f.repo.UpsertBookAIVerdict(ctx, BookAIVerdict{
		BookID: book, Verdict: "publish", ReasonCodes: "[]", Reason: "Всё в порядке.", Confidence: 0.95,
		Model: "claude-haiku-5-5", PromptVersion: 2, InputHash: "abc", InputTokens: 1200, OutputTokens: 90,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var n int64
	f.db.Table("book_ai_verdicts").Count(&n)
	if n != 1 {
		t.Fatalf("expected exactly one verdict, got %d", n)
	}
	got, err := f.repo.GetBookAIVerdicts(ctx, []uuid.UUID{book, uuid.New()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	v, ok := got[book]
	if !ok || len(got) != 1 {
		t.Fatalf("unexpected verdicts: %+v", got)
	}
	if v.Verdict != "publish" || v.Reason != "Всё в порядке." || v.Confidence != 0.95 || v.PromptVersion != 2 ||
		v.InputHash != "abc" || v.InputTokens != 1200 || v.OutputTokens != 90 || v.Model != "claude-haiku-5-5" {
		t.Fatalf("verdict not replaced: %+v", v)
	}
	if !v.CreatedAt.After(old) {
		t.Fatalf("created_at must be refreshed, got %v", v.CreatedAt)
	}

	empty, err := f.repo.GetBookAIVerdicts(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty ids: %v %v", empty, err)
	}
}

func TestListAIReviewCandidateIDs_OrderAndPrefilter(t *testing.T) {
	f := newAIVerdictRepo(t)
	ctx := context.Background()

	oldWithFreshVerdict := f.book("Свежий вердикт", catalogDomain.BookStatusPending, nil, nil, false)
	oldPromptVerdict := f.book("Старая версия", catalogDomain.BookStatusPending, nil, nil, false)
	noVerdictOld := f.book("Без вердикта 1", catalogDomain.BookStatusPending, nil, nil, false)
	updatedAfter := f.book("Изменена", catalogDomain.BookStatusPending, nil, nil, false)
	noVerdictNew := f.book("Без вердикта 2", catalogDomain.BookStatusPending, nil, nil, false)
	f.book("Опубликована", catalogDomain.BookStatusPublished, nil, nil, false)
	f.book("Отклонена", catalogDomain.BookStatusRejected, nil, nil, false)

	later := f.base.Add(time.Hour)
	f.verdictAt(oldWithFreshVerdict, "publish", 1, later)
	f.verdictAt(oldPromptVerdict, "publish", 0, later)
	f.verdictAt(updatedAfter, "reject", 1, later)
	if err := f.db.Exec("UPDATE books SET updated_at = ? WHERE id = ?", later.Add(time.Minute), updatedAfter).Error; err != nil {
		t.Fatalf("touch book: %v", err)
	}

	ids, err := f.repo.ListAIReviewCandidateIDs(ctx, 10, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []uuid.UUID{noVerdictOld, noVerdictNew, oldPromptVerdict, updatedAfter}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("candidates = %v, want %v", ids, want)
	}
	ids, err = f.repo.ListAIReviewCandidateIDs(ctx, 1, 1)
	if err != nil || len(ids) != 1 || ids[0] != noVerdictOld {
		t.Fatalf("limit 1 = %v %v", ids, err)
	}
	n, err := f.repo.CountAIReviewCandidates(ctx, 1)
	if err != nil || n != 4 {
		t.Fatalf("count = %d %v", n, err)
	}
}

func TestListBooksForModeration_AIFilter(t *testing.T) {
	f := newAIVerdictRepo(t)
	ctx := context.Background()
	publish := f.book("П", catalogDomain.BookStatusPending, nil, nil, false)
	reject := f.book("О", catalogDomain.BookStatusPending, nil, nil, false)
	human := f.book("Ч", catalogDomain.BookStatusPending, nil, nil, false)
	none := f.book("Н", catalogDomain.BookStatusPending, nil, nil, false)
	publishedPublish := f.book("Опубл", catalogDomain.BookStatusPublished, nil, nil, false)
	at := f.base.Add(time.Hour)
	f.verdictAt(publish, "publish", 1, at)
	f.verdictAt(reject, "reject", 1, at)
	f.verdictAt(human, "needs_human", 0, at)
	f.verdictAt(publishedPublish, "publish", 1, at)
	author := f.author("Автор", catalogDomain.AuthorStatusPublished)
	f.link(publish, []uuid.UUID{author}, nil)

	cases := map[string][]uuid.UUID{
		"publish":     {publish},
		"reject":      {reject},
		"needs_human": {human},
		"none":        {none},
		"":            {none, human, reject, publish},
	}
	for ai, want := range cases {
		books, total, err := f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusPending, AI: ai})
		if err != nil {
			t.Fatalf("ai=%q: %v", ai, err)
		}
		if got := bookIDs(books); total != int64(len(want)) || !reflect.DeepEqual(got, want) {
			t.Fatalf("ai=%q: total=%d got %v, want %v", ai, total, got, want)
		}
		if ai == "publish" && len(books[0].Authors) != 1 {
			t.Fatalf("authors must stay preloaded with the join, got %+v", books[0].Authors)
		}
	}
}

func TestLoadBooksForReview(t *testing.T) {
	f := newAIVerdictRepo(t)
	ctx := context.Background()
	book := f.book("Война и мир", catalogDomain.BookStatusPublished, nil, nil, false)
	author := f.author("Лев Толстой", catalogDomain.AuthorStatusPublished)
	genre := f.genre("роман", catalogDomain.GenreStatusPublished)
	f.link(book, []uuid.UUID{author}, []uuid.UUID{genre})

	books, err := f.repo.LoadBooksForReview(ctx, []uuid.UUID{book, uuid.New()})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(books) != 1 || len(books[0].Authors) != 1 || len(books[0].Genres) != 1 || books[0].Status != catalogDomain.BookStatusPublished {
		t.Fatalf("unexpected books: %+v", books)
	}
	if empty, err := f.repo.LoadBooksForReview(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty ids: %v %v", empty, err)
	}
}
