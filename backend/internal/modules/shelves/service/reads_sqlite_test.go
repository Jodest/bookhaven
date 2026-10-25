package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/google/uuid"
)

// Reading-history tests run the real service and repository SQL on SQLite.

var historyNow = time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)

func (e *journalEnv) fixClock(t time.Time) { e.svc.now = func() time.Time { return t } }

func str(s string) *string { return &s }

func day(s string) OptionalDate { return OptionalDate{Set: true, Value: str(s)} }

var null = OptionalDate{Set: true}

func (e *journalEnv) reads() []domain.BookRead {
	e.t.Helper()
	r, err := e.svc.ListReads(context.Background(), e.user, e.book)
	e.must(err)
	return r
}

func (e *journalEnv) addPast(start *string, finish string) *domain.BookRead {
	e.t.Helper()
	r, err := e.svc.AddPastRead(context.Background(), e.user, e.book, start, finish)
	e.must(err)
	return r
}

func (e *journalEnv) setAddedAt(typ domain.ShelfType, at time.Time) {
	e.t.Helper()
	if err := e.db.Exec("UPDATE shelf_items SET added_at = ? WHERE shelf_id = ?", at, e.shelf[typ]).Error; err != nil {
		e.t.Fatal(err)
	}
}

func (e *journalEnv) onShelf(typ domain.ShelfType) bool {
	var n int64
	e.db.Raw("SELECT COUNT(*) FROM shelf_items WHERE shelf_id = ? AND book_id = ?", e.shelf[typ], e.book).Scan(&n)
	return n > 0
}

func noon(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }

func TestReads_AddPastRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeWantToRead))
	r := e.addPast(str("2015-12-01"), "2015-12-31")
	if r.State() != domain.ReadStateFinished || !r.FinishedAt.Equal(noon(2015, 12, 31)) || !r.StartedAt.Equal(noon(2015, 12, 1)) {
		t.Fatalf("unexpected read %+v", r)
	}
	e.addPast(nil, "2015-04-20")
	got := e.reads()
	if len(got) != 2 || got[0].StartedAt != nil && got[1].StartedAt != nil {
		t.Fatalf("unexpected list %+v", got)
	}
	// Shelves unchanged.
	if !e.onShelf(domain.ShelfTypeWantToRead) || e.onShelf(domain.ShelfTypeRead) {
		t.Fatal("history add must not touch shelves")
	}
	sum, _ := e.svc.ReadSummary(context.Background(), e.user, e.book)
	if sum.Count != 2 {
		t.Fatalf("count = %d", sum.Count)
	}
}

func TestReads_AddKeepsOpenRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeReading))
	e.addPast(nil, "2020-01-01")
	if e.open() != 1 || !e.onShelf(domain.ShelfTypeReading) {
		t.Fatal("open read must stay")
	}
	sum, _ := e.svc.ReadSummary(context.Background(), e.user, e.book)
	if !sum.Rereading {
		t.Fatal("expected rereading")
	}
}

func TestReads_AddUnpublishedBook(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.db.Exec("UPDATE books SET status = 'draft'")
	_, err := e.svc.AddPastRead(context.Background(), e.user, e.book, nil, "2020-01-01")
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestReads_AddCap(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	for i := 0; i < MaxReadsPerBook; i++ {
		if err := e.db.Exec("INSERT INTO book_reads (id, user_id, book_id, finished_at, created_at) VALUES (?, ?, ?, ?, ?)",
			uuid.New(), e.user, e.book, historyNow, historyNow).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.svc.AddPastRead(context.Background(), e.user, e.book, nil, "2020-01-01")
	if !errors.Is(err, ErrTooManyReads) {
		t.Fatalf("got %v", err)
	}
}

func TestReads_DateRules(t *testing.T) {
	e := newJournalEnv(t)
	ctx := context.Background()
	// 23:00 UTC on 25 Oct is already 26 Oct in UTC+14.
	e.fixClock(time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC))
	cases := []struct {
		start *string
		end   string
		want  error
	}{
		{nil, "2026-10-26", nil},
		{nil, "2026-10-27", ErrDateInFuture},
		{nil, "1899-12-31", ErrInvalidDate},
		{nil, "1900-01-01", nil},
		{nil, "2020-02-30", ErrInvalidDate},
		{nil, "20-01-2020", ErrInvalidDate},
		{nil, "", ErrFinishRequired},
		{str("2020-05-10"), "2020-05-01", ErrFinishBeforeStart},
		{str("2020-05-03"), "2020-05-03", nil},
		{str("2026-10-27"), "2026-10-26", ErrDateInFuture},
	}
	for _, c := range cases {
		_, err := e.svc.AddPastRead(ctx, e.user, e.book, c.start, c.end)
		if !errors.Is(err, c.want) {
			t.Errorf("start=%v end=%q: got %v want %v", c.start, c.end, err, c.want)
		}
	}
}

func TestReads_EditOpenRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	ctx := context.Background()
	e.must(e.add(domain.ShelfTypeReading))
	id := e.reads()[0].ID
	if _, err := e.svc.UpdateRead(ctx, e.user, id, ReadPatch{FinishedOn: day("2026-10-01")}); !errors.Is(err, ErrFieldNotEditable) {
		t.Fatalf("finish on open: %v", err)
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, id, ReadPatch{AbandonedOn: day("2026-10-01")}); !errors.Is(err, ErrFieldNotEditable) {
		t.Fatalf("abandon on open: %v", err)
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, id, ReadPatch{StartedOn: null}); !errors.Is(err, ErrStartRequired) {
		t.Fatalf("null start on open: %v", err)
	}
	r, err := e.svc.UpdateRead(ctx, e.user, id, ReadPatch{StartedOn: day("2026-05-01")})
	e.must(err)
	if r.State() != domain.ReadStateOpen || !r.StartedAt.Equal(noon(2026, 5, 1)) || r.ManualEnd {
		t.Fatalf("unexpected %+v", r)
	}
	if !e.onShelf(domain.ShelfTypeReading) {
		t.Fatal("shelf must be unchanged")
	}
}

func TestReads_EditClosedRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	ctx := context.Background()
	fin := e.addPast(str("2020-05-01"), "2020-06-01")
	if _, err := e.svc.UpdateRead(ctx, e.user, fin.ID, ReadPatch{}); !errors.Is(err, ErrNothingToUpdate) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, fin.ID, ReadPatch{AbandonedOn: day("2020-06-02")}); !errors.Is(err, ErrFieldNotEditable) {
		t.Fatalf("abandon on finished: %v", err)
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, fin.ID, ReadPatch{FinishedOn: null}); !errors.Is(err, ErrFinishRequired) {
		t.Fatalf("null finish: %v", err)
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, fin.ID, ReadPatch{FinishedOn: day("2020-04-30")}); !errors.Is(err, ErrFinishBeforeStart) {
		t.Fatalf("end before stored start: %v", err)
	}
	r, err := e.svc.UpdateRead(ctx, e.user, fin.ID, ReadPatch{StartedOn: null})
	e.must(err)
	if r.StartedAt != nil || !r.FinishedAt.Equal(noon(2020, 6, 1)) {
		t.Fatalf("unexpected %+v", r)
	}

	// Abandoned row: a status move to «Брошено».
	e.must(e.add(domain.ShelfTypeReading))
	e.must(e.add(domain.ShelfTypeAbandoned))
	var ab domain.BookRead
	for _, x := range e.reads() {
		if x.State() == domain.ReadStateAbandoned {
			ab = x
		}
	}
	if _, err := e.svc.UpdateRead(ctx, e.user, ab.ID, ReadPatch{FinishedOn: day("2026-10-01")}); !errors.Is(err, ErrFieldNotEditable) {
		t.Fatalf("finish on abandoned: %v", err)
	}
	r, err = e.svc.UpdateRead(ctx, e.user, ab.ID, ReadPatch{AbandonedOn: day("2026-10-25")})
	e.must(err)
	if r.State() != domain.ReadStateAbandoned || !r.ManualEnd {
		t.Fatalf("unexpected %+v", r)
	}
}

func TestReads_ForeignAndMissing(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	ctx := context.Background()
	r := e.addPast(nil, "2020-01-01")
	other := uuid.New()
	if _, err := e.svc.UpdateRead(ctx, other, r.ID, ReadPatch{StartedOn: day("2019-01-01")}); !errors.Is(err, ErrReadNotFound) {
		t.Fatalf("update foreign: %v", err)
	}
	if err := e.svc.DeleteRead(ctx, other, r.ID); !errors.Is(err, ErrReadNotFound) {
		t.Fatalf("delete foreign: %v", err)
	}
	if len(e.reads()) != 1 {
		t.Fatal("foreign delete must keep the row")
	}
	list, err := e.svc.ListReads(ctx, other, e.book)
	e.must(err)
	if len(list) != 0 {
		t.Fatal("foreign list must be empty")
	}
}

func TestReads_UpdateStateChangedIs409(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeReading))
	id := e.reads()[0].ID
	// The conditional update runs against the state loaded earlier (open),
	// but the row was finished in between.
	e.must(e.add(domain.ShelfTypeRead))
	n, err := e.svc.repo.UpdateReadDates(context.Background(), e.user, id, domain.ReadStateOpen, map[string]any{"started_at": noon(2026, 5, 1)})
	e.must(err)
	if n != 0 {
		t.Fatalf("expected 0 rows, got %d", n)
	}
}

func TestReads_Delete(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	ctx := context.Background()
	e.must(e.add(domain.ShelfTypeReading))
	openID := e.reads()[0].ID
	if err := e.svc.DeleteRead(ctx, e.user, openID); !errors.Is(err, ErrOpenReadNotDeletable) {
		t.Fatalf("open delete: %v", err)
	}
	if e.open() != 1 {
		t.Fatal("open row must be kept")
	}
	closed := e.addPast(nil, "2020-01-01")
	e.must(e.svc.DeleteRead(ctx, e.user, closed.ID))
	if err := e.svc.DeleteRead(ctx, e.user, closed.ID); !errors.Is(err, ErrReadNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestReads_DeleteLastReadKeepsShelf(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeRead))
	e.must(e.svc.DeleteRead(context.Background(), e.user, e.reads()[0].ID))
	if !e.onShelf(domain.ShelfTypeRead) {
		t.Fatal("shelf must stay")
	}
	sum, _ := e.svc.ReadSummary(context.Background(), e.user, e.book)
	if sum.Count != 0 || sum.LastReadAt != nil || sum.Rereading {
		t.Fatalf("summary %+v", sum)
	}
}

func TestReads_ListOrder(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	late := e.addPast(str("2022-01-01"), "2022-02-01")
	early := e.addPast(str("2019-03-01"), "2019-04-01")
	noStart := e.addPast(nil, "2020-06-01")
	got := e.reads()
	if got[0].ID != early.ID || got[1].ID != noStart.ID || got[2].ID != late.ID {
		t.Fatalf("order %v", got)
	}
}

// ---- «Прочитано» undo ----

func TestUndo_DeletesStatusRecordedRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.addPast(nil, "2025-03-01")
	e.must(e.add(domain.ShelfTypeRead))
	e.setAddedAt(domain.ShelfTypeRead, historyNow)
	e.must(e.remove(domain.ShelfTypeRead))
	got := e.reads()
	if len(got) != 1 || !got[0].FinishedAt.Equal(noon(2025, 3, 1)) {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestUndo_BackfillAndGoClockRows(t *testing.T) {
	for name, delta := range map[string]time.Duration{"equal": 0, "go clock slightly earlier": -30 * time.Second} {
		t.Run(name, func(t *testing.T) {
			e := newJournalEnv(t)
			e.fixClock(historyNow)
			e.must(e.add(domain.ShelfTypeRead))
			e.setAddedAt(domain.ShelfTypeRead, historyNow.Add(-delta))
			e.must(e.remove(domain.ShelfTypeRead))
			if e.total() != 0 {
				t.Fatalf("expected the status-recorded read to be deleted, %d left", e.total())
			}
		})
	}
}

func TestUndo_AfterCurrentReadDeletedKeepsOlder(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.addPast(nil, "2025-03-01")
	e.must(e.add(domain.ShelfTypeRead))
	e.setAddedAt(domain.ShelfTypeRead, historyNow)
	for _, r := range e.reads() {
		if !r.ManualEnd {
			e.must(e.svc.DeleteRead(context.Background(), e.user, r.ID))
		}
	}
	e.must(e.remove(domain.ShelfTypeRead))
	got := e.reads()
	if len(got) != 1 || !got[0].FinishedAt.Equal(noon(2025, 3, 1)) {
		t.Fatalf("older read must be kept, got %+v", got)
	}
	if e.onShelf(domain.ShelfTypeRead) {
		t.Fatal("book must be off the shelf")
	}
}

func TestUndo_KeepsRedatedRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeRead))
	e.setAddedAt(domain.ShelfTypeRead, historyNow)
	_, err := e.svc.UpdateRead(context.Background(), e.user, e.reads()[0].ID, ReadPatch{FinishedOn: day("2026-09-10")})
	e.must(err)
	e.must(e.remove(domain.ShelfTypeRead))
	if got := e.reads(); len(got) != 1 || !got[0].FinishedAt.Equal(noon(2026, 9, 10)) {
		t.Fatalf("re-dated read must be kept, got %+v", got)
	}
}

func TestUndo_StartOnlyEditStillUndone(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeReading))
	_, err := e.svc.UpdateRead(context.Background(), e.user, e.reads()[0].ID, ReadPatch{StartedOn: day("2026-09-01")})
	e.must(err)
	e.must(e.add(domain.ShelfTypeRead))
	e.setAddedAt(domain.ShelfTypeRead, historyNow)
	e.must(e.remove(domain.ShelfTypeRead))
	if e.total() != 0 {
		t.Fatalf("expected the read to be undone, %d left", e.total())
	}
}

func TestUndo_SkipsHandAddedLaterRead(t *testing.T) {
	e := newJournalEnv(t)
	e.fixClock(historyNow)
	e.must(e.add(domain.ShelfTypeRead))
	e.setAddedAt(domain.ShelfTypeRead, historyNow)
	// finished on 25 Oct 2026 12:00 UTC, i.e. after the shelf add at 10:00
	e.addPast(nil, "2026-10-25")
	e.must(e.remove(domain.ShelfTypeRead))
	got := e.reads()
	if len(got) != 1 || !got[0].ManualEnd {
		t.Fatalf("hand-added read must be kept, got %+v", got)
	}
}
