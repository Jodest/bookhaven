package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/modules/curated/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func autoIn(metric string) CreateInput {
	return CreateInput{Title: "Топ читаемых", Size: 10, Kind: domain.KindAuto, Rule: &domain.Rule{Metric: metric}}
}

func readShelf(t *testing.T, db *gorm.DB, bookID uuid.UUID, typ string) {
	t.Helper()
	shelf := uuid.New()
	if err := db.Exec("INSERT INTO shelves (id, user_id, name, type) VALUES (?, ?, ?, ?)", shelf, uuid.New(), typ, typ).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, bookID, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCreateAuto_DefaultsAndFirstSnapshot(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	a := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, a, "read")
	c, err := s.Create(ctx, autoIn(domain.MetricWeightedRating))
	if err != nil {
		t.Fatal(err)
	}
	r := domain.RuleOf(c)
	if c.Kind != domain.KindAuto || r == nil || *r.MinRatings != 3 || *r.Confidence != 10 || r.PeriodDays != nil {
		t.Fatalf("rule: %+v %+v", c, r)
	}
	if c.ComputedAt == nil || c.BaselineAt != nil {
		t.Fatalf("times: %v %v", c.ComputedAt, c.BaselineAt)
	}
	// most_read picks the book up on creation.
	in2 := autoIn(domain.MetricMostRead)
	in2.Title = "Топ прочитанных"
	c2, err := s.Create(ctx, in2)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.GetAdmin(ctx, c2.ID)
	if err != nil || len(d.Items) != 1 || d.Items[0].Book.ID != a || d.Items[0].Score == nil || d.Items[0].Movement != nil {
		t.Fatalf("detail: %+v %v", d, err)
	}
}

func TestCreateAuto_Validation(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	cases := map[string]CreateInput{
		"rule: required for automatic collections": {Title: "Топ", Size: 5, Kind: domain.KindAuto},
		"rule: only for automatic collections":     {Title: "Топ", Size: 5, Rule: &domain.Rule{Metric: domain.MetricMostRead}},
		"kind: must be manual or auto":             {Title: "Топ", Size: 5, Kind: "x"},
		"rule.metric: unknown metric":              {Title: "Топ", Size: 5, Kind: domain.KindAuto, Rule: &domain.Rule{Metric: "x"}},
	}
	for want, in := range cases {
		_, err := s.Create(ctx, in)
		var v *ValidationError
		if !errors.As(err, &v) || v.Msg != want {
			t.Errorf("%s: got %v", want, err)
		}
	}
	list, _ := s.ListAdmin(ctx)
	if len(list) != 0 {
		t.Fatal("nothing must be created")
	}
}

// failingRecompute makes Recompute fail for the listed ids.
type failingRecompute struct {
	repository.Repository
	fail map[uuid.UUID]bool
}

func (f failingRecompute) Recompute(ctx context.Context, id uuid.UUID, reset bool, now time.Time, rank repository.RankFunc) error {
	if f.fail[id] {
		return errors.New("boom")
	}
	return f.Repository.Recompute(ctx, id, reset, now, rank)
}

func TestCreateAuto_ComputeFailureStillCreates(t *testing.T) {
	s, _ := setup(t)
	svc := NewService(failingAll{s.repo})
	c, err := svc.Create(context.Background(), autoIn(domain.MetricMostRead))
	if err != nil || c.ComputedAt != nil {
		t.Fatalf("create: %+v %v", c, err)
	}
}

type failingAll struct{ repository.Repository }

func (failingAll) Recompute(context.Context, uuid.UUID, bool, time.Time, repository.RankFunc) error {
	return errors.New("boom")
}

func TestUpdateAuto_RecomputeOnRuleGenreSizeChange(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	a := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, a, "read")
	c, err := s.Create(ctx, autoIn(domain.MetricMostRead))
	if err != nil {
		t.Fatal(err)
	}
	// A normal recompute sets a baseline.
	if _, err := s.Recompute(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.repo.GetByID(ctx, c.ID)
	if got.BaselineAt == nil {
		t.Fatal("baseline expected after a normal recompute")
	}
	computed := *got.ComputedAt

	// A title edit does not recompute.
	title := "Новое название"
	if _, err := s.Update(ctx, c.ID, UpdateInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.repo.GetByID(ctx, c.ID)
	if !got.ComputedAt.Equal(computed) || got.BaselineAt == nil {
		t.Fatalf("title edit must keep the snapshot: %+v", got)
	}

	// Same rule re-sent is not a change.
	same := domain.Rule{Metric: domain.MetricMostRead}
	if _, err := s.Update(ctx, c.ID, UpdateInput{Rule: &same}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.repo.GetByID(ctx, c.ID)
	if got.BaselineAt == nil {
		t.Fatal("unchanged rule must not reset the baseline")
	}

	// Rule, size and genre changes recompute with a reset baseline.
	changes := []UpdateInput{
		{Rule: &domain.Rule{Metric: domain.MetricMostWanted}},
		{Size: func() *int { n := 5; return &n }()},
		{GenreIDSet: true, GenreID: func() *uuid.UUID { g := genre(t, db, catalogDomain.GenreStatusPublished, nil); return &g }()},
	}
	for i, in := range changes {
		if _, err := s.Recompute(ctx, c.ID); err != nil { // give it a baseline first
			t.Fatal(err)
		}
		if _, err := s.Update(ctx, c.ID, in); err != nil {
			t.Fatalf("change %d: %v", i, err)
		}
		got, _ = s.repo.GetByID(ctx, c.ID)
		if got.BaselineAt != nil || got.ComputedAt == nil {
			t.Fatalf("change %d must reset the baseline: %+v", i, got)
		}
	}
}

func TestUpdate_KindAndRuleOnManual(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	m, err := s.Create(ctx, CreateInput{Title: "Ручная", Size: 5})
	if err != nil {
		t.Fatal(err)
	}
	kind := domain.KindAuto
	_, err = s.Update(ctx, m.ID, UpdateInput{Kind: &kind})
	var v *ValidationError
	if !errors.As(err, &v) || v.Msg != "kind: cannot be changed" {
		t.Fatalf("kind: %v", err)
	}
	_, err = s.Update(ctx, m.ID, UpdateInput{Rule: &domain.Rule{Metric: domain.MetricMostRead}})
	if !errors.As(err, &v) || v.Msg != "rule: only for automatic collections" {
		t.Fatalf("rule: %v", err)
	}
	got, _ := s.repo.GetByID(ctx, m.ID)
	if got.Kind != domain.KindManual {
		t.Fatal("kind changed")
	}
}

func TestRecompute_Movement(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	a := book(t, db, catalogDomain.BookStatusPublished)
	b := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, a, "read")
	readShelf(t, db, a, "read")
	readShelf(t, db, b, "read")
	c, _ := s.Create(ctx, autoIn(domain.MetricMostRead))
	pub := true
	if _, err := s.Update(ctx, c.ID, UpdateInput{IsPublished: &pub}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.repo.GetByID(ctx, c.ID)

	// No baseline yet: movement is null.
	pg, err := s.GetPublic(ctx, got.Slug, 1, 10)
	if err != nil || len(pg.Items) != 2 || pg.Items[0].Movement != nil {
		t.Fatalf("page: %+v %v", pg, err)
	}

	// b overtakes a; a new book enters.
	n := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, n, "read")
	for i := 0; i < 3; i++ {
		readShelf(t, db, b, "read")
	}
	d, err := s.Recompute(ctx, c.ID)
	if err != nil || d.Collection.BaselineAt == nil {
		t.Fatalf("recompute: %+v %v", d, err)
	}
	pg, _ = s.GetPublic(ctx, got.Slug, 1, 10)
	if len(pg.Items) != 3 || pg.Items[0].Book.ID != b {
		t.Fatalf("order: %+v", pg.Items)
	}
	if m := pg.Items[0].Movement; m == nil || m.Direction != domain.MoveUp || m.By != 1 {
		t.Fatalf("b movement: %+v", m)
	}
	if m := pg.Items[1].Movement; m == nil || m.Direction != domain.MoveDown || m.By != 1 {
		t.Fatalf("a movement: %+v", m)
	}
	if m := pg.Items[2].Movement; m == nil || m.Direction != domain.MoveNew {
		t.Fatalf("new movement: %+v", m)
	}
}

func TestPublicManual_NoMovement(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	m, _ := s.Create(ctx, CreateInput{Title: "Ручная", Size: 5, IsPublished: true})
	if err := s.AddBook(ctx, m.ID, book(t, db, catalogDomain.BookStatusPublished), nil); err != nil {
		t.Fatal(err)
	}
	pg, err := s.GetPublic(ctx, m.Slug, 1, 10)
	if err != nil || len(pg.Items) != 1 || pg.Items[0].Movement != nil {
		t.Fatalf("manual page: %+v %v", pg, err)
	}
}

func TestRecompute_ManualAndMissing(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	m, _ := s.Create(ctx, CreateInput{Title: "Ручная", Size: 5})
	if _, err := s.Recompute(ctx, m.ID); !errors.Is(err, domain.ErrNotAutomatic) {
		t.Fatalf("manual: %v", err)
	}
	if _, err := s.Recompute(ctx, uuid.New()); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestAutoCollection_BookEditingRejected(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	c, _ := s.Create(ctx, autoIn(domain.MetricMostRead))
	b := book(t, db, catalogDomain.BookStatusPublished)
	if err := s.AddBook(ctx, c.ID, b, nil); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("AddBook: %v", err)
	}
	if err := s.RemoveBook(ctx, c.ID, b); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("RemoveBook: %v", err)
	}
	if err := s.MoveBook(ctx, c.ID, b, 1); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("MoveBook: %v", err)
	}
	if _, err := s.PreviewPaste(ctx, c.ID, "Книга"); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("PreviewPaste: %v", err)
	}
	if _, err := s.BulkAdd(ctx, c.ID, []uuid.UUID{b}, nil); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("BulkAdd: %v", err)
	}
}

func TestRecomputeAll_ContinuesAfterFailure(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	a := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, a, "read")
	bad, _ := s.Create(ctx, autoIn(domain.MetricMostRead))
	in := autoIn(domain.MetricMostRead)
	in.Title = "Другая"
	good, _ := s.Create(ctx, in)
	manual, _ := s.Create(ctx, CreateInput{Title: "Ручная", Size: 5})

	// A new book appears; only the healthy collection may pick it up.
	n := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, n, "read")
	readShelf(t, db, n, "read")
	svc := NewService(failingRecompute{Repository: s.repo, fail: map[uuid.UUID]bool{bad.ID: true}})
	if err := svc.RecomputeAll(ctx); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[uuid.UUID]int{bad.ID: 1, good.ID: 2} {
		d, _ := s.GetAdmin(ctx, id)
		if len(d.Items) != want {
			t.Errorf("collection %s: %d items, want %d", id, len(d.Items), want)
		}
	}
	if d, _ := s.GetAdmin(ctx, manual.ID); len(d.Items) != 0 {
		t.Fatal("manual collection must be untouched")
	}
}

func TestRecomputeCollections(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	a := book(t, db, catalogDomain.BookStatusPublished)
	readShelf(t, db, a, "read")
	c, err := s.Create(ctx, autoIn(domain.MetricMostRead))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recompute(ctx, c.ID); err != nil { // gives it a baseline
		t.Fatal(err)
	}
	manual, err := s.Create(ctx, CreateInput{Title: "Ручная", Size: 5})
	if err != nil {
		t.Fatal(err)
	}

	// Auto collections get a fresh snapshot without a baseline; a manual id
	// is only logged, nothing fails.
	s.RecomputeCollections(ctx, []uuid.UUID{c.ID, manual.ID, uuid.New()})

	got, _ := s.repo.GetByID(ctx, c.ID)
	if got.BaselineAt != nil || got.ComputedAt == nil {
		t.Fatalf("auto collection must be recomputed without baseline: %+v", got)
	}
	gotManual, _ := s.repo.GetByID(ctx, manual.ID)
	if gotManual.ComputedAt != nil {
		t.Fatalf("manual collection must stay untouched: %+v", gotManual)
	}
}
