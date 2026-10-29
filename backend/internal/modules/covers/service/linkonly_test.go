package service

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/covers/domain"
	"github.com/bookhaven/backend/internal/modules/covers/repository"
)

const fantLabLink = "https://fantlab.ru/images/editions/big/77"

func (f *fixture) withLinkOnly(hosts ...string) {
	f.svc = New(repository.NewGormRepository(f.db), f.store, f.fetcher, nil,
		WithLinkOnlyHosts(hosts),
		WithClock(func() time.Time { return f.now }, func(_ context.Context, d time.Duration) error {
			f.slept = append(f.slept, d)
			f.now = f.now.Add(d)
			return nil
		}))
}

func TestMirrorOne_LinkOnlyHostIsLinkedWithoutFetch(t *testing.T) {
	f := newFixture(t)
	f.withLinkOnly("fantlab.ru", "*.fantlab.ru")
	id := f.book(fantLabLink)
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	if len(f.fetcher.calls) != 0 || len(f.slept) != 0 {
		t.Fatalf("fetched %v, slept %v", f.fetcher.calls, f.slept)
	}
	b := f.load(id)
	if b.CoverStatus == nil || *b.CoverStatus != "linked" || b.CoverError != nil || b.CoverAttempts != 0 ||
		b.CoverPath != nil || b.CoverStoredAt != nil {
		t.Fatalf("state: %+v", b)
	}
	if b.CoverNextAttemptAt == nil || !b.CoverNextAttemptAt.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("next attempt = %v, want now+7d", b.CoverNextAttemptAt)
	}
	if d := b.DisplayCoverURL(); d == nil || *d != fantLabLink {
		t.Fatalf("display = %v, want the source link", d)
	}
}

func TestMirrorOne_LinkOnlyCollectsOldStoredFile(t *testing.T) {
	f := newFixture(t)
	id := f.book(ol)
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	if len(f.storedFiles()) != 1 {
		t.Fatalf("files: %v", f.storedFiles())
	}
	f.withLinkOnly("fantlab.ru")
	if err := f.db.Exec("UPDATE books SET cover_url = ? WHERE id = ?", fantLabLink, id).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MirrorOne(context.Background(), domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	b := f.load(id)
	if *b.CoverStatus != "linked" || b.HasStoredCover() || len(f.storedFiles()) != 0 {
		t.Fatalf("state %+v files %v", b, f.storedFiles())
	}
	if d := b.DisplayCoverURL(); d == nil || *d != fantLabLink {
		t.Fatalf("display = %v", d)
	}
}

func TestSweep_LinkedRecheckedWeeklyWithoutFetch(t *testing.T) {
	f := newFixture(t)
	f.withLinkOnly("fantlab.ru")
	ctx := context.Background()
	id := f.book(fantLabLink)
	if err := f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}

	f.now = f.now.Add(24 * time.Hour) // not due yet
	if stats, err := f.svc.Sweep(ctx, 50); err != nil || stats.Mirrored != 0 {
		t.Fatalf("early sweep: %+v %v", stats, err)
	}

	f.now = f.now.Add(7 * 24 * time.Hour) // 8 days after the first record
	if _, err := f.svc.Sweep(ctx, 50); err != nil {
		t.Fatal(err)
	}
	b := f.load(id)
	if len(f.fetcher.calls) != 0 || *b.CoverStatus != "linked" {
		t.Fatalf("calls %v state %+v", f.fetcher.calls, b)
	}
	if b.CoverNextAttemptAt == nil || !b.CoverNextAttemptAt.Equal(f.now.Add(7*24*time.Hour)) {
		t.Fatalf("next attempt = %v", b.CoverNextAttemptAt)
	}
}

func TestSweep_HostTakenOffLinkOnlyListIsFetched(t *testing.T) {
	f := newFixture(t)
	f.withLinkOnly("fantlab.ru")
	ctx := context.Background()
	id := f.book(fantLabLink)
	if err := f.svc.MirrorOne(ctx, domain.OwnerBook, id, ReasonSave); err != nil {
		t.Fatal(err)
	}
	f.withLinkOnly() // the operator moved the host to the download allowlist
	f.now = f.now.Add(8 * 24 * time.Hour)
	if _, err := f.svc.Sweep(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if len(f.fetcher.calls) != 1 || !f.load(id).HasStoredCover() {
		t.Fatalf("calls %v state %+v", f.fetcher.calls, f.load(id))
	}
}

func TestMirrorOne_LinkOnlyDoesNotAffectPostersOrOtherHosts(t *testing.T) {
	f := newFixture(t)
	f.withLinkOnly("fantlab.ru")
	ctx := context.Background()

	other := f.book(ol)
	if err := f.svc.MirrorOne(ctx, domain.OwnerBook, other, ReasonSave); err != nil || !f.load(other).HasStoredCover() {
		t.Fatalf("other host not stored: %v", err)
	}

	work := f.work("commons", fantLabLink)
	if err := f.svc.MirrorOne(ctx, domain.OwnerScreenWorkPoster, work, ReasonSave); err != nil {
		t.Fatal(err)
	}
	if p := f.poster(work); p.CopyStatus == nil || *p.CopyStatus != "stored" {
		t.Fatalf("poster: %+v", p)
	}
}
