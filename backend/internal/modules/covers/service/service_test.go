package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/bookhaven/backend/internal/modules/covers/repository"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const booksDDL = `
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	slug TEXT NOT NULL DEFAULT '',
	cover_url TEXT,
	status TEXT NOT NULL DEFAULT 'published',
	created_at DATETIME,
	updated_at DATETIME,
	cover_source_url TEXT,
	cover_path TEXT,
	cover_status TEXT,
	cover_error TEXT,
	cover_attempts INTEGER NOT NULL DEFAULT 0,
	cover_next_attempt_at DATETIME,
	cover_stored_at DATETIME,
	cover_checked_at DATETIME
);`

const screenWorksDDL = `
CREATE TABLE screen_works (
	id TEXT PRIMARY KEY,
	created_at DATETIME,
	updated_at DATETIME,
	poster_source TEXT,
	poster_url TEXT,
	poster_path TEXT,
	poster_author TEXT,
	poster_license TEXT,
	poster_license_url TEXT,
	poster_source_url TEXT,
	poster_updated_at DATETIME,
	poster_copy_source_url TEXT,
	poster_copy_path TEXT,
	poster_copy_status TEXT,
	poster_copy_error TEXT,
	poster_copy_attempts INTEGER NOT NULL DEFAULT 0,
	poster_copy_next_attempt_at DATETIME,
	poster_copy_fetched_at DATETIME,
	poster_copy_checked_at DATETIME,
	poster_tmdb_ref TEXT,
	poster_verified_at DATETIME,
	poster_refresh_at DATETIME
);`

type fakeFetcher struct {
	calls []string
	fn    func(url string) (*coverfetch.Result, error)
}

func (f *fakeFetcher) Fetch(_ context.Context, u string) (*coverfetch.Result, error) {
	f.calls = append(f.calls, u)
	return f.fn(u)
}

func jpeg(tag string) *coverfetch.Result {
	return &coverfetch.Result{Data: append([]byte{0xFF, 0xD8, 0xFF}, []byte(tag)...), Ext: "jpg"}
}

type fixture struct {
	t       *testing.T
	db      *gorm.DB
	store   *coverstore.MemStore
	fetcher *fakeFetcher
	svc     *Service
	now     time.Time
	slept   []time.Duration
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(booksDDL).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(screenWorksDDL).Error; err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, db: db, store: coverstore.NewMem(), now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	f.store.Now = func() time.Time { return f.now }
	f.fetcher = &fakeFetcher{fn: func(string) (*coverfetch.Result, error) { return jpeg("default"), nil }}
	f.svc = New(repository.NewGormRepository(db), f.store, f.fetcher, nil,
		WithClock(func() time.Time { return f.now }, func(_ context.Context, d time.Duration) error {
			f.slept = append(f.slept, d)
			f.now = f.now.Add(d)
			return nil
		}))
	return f
}

func (f *fixture) book(cover string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var c interface{}
	if cover != "" {
		c = cover
	}
	if err := f.db.Exec("INSERT INTO books (id, title, slug, cover_url, created_at, updated_at) VALUES (?, 'T', ?, ?, ?, ?)",
		id, id.String(), c, f.now, f.now).Error; err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) load(id uuid.UUID) catalogDomain.Book {
	f.t.Helper()
	var b catalogDomain.Book
	if err := f.db.First(&b, "id = ?", id).Error; err != nil {
		f.t.Fatal(err)
	}
	return b
}

func (f *fixture) storedFiles() []string {
	var out []string
	_ = f.store.List(context.Background(), func(n string, _ time.Time) error { out = append(out, n); return nil })
	return out
}

func transient() error { return &coverfetch.Error{Code: coverfetch.CodeHTTPError, Transient: true} }

const ol = "https://covers.openlibrary.org/b/id/1-L.jpg"

func TestMirrorOne_StoresAndDisplays(t *testing.T) {
	f := newFixture(t)
	id := f.book(ol)
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	b := f.load(id)
	if !b.HasStoredCover() {
		t.Fatalf("not stored: %+v", b)
	}
	d := b.DisplayCoverURL()
	if d == nil || !strings.HasPrefix(*d, "/covers/") || !coverstore.ValidName((*d)[len("/covers/"):]) {
		t.Fatalf("display: %v", d)
	}
	if b.CoverStoredAt == nil || b.CoverAttempts != 0 || b.CoverError != nil {
		t.Fatalf("state: %+v", b)
	}
	if got := f.storedFiles(); len(got) != 1 {
		t.Fatalf("files: %v", got)
	}
	// Stored copy of the same link: nothing more to do.
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil || len(f.fetcher.calls) != 1 {
		t.Fatalf("second mirror fetched again: %d %v", len(f.fetcher.calls), err)
	}
}

func TestMirrorOne_NoBookOrNoCover(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, uuid.New(), ReasonSave); err != nil {
		t.Fatal(err)
	}
	id := f.book("")
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	if len(f.fetcher.calls) != 0 {
		t.Fatal("must not fetch")
	}
}

func TestMirrorOne_Absent(t *testing.T) {
	f := newFixture(t)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return &coverfetch.Result{Absent: true}, nil }
	id := f.book(ol)
	_ = f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSweep)
	b := f.load(id)
	if *b.CoverStatus != "absent" || b.DisplayCoverURL() != nil || len(f.storedFiles()) != 0 {
		t.Fatalf("absent: %+v", b)
	}
}

func TestMirrorOne_LinkChangedMidDownload_GuardedNoop(t *testing.T) {
	f := newFixture(t)
	id := f.book(ol)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) {
		// The admin replaces the link while the download is running.
		if err := f.db.Exec("UPDATE books SET cover_url = 'https://archive.org/new.jpg' WHERE id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		return jpeg("old"), nil
	}
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	b := f.load(id)
	if b.CoverStatus != nil || b.CoverPath != nil || b.CoverSourceURL != nil {
		t.Fatalf("state must be untouched: %+v", b)
	}
	if got := b.DisplayCoverURL(); got == nil || *got != "https://archive.org/new.jpg" {
		t.Fatalf("display: %v", got)
	}
	// The orphan file is removed by the sweep once it is old enough.
	if len(f.storedFiles()) != 1 {
		t.Fatalf("files: %v", f.storedFiles())
	}
}

func TestMirrorOne_TransientBackoffAndExhaustion(t *testing.T) {
	f := newFixture(t)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	id := f.book(ol)
	ctx := context.Background()

	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	b := f.load(id)
	if *b.CoverStatus != "pending" || b.CoverAttempts != 1 || *b.CoverError != "http_error" {
		t.Fatalf("first: %+v", b)
	}
	if want := f.now.Add(10 * time.Minute); b.CoverNextAttemptAt == nil || !b.CoverNextAttemptAt.Equal(want) {
		t.Fatalf("next=%v want %v", b.CoverNextAttemptAt, want)
	}
	if got := b.DisplayCoverURL(); got == nil || *got != ol {
		t.Fatalf("source must still be displayed: %v", got)
	}

	for i := 2; i <= 5; i++ {
		_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSweep)
		if b = f.load(id); b.CoverAttempts != i || *b.CoverStatus != "pending" {
			t.Fatalf("attempt %d: %+v", i, b)
		}
	}
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSweep)
	b = f.load(id)
	if *b.CoverStatus != "failed" || b.CoverAttempts != 0 {
		t.Fatalf("6th: %+v", b)
	}
	if want := f.now.Add(7 * 24 * time.Hour); !b.CoverNextAttemptAt.Equal(want) {
		t.Fatalf("next=%v want %v", b.CoverNextAttemptAt, want)
	}
}

func TestMirrorOne_PermanentNoNextAttempt(t *testing.T) {
	f := newFixture(t)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) {
		return nil, &coverfetch.Error{Code: coverfetch.CodeNotImage}
	}
	id := f.book(ol)
	_ = f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave)
	b := f.load(id)
	if *b.CoverStatus != "failed" || *b.CoverError != "not_image" || b.CoverNextAttemptAt != nil {
		t.Fatalf("permanent: %+v", b)
	}
	due, _ := repository.NewGormRepository(f.db).DueCandidates(context.Background(), f.now.Add(365*24*time.Hour), 10)
	if len(due) != 0 {
		t.Fatalf("permanent failure must not be due: %v", due)
	}
}

func TestMirrorOne_SaveResetsAttempts(t *testing.T) {
	f := newFixture(t)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	id := f.book(ol)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSweep)
	}
	if b := f.load(id); b.CoverAttempts != 3 {
		t.Fatalf("attempts=%d", b.CoverAttempts)
	}
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	if b := f.load(id); b.CoverAttempts != 1 {
		t.Fatalf("save must restart the count, attempts=%d", b.CoverAttempts)
	}
}

func TestMirrorOne_PacesDownloads(t *testing.T) {
	f := newFixture(t)
	a, b := f.book(ol), f.book("https://archive.org/b.jpg")
	_ = f.svc.MirrorOne(context.Background(), domain.OwnerBook, a, ReasonSave)
	_ = f.svc.MirrorOne(context.Background(), domain.OwnerBook, b, ReasonSave)
	if len(f.slept) != 1 || f.slept[0] < time.Second {
		t.Fatalf("slept=%v", f.slept)
	}
}

func TestMirrorOne_ReplacedCoverGCsOldFile_SharedFileKept(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, b := f.book("https://archive.org/a.jpg"), f.book("https://archive.org/b.jpg")
	// Both books have the same image.
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return jpeg("same"), nil }
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, a, ReasonSave)
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, b, ReasonSave)
	if len(f.storedFiles()) != 1 {
		t.Fatalf("identical images must share a file: %v", f.storedFiles())
	}
	shared := *f.load(a).CoverPath

	// Book a gets a new cover: the shared file stays for b.
	f.db.Exec("UPDATE books SET cover_url = 'https://archive.org/a2.jpg' WHERE id = ?", a)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return jpeg("different"), nil }
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, a, ReasonSave)
	if ok, _ := f.store.Exists(ctx, shared); !ok {
		t.Fatal("shared file deleted while b still uses it")
	}
	newPath := *f.load(a).CoverPath
	if newPath == shared {
		t.Fatal("expected a new file")
	}

	// Now b gets a new cover too: the old shared file is deleted.
	f.db.Exec("UPDATE books SET cover_url = 'https://archive.org/b2.jpg' WHERE id = ?", b)
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, b, ReasonSave)
	if ok, _ := f.store.Exists(ctx, shared); ok {
		t.Fatal("unreferenced old file must be deleted")
	}
	if ok, _ := f.store.Exists(ctx, newPath); !ok {
		t.Fatal("current file must stay")
	}
}

func TestMirrorOne_FailedReplacementDropsOldCopy(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.book("https://archive.org/a.jpg")
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	old := *f.load(id).CoverPath
	f.db.Exec("UPDATE books SET cover_url = 'https://evil.example/x.jpg' WHERE id = ?", id)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) {
		return nil, &coverfetch.Error{Code: coverfetch.CodeHostNotAllowed}
	}
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	b := f.load(id)
	if b.CoverPath != nil || *b.CoverStatus != "failed" || *b.CoverError != "host_not_allowed" {
		t.Fatalf("%+v", b)
	}
	if ok, _ := f.store.Exists(ctx, old); ok {
		t.Fatal("old copy must be deleted")
	}
}

func TestSweep_MirrorsBackfillAndDueRetries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	withCover := f.book(ol)
	noCover := f.book("")
	retryLater := f.book("https://archive.org/later.jpg")
	retryDue := f.book("https://archive.org/due.jpg")
	later, due := f.now.Add(time.Hour), f.now.Add(-time.Minute)
	f.db.Exec("UPDATE books SET cover_source_url = cover_url, cover_status = 'pending', cover_attempts = 2, cover_next_attempt_at = ? WHERE id = ?", later, retryLater)
	f.db.Exec("UPDATE books SET cover_source_url = cover_url, cover_status = 'pending', cover_attempts = 2, cover_next_attempt_at = ? WHERE id = ?", due, retryDue)

	stats, err := f.svc.Sweep(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Mirrored != 2 {
		t.Fatalf("mirrored=%d", stats.Mirrored)
	}
	if !f.load(withCover).HasStoredCover() || !f.load(retryDue).HasStoredCover() {
		t.Fatal("backfill and due retry must be stored")
	}
	if f.load(noCover).CoverStatus != nil || *f.load(retryLater).CoverStatus != "pending" {
		t.Fatal("no-cover and not-yet-due books must be untouched")
	}
}

func TestSweep_BatchLimit(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		f.book("https://archive.org/" + uuid.NewString() + ".jpg")
	}
	f.fetcher.fn = func(u string) (*coverfetch.Result, error) { return jpeg(u), nil }
	stats, _ := f.svc.Sweep(context.Background(), 3)
	if stats.Mirrored != 3 {
		t.Fatalf("mirrored=%d", stats.Mirrored)
	}
}

func TestSweep_MissingFileBecomesPendingThenRestored(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.book(ol)
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	_, _ = f.svc.Sweep(ctx, 50) // consumes the new-marker reset and re-mirrors
	path := *f.load(id).CoverPath
	if err := f.store.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}

	// Verification marks it pending and the same run copies it again.
	f.fetcher.calls = nil
	stats, err := f.svc.Sweep(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Missing != 1 || stats.Mirrored != 1 || len(f.fetcher.calls) != 1 {
		t.Fatalf("stats=%+v calls=%d", stats, len(f.fetcher.calls))
	}
	if ok, _ := f.store.Exists(ctx, *f.load(id).CoverPath); !ok {
		t.Fatal("file must be back")
	}
}

func TestSweep_MissingFileShownAsSourceInBetween(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.book(ol)
	_ = f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave)
	_, _ = f.svc.Sweep(ctx, 50)
	path := *f.load(id).CoverPath
	_ = f.store.Delete(ctx, path)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, errors.New("boom") }
	_, _ = f.svc.Sweep(ctx, 50)
	b := f.load(id)
	if d := b.DisplayCoverURL(); d == nil || *d != ol {
		t.Fatalf("display=%v", d)
	}
}

func TestSweep_NewMarkerResetsStoredRows(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	id := f.book(ol)
	// A row recorded as stored although the volume is new (marker missing).
	f.db.Exec(`UPDATE books SET cover_source_url = cover_url, cover_status = 'stored',
		cover_path = ? WHERE id = ?`, strings.Repeat("a", 2)+"/"+strings.Repeat("a", 64)+".jpg", id)
	stats, err := f.svc.Sweep(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Reset != 1 {
		t.Fatalf("reset=%d", stats.Reset)
	}
	b := f.load(id)
	if *b.CoverStatus != "pending" || b.CoverPath != nil {
		t.Fatalf("%+v", b)
	}
	// The marker now exists: a second sweep does not reset again.
	f.db.Exec(`UPDATE books SET cover_status = 'stored', cover_path = ? WHERE id = ?`,
		strings.Repeat("b", 2)+"/"+strings.Repeat("b", 64)+".jpg", id)
	_ = f.store.Put(ctx, strings.Repeat("b", 2)+"/"+strings.Repeat("b", 64)+".jpg", []byte("x"))
	stats, _ = f.svc.Sweep(ctx, 0)
	if stats.Reset != 0 {
		t.Fatalf("second sweep reset=%d", stats.Reset)
	}
}

func TestSweep_OrphanGC(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	ref := strings.Repeat("a", 2) + "/" + strings.Repeat("a", 64) + ".jpg"
	oldOrphan := strings.Repeat("b", 2) + "/" + strings.Repeat("b", 64) + ".jpg"
	youngOrphan := strings.Repeat("c", 2) + "/" + strings.Repeat("c", 64) + ".png"
	for _, n := range []string{ref, oldOrphan, youngOrphan} {
		_ = f.store.Put(ctx, n, []byte("x"))
	}
	f.store.SetModTime(ref, f.now.Add(-48*time.Hour))
	f.store.SetModTime(oldOrphan, f.now.Add(-2*time.Hour))
	f.store.SetModTime(youngOrphan, f.now.Add(-5*time.Minute))
	id := f.book(ol)
	f.db.Exec(`UPDATE books SET cover_source_url = cover_url, cover_status = 'stored', cover_path = ?,
		cover_checked_at = ? WHERE id = ?`, ref, f.now, id)
	_, _ = f.store.EnsureMarker(ctx) // existing volume: no reset

	stats, err := f.svc.Sweep(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Orphans != 1 {
		t.Fatalf("orphans=%d", stats.Orphans)
	}
	has := func(n string) bool { ok, _ := f.store.Exists(ctx, n); return ok }
	if !has(ref) || has(oldOrphan) || !has(youngOrphan) {
		t.Fatalf("ref=%v old=%v young=%v", has(ref), has(oldOrphan), has(youngOrphan))
	}
}
