package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/bookhaven/backend/internal/modules/covers/repository"
	"github.com/bookhaven/backend/internal/pkg/coverfetch"
	"github.com/bookhaven/backend/internal/pkg/tmdb"
	"github.com/google/uuid"
)

const (
	commonsLink = "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a1/Film.jpg/500px-Film.jpg"
	tmdbLink    = "https://image.tmdb.org/t/p/w500/abc.jpg"
	tmdbLink2   = "https://image.tmdb.org/t/p/w500/def.jpg"
)

type posterRow struct {
	Source     *string
	URL        *string
	CopyPath   *string
	CopyStatus *string
	CopySrcURL *string
	CopyAtt    int
	TMDBRef    *string
	VerifiedAt *time.Time
	RefreshAt  *time.Time
}

func (f *fixture) work(source, link string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var u any
	if link != "" {
		u = link
	}
	if err := f.db.Exec(`INSERT INTO screen_works (id, created_at, updated_at, poster_source, poster_url, poster_updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, f.now, f.now, source, u, f.now).Error; err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) tmdbWork(link, ref string, verified time.Time, refresh *time.Time) uuid.UUID {
	f.t.Helper()
	id := f.work("tmdb", link)
	if err := f.db.Exec(`UPDATE screen_works SET poster_tmdb_ref = ?, poster_verified_at = ?, poster_refresh_at = ? WHERE id = ?`,
		nilIfEmpty(ref), verified, refresh, id).Error; err != nil {
		f.t.Fatal(err)
	}
	return id
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (f *fixture) poster(id uuid.UUID) posterRow {
	f.t.Helper()
	var r posterRow
	err := f.db.Raw(`SELECT poster_source AS source, poster_url AS url, poster_copy_path AS copy_path,
		poster_copy_status AS copy_status, poster_copy_source_url AS copy_src_url, poster_copy_attempts AS copy_att,
		poster_tmdb_ref AS tmdb_ref, poster_verified_at AS verified_at, poster_refresh_at AS refresh_at
		FROM screen_works WHERE id = ?`, id).Scan(&r).Error
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) workCount(where string) int {
	var n int
	f.db.Raw("SELECT COUNT(*) FROM screen_works WHERE " + where).Scan(&n)
	return n
}

type fakeLookup struct {
	calls []string
	fn    func(kind, id string) (string, error)
}

func (l *fakeLookup) Poster(_ context.Context, kind, id string) (string, error) {
	l.calls = append(l.calls, kind+"/"+id)
	return l.fn(kind, id)
}

func TestMirrorOne_PosterStoredAndDisplaysCopy(t *testing.T) {
	f := newFixture(t)
	id := f.work("commons", commonsLink)
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerScreenWorkPoster, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	p := f.poster(id)
	if p.CopyStatus == nil || *p.CopyStatus != "stored" || p.CopyPath == nil || *p.CopySrcURL != commonsLink {
		t.Fatalf("%+v", p)
	}
	if got := f.storedFiles(); len(got) != 1 || got[0] != *p.CopyPath {
		t.Fatalf("files: %v", got)
	}
	// A stored copy of the same link is not fetched again.
	_ = f.svc.MirrorOne(context.Background(), domain.OwnerScreenWorkPoster, id, ReasonSave)
	if len(f.fetcher.calls) != 1 {
		t.Fatalf("fetched %d times", len(f.fetcher.calls))
	}
}

func TestMirrorOne_PosterIneligibleSourcesAreSkipped(t *testing.T) {
	f := newFixture(t)
	for _, src := range []string{"manual", "removed"} {
		id := f.work(src, "")
		if err := f.svc.MirrorOne(context.Background(), domain.OwnerScreenWorkPoster, id, ReasonSave); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.fetcher.calls) != 0 {
		t.Fatalf("fetched %v", f.fetcher.calls)
	}
}

func TestMirrorOne_PosterGuardedWhenRemovedOrReplacedDuringDownload(t *testing.T) {
	for name, mutate := range map[string]string{
		"removed":  `UPDATE screen_works SET poster_source = 'removed', poster_url = NULL WHERE id = ?`,
		"replaced": `UPDATE screen_works SET poster_source = 'manual', poster_url = NULL, poster_path = 'x/y.jpg' WHERE id = ?`,
		"relinked": `UPDATE screen_works SET poster_url = 'https://upload.wikimedia.org/other.jpg' WHERE id = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			id := f.work("commons", commonsLink)
			f.fetcher.fn = func(string) (*coverfetch.Result, error) {
				if err := f.db.Exec(mutate, id).Error; err != nil {
					t.Fatal(err)
				}
				return jpeg("late"), nil
			}
			if err := f.svc.MirrorOne(context.Background(), domain.OwnerScreenWorkPoster, id, ReasonSave); err != nil {
				t.Fatal(err)
			}
			if p := f.poster(id); p.CopyPath != nil || p.CopyStatus != nil {
				t.Fatalf("stale state written: %+v", p)
			}
		})
	}
}

func TestMirrorOne_PosterPermanentAndTransientFailures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	gone := f.work("commons", commonsLink)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) {
		return nil, &coverfetch.Error{Code: coverfetch.CodeHTTPError}
	}
	_ = f.svc.MirrorOne(ctx, domain.OwnerScreenWorkPoster, gone, ReasonSave)
	if p := f.poster(gone); *p.CopyStatus != "failed" {
		t.Fatalf("%+v", p)
	}
	flaky := f.work("tmdb", tmdbLink)
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	_ = f.svc.MirrorOne(ctx, domain.OwnerScreenWorkPoster, flaky, ReasonSave)
	if p := f.poster(flaky); *p.CopyStatus != "pending" || p.CopyAtt != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestSweep_PosterBackfillAndOrphanGCKeepsPosterOnlyFile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, _ = f.store.EnsureMarker(ctx)
	id := f.work("commons", commonsLink)
	stats, err := f.svc.Sweep(ctx, 50)
	if err != nil || stats.Mirrored != 1 {
		t.Fatalf("%+v %v", stats, err)
	}
	p := f.poster(id)
	if p.CopyPath == nil {
		t.Fatal("not stored")
	}
	// Two hours later the file is old enough for GC but a poster refers to it.
	f.now = f.now.Add(2 * time.Hour)
	f.store.SetModTime(*p.CopyPath, f.now.Add(-2*time.Hour))
	stats, err = f.svc.Sweep(ctx, 50)
	if err != nil || stats.Orphans != 0 {
		t.Fatalf("%+v %v", stats, err)
	}
	if ok, _ := f.store.Exists(ctx, *p.CopyPath); !ok {
		t.Fatal("poster copy was deleted by the orphan GC")
	}
	// After the poster is removed the file is an orphan.
	f.db.Exec(`UPDATE screen_works SET poster_source = 'removed', poster_url = NULL, poster_copy_path = NULL,
		poster_copy_status = NULL, poster_copy_source_url = NULL WHERE id = ?`, id)
	stats, _ = f.svc.Sweep(ctx, 50)
	if stats.Orphans != 1 {
		t.Fatalf("orphans=%d", stats.Orphans)
	}
}

func TestSweep_NewMarkerResetsPostersAndBooks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.fetcher.fn = func(string) (*coverfetch.Result, error) { return nil, transient() }
	name := strings.Repeat("a", 2) + "/" + strings.Repeat("a", 64) + ".jpg"
	book := f.book(ol)
	f.db.Exec(`UPDATE books SET cover_source_url = cover_url, cover_status = 'stored', cover_path = ? WHERE id = ?`, name, book)
	w := f.work("commons", commonsLink)
	f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored',
		poster_copy_path = ? WHERE id = ?`, name, w)
	stats, err := f.svc.Sweep(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Reset != 2 {
		t.Fatalf("reset=%d", stats.Reset)
	}
	if p := f.poster(w); *p.CopyStatus != "pending" && *p.CopyStatus != "failed" || p.CopyPath != nil {
		t.Fatalf("%+v", p)
	}
}

func TestDueCandidates_InterleaveOwnersByWaitTime(t *testing.T) {
	f := newFixture(t)
	b1 := f.book(ol)
	w1 := f.work("commons", commonsLink)
	b2 := f.book("https://archive.org/2.jpg")
	// Order by next_attempt NULLS FIRST, then created_at: set created_at so
	// that the order is b1, w1, b2.
	for i, q := range []struct {
		table string
		id    uuid.UUID
	}{{"books", b1}, {"screen_works", w1}, {"books", b2}} {
		f.db.Exec("UPDATE "+q.table+" SET created_at = ? WHERE id = ?", f.now.Add(time.Duration(i)*time.Minute), q.id)
	}
	got, err := repository.NewGormRepository(f.db).DueCandidates(context.Background(), f.now, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []repository.Ref{{Owner: domain.OwnerBook, ID: b1}, {Owner: domain.OwnerScreenWorkPoster, ID: w1}, {Owner: domain.OwnerBook, ID: b2}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got %v want %v", got, want)
	}
	if lim, _ := repository.NewGormRepository(f.db).DueCandidates(context.Background(), f.now, 2); len(lim) != 2 {
		t.Fatalf("single LIMIT: %d", len(lim))
	}
}

func TestMirrorPayload_LegacyBookIDDecodesAsBook(t *testing.T) {
	id := uuid.New()
	var p MirrorPayload
	if err := json.Unmarshal([]byte(`{"book_id":"`+id.String()+`","reason":"save"}`), &p); err != nil {
		t.Fatal(err)
	}
	owner, got, err := p.Resolve()
	if err != nil || owner != domain.OwnerBook || got != id {
		t.Fatalf("%v %v %v", owner, got, err)
	}
	task, _ := NewOwnerMirrorTask(domain.OwnerScreenWorkPoster, id, ReasonSave)
	var q MirrorPayload
	_ = json.Unmarshal(task.Payload(), &q)
	if o, i, err := q.Resolve(); err != nil || o != domain.OwnerScreenWorkPoster || i != id {
		t.Fatalf("%v %v %v", o, i, err)
	}
	if _, _, err := (MirrorPayload{Owner: "nope", ID: id.String()}).Resolve(); err == nil {
		t.Fatal("unknown owner must fail")
	}
}

// --- TMDB re-check ---

func tmdbFixture(t *testing.T, fn func(kind, id string) (string, error)) (*fixture, *fakeLookup) {
	f := newFixture(t)
	lk := &fakeLookup{fn: fn}
	f.svc = New(repository.NewGormRepository(f.db), f.store, f.fetcher, nil,
		WithClock(func() time.Time { return f.now }, func(_ context.Context, d time.Duration) error { f.now = f.now.Add(d); return nil }),
		WithTMDB(lk), WithRand(func() float64 { return 0.5 }))
	_, _ = f.store.EnsureMarker(context.Background())
	return f, lk
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestRefreshTMDB_SamePosterKeptAndRescheduled(t *testing.T) {
	f, lk := tmdbFixture(t, func(string, string) (string, error) { return tmdbLink, nil })
	id := f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-155*24*time.Hour), ptrTime(f.now.Add(-time.Hour)))
	f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored',
		poster_copy_path = 'aa/x.jpg', poster_copy_checked_at = ? WHERE id = ?`, f.now, id)
	_ = f.store.Put(context.Background(), "aa/x.jpg", []byte("x"))
	if _, err := f.svc.Sweep(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	if len(lk.calls) != 1 || lk.calls[0] != "movie/5" {
		t.Fatalf("calls %v", lk.calls)
	}
	p := f.poster(id)
	if p.Source == nil || *p.URL != tmdbLink || p.CopyPath == nil {
		t.Fatalf("%+v", p)
	}
	if want := f.now.Add(157 * 24 * time.Hour); !p.RefreshAt.Equal(want) {
		t.Fatalf("refresh %v want %v", p.RefreshAt, want)
	}
	if !p.VerifiedAt.Equal(f.now) {
		t.Fatalf("verified %v", p.VerifiedAt)
	}
}

func TestRefreshTMDB_ChangedPosterReplacedAndCopied(t *testing.T) {
	f, _ := tmdbFixture(t, func(string, string) (string, error) { return tmdbLink2, nil })
	id := f.tmdbWork(tmdbLink, "tv/9", f.now.Add(-155*24*time.Hour), ptrTime(f.now.Add(-time.Hour)))
	f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored',
		poster_copy_path = 'aa/x.jpg' WHERE id = ?`, id)
	if _, err := f.svc.Sweep(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	p := f.poster(id)
	if *p.URL != tmdbLink2 || p.CopyStatus == nil || *p.CopyStatus != "stored" || *p.CopySrcURL != tmdbLink2 {
		t.Fatalf("%+v", p)
	}
	if len(f.fetcher.calls) == 0 || f.fetcher.calls[len(f.fetcher.calls)-1] != tmdbLink2 {
		t.Fatalf("fetched %v", f.fetcher.calls)
	}
}

func TestRefreshTMDB_GoneNoKeyNoRefDropWithoutRemoved(t *testing.T) {
	ctx := context.Background()
	due := func(f *fixture) *time.Time { return ptrTime(f.now.Add(-time.Hour)) }

	t.Run("gone", func(t *testing.T) {
		f, _ := tmdbFixture(t, func(string, string) (string, error) { return "", nil })
		id := f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-155*24*time.Hour), due(f))
		stats, err := f.svc.Sweep(ctx, 50)
		if err != nil || stats.TMDBDropped != 1 {
			t.Fatalf("%+v %v", stats, err)
		}
		assertDropped(t, f, id)
	})
	t.Run("no key", func(t *testing.T) {
		f, _ := tmdbFixture(t, nil)
		f.svc = New(repository.NewGormRepository(f.db), f.store, f.fetcher, nil, WithClock(func() time.Time { return f.now }, nil))
		id := f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-155*24*time.Hour), due(f))
		if _, err := f.svc.Sweep(ctx, 50); err != nil {
			t.Fatal(err)
		}
		assertDropped(t, f, id)
	})
	t.Run("no ref (legacy row, refresh_at NULL)", func(t *testing.T) {
		f, lk := tmdbFixture(t, func(string, string) (string, error) { return tmdbLink, nil })
		id := f.tmdbWork(tmdbLink, "", f.now.Add(-time.Hour), nil)
		if _, err := f.svc.Sweep(ctx, 50); err != nil {
			t.Fatal(err)
		}
		if len(lk.calls) != 0 {
			t.Fatalf("looked up %v", lk.calls)
		}
		assertDropped(t, f, id)
	})
}

func assertDropped(t *testing.T, f *fixture, id uuid.UUID) {
	t.Helper()
	p := f.poster(id)
	if p.Source != nil || p.URL != nil || p.CopyPath != nil || p.CopyStatus != nil || p.TMDBRef != nil || p.RefreshAt != nil || p.VerifiedAt != nil {
		t.Fatalf("not dropped: %+v", p)
	}
}

func TestRefreshTMDB_UnavailableKeepsPosterAndRetriesTomorrow(t *testing.T) {
	for _, e := range []error{tmdb.ErrUnavailable, tmdb.ErrUnauthorized} {
		f, _ := tmdbFixture(t, func(string, string) (string, error) { return "", e })
		id := f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-155*24*time.Hour), ptrTime(f.now.Add(-time.Hour)))
		if _, err := f.svc.Sweep(context.Background(), 50); err != nil {
			t.Fatal(err)
		}
		p := f.poster(id)
		if p.Source == nil || *p.URL != tmdbLink {
			t.Fatalf("%v: poster lost %+v", e, p)
		}
		if want := f.now.Add(24 * time.Hour); !p.RefreshAt.Equal(want) {
			t.Fatalf("%v: refresh %v want %v", e, p.RefreshAt, want)
		}
	}
}

func TestRefreshTMDB_HardLimitDropsAt175DaysWhateverTMDBSays(t *testing.T) {
	f, lk := tmdbFixture(t, func(string, string) (string, error) { return "", tmdb.ErrUnavailable })
	old := f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-176*24*time.Hour), ptrTime(f.now.Add(24*time.Hour)))
	young := f.tmdbWork(tmdbLink2, "movie/6", f.now.Add(-174*24*time.Hour), ptrTime(f.now.Add(24*time.Hour)))
	commons := f.work("commons", commonsLink)
	f.db.Exec("UPDATE screen_works SET poster_updated_at = ? WHERE id = ?", f.now.Add(-300*24*time.Hour), commons)
	stats, err := f.svc.Sweep(context.Background(), 50)
	if err != nil || stats.TMDBDropped != 1 {
		t.Fatalf("%+v %v", stats, err)
	}
	assertDropped(t, f, old)
	if p := f.poster(young); p.Source == nil {
		t.Fatal("174-day poster must stay")
	}
	if p := f.poster(commons); p.Source == nil || *p.Source != "commons" {
		t.Fatal("commons poster must never be dropped by the re-check")
	}
	if len(lk.calls) != 0 {
		t.Fatalf("not due, must not be looked up: %v", lk.calls)
	}
	if f.workCount("poster_source = 'removed'") != 0 {
		t.Fatal("drop must not set removed")
	}
}

func TestRefreshTMDB_BatchOf20(t *testing.T) {
	f, lk := tmdbFixture(t, func(string, string) (string, error) { return tmdbLink, nil })
	for i := 0; i < 25; i++ {
		f.tmdbWork(tmdbLink, "movie/5", f.now.Add(-time.Hour), ptrTime(f.now.Add(-time.Hour)))
	}
	if _, err := f.svc.Sweep(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	if len(lk.calls) != TMDBRefreshBatch {
		t.Fatalf("looked up %d", len(lk.calls))
	}
}

// --- purge ---

func TestPurgeTMDB(t *testing.T) {
	ctx := context.Background()
	setup := func() (*fixture, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, string, string) {
		f, _ := tmdbFixture(t, func(string, string) (string, error) { return "", nil })
		tmdbOnly := strings.Repeat("a", 2) + "/" + strings.Repeat("a", 64) + ".jpg"
		shared := strings.Repeat("b", 2) + "/" + strings.Repeat("b", 64) + ".jpg"
		for _, n := range []string{tmdbOnly, shared} {
			_ = f.store.Put(ctx, n, []byte("x"))
		}
		t1 := f.tmdbWork(tmdbLink, "movie/1", f.now, ptrTime(f.now))
		t2 := f.tmdbWork(tmdbLink2, "tv/2", f.now, ptrTime(f.now))
		cm := f.work("commons", commonsLink)
		man := f.work("manual", "")
		f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored', poster_copy_path = ? WHERE id = ?`, tmdbOnly, t1)
		f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored', poster_copy_path = ? WHERE id = ?`, shared, t2)
		f.db.Exec(`UPDATE screen_works SET poster_copy_source_url = poster_url, poster_copy_status = 'stored', poster_copy_path = ? WHERE id = ?`, shared, cm)
		return f, t1, t2, cm, man, tmdbOnly, shared
	}

	t.Run("dry run changes nothing", func(t *testing.T) {
		f, _, _, _, _, tmdbOnly, _ := setup()
		st, err := f.svc.PurgeTMDB(ctx, true)
		if err != nil || st.PostersRemoved != 2 || st.FilesDeleted != 1 {
			t.Fatalf("%+v %v", st, err)
		}
		if f.workCount("poster_source = 'tmdb'") != 2 {
			t.Fatal("dry run changed rows")
		}
		if ok, _ := f.store.Exists(ctx, tmdbOnly); !ok {
			t.Fatal("dry run deleted a file")
		}
	})

	t.Run("purge", func(t *testing.T) {
		f, t1, t2, cm, man, tmdbOnly, shared := setup()
		st, err := f.svc.PurgeTMDB(ctx, false)
		if err != nil || st.PostersRemoved != 2 || st.FilesDeleted != 1 {
			t.Fatalf("%+v %v", st, err)
		}
		assertDropped(t, f, t1)
		assertDropped(t, f, t2)
		if p := f.poster(cm); p.Source == nil || *p.Source != "commons" || p.CopyPath == nil {
			t.Fatalf("commons changed: %+v", p)
		}
		if p := f.poster(man); *p.Source != "manual" {
			t.Fatalf("manual changed: %+v", p)
		}
		if ok, _ := f.store.Exists(ctx, tmdbOnly); ok {
			t.Fatal("unreferenced file must be deleted")
		}
		if ok, _ := f.store.Exists(ctx, shared); !ok {
			t.Fatal("file still used by a commons poster must stay")
		}
	})
}
