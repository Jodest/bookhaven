package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/repository"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/coverstore"
	"github.com/bookhaven/backend/internal/pkg/tmdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeCommonsSource struct {
	*fakeSource
	images map[string]*bookdata.CommonsImage
	err    error
	calls  int
}

func (f *fakeCommonsSource) FetchCommonsImages(_ context.Context, names []string) (map[string]*bookdata.CommonsImage, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]*bookdata.CommonsImage{}
	for _, n := range names {
		if img, ok := f.images[n]; ok {
			out[n] = img
		}
	}
	return out, nil
}

type fakeTMDB struct {
	posters map[string]string // "kind:id" -> url
	err     error
	calls   []string
}

func (f *fakeTMDB) Poster(_ context.Context, kind, id string) (string, error) {
	f.calls = append(f.calls, kind+":"+id)
	if f.err != nil {
		return "", f.err
	}
	return f.posters[kind+":"+id], nil
}

func freeImage(url string) *bookdata.CommonsImage {
	return &bookdata.CommonsImage{ThumbURL: url, PageURL: "https://commons.wikimedia.org/wiki/File:X", Free: true, License: "CC0"}
}

func posterRow(t *testing.T, db *gorm.DB, qid string) (source, url string) {
	t.Helper()
	var row struct {
		Source *string
		URL    *string
	}
	if err := db.Raw("SELECT poster_source AS source, poster_url AS url FROM screen_works WHERE external_id = ?", qid).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Source != nil {
		source = *row.Source
	}
	if row.URL != nil {
		url = *row.URL
	}
	return
}

func withPosters(t *testing.T, src Source, db *gorm.DB, opts ...Option) *Service {
	t.Helper()
	return NewService(repository.NewGormRepository(db), src, nil, opts...)
}

func TestSyncPosters_CommonsPosterWinsOverImage(t *testing.T) {
	w := work("Q10", "Фильм", year(1994))
	w.PosterFile, w.ImageFile = "Poster.jpg", "Still.jpg"
	src := &fakeCommonsSource{
		fakeSource: &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w}}},
		images:     map[string]*bookdata.CommonsImage{"Poster.jpg": freeImage("https://upload.wikimedia.org/p.jpg"), "Still.jpg": freeImage("https://upload.wikimedia.org/s.jpg")},
	}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db)
	book := mkBook(t, db, "published", "Q1")

	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Posters != 1 || rep.Items[0].Posters != 1 || rep.Items[0].Outcome != domain.OutcomeAdded {
		t.Fatalf("report = %+v", rep)
	}
	if s, u := posterRow(t, db, "Q10"); s != "commons" || u != "https://upload.wikimedia.org/p.jpg" {
		t.Errorf("poster = %q %q", s, u)
	}
}

func TestSyncPosters_NonFreeFallsBackToTMDB_SeriesTVFirst(t *testing.T) {
	w := work("Q10", "Сериал", year(2000))
	w.Kind = bookdata.ScreenKindSeries
	w.PosterFile, w.TMDBMovieID, w.TMDBTVID = "Poster.jpg", "5", "7"
	src := &fakeCommonsSource{
		fakeSource: &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w}}},
		images: map[string]*bookdata.CommonsImage{"Poster.jpg": {
			ThumbURL: "https://upload.wikimedia.org/p.jpg", Free: false,
		}},
	}
	tm := &fakeTMDB{posters: map[string]string{"tv:7": "https://image.tmdb.org/t/p/w342/a.jpg", "movie:5": "https://image.tmdb.org/t/p/w342/b.jpg"}}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db, WithTMDB(tm))
	book := mkBook(t, db, "published", "Q1")

	if _, err := svc.SyncBooks(context.Background(), []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	if s, u := posterRow(t, db, "Q10"); s != "tmdb" || u != "https://image.tmdb.org/t/p/w342/a.jpg" {
		t.Errorf("poster = %q %q", s, u)
	}
	if len(tm.calls) != 1 || tm.calls[0] != "tv:7" {
		t.Errorf("tmdb calls = %v", tm.calls)
	}
}

func TestSyncPosters_AttributionRequiredNeedsAuthor(t *testing.T) {
	w := work("Q10", "Фильм", year(1994))
	w.PosterFile = "Poster.jpg"
	img := freeImage("https://upload.wikimedia.org/p.jpg")
	img.AttributionRequired = true
	src := &fakeCommonsSource{
		fakeSource: &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w}}},
		images:     map[string]*bookdata.CommonsImage{"Poster.jpg": img},
	}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db)
	book := mkBook(t, db, "published", "Q1")
	if _, err := svc.SyncBooks(context.Background(), []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	if s, _ := posterRow(t, db, "Q10"); s != "" {
		t.Errorf("poster source = %q, want none", s)
	}
}

func TestSyncPosters_NoTMDBClientNoPoster(t *testing.T) {
	w := work("Q10", "Фильм", year(1994))
	w.TMDBMovieID = "5"
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w}}}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db) // plain source: no Commons, no TMDB
	book := mkBook(t, db, "published", "Q1")
	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{book})
	if err != nil || rep.Posters != 0 {
		t.Fatalf("report = %+v, %v", rep, err)
	}
}

func TestSyncPosters_ErrorsLeaveOutcomeUnchanged(t *testing.T) {
	w := work("Q10", "Фильм", year(1994))
	w.PosterFile, w.TMDBMovieID = "Poster.jpg", "5"
	src := &fakeCommonsSource{
		fakeSource: &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w}}},
		err:        errors.New("commons down"),
	}
	tm := &fakeTMDB{err: tmdb.ErrUnavailable}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db, WithTMDB(tm))
	book := mkBook(t, db, "published", "Q1")
	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeAdded || rep.Added != 1 || rep.Posters != 0 {
		t.Fatalf("report = %+v", rep)
	}
}

func TestSyncPosters_UnauthorizedStopsTMDBForChunk(t *testing.T) {
	w1, w2 := work("Q10", "Один", year(1994)), work("Q11", "Два", year(1995))
	w1.TMDBMovieID, w2.TMDBMovieID = "5", "6"
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {w1, w2}}}
	tm := &fakeTMDB{err: tmdb.ErrUnauthorized}
	_, db := setup(t, nil)
	svc := withPosters(t, src, db, WithTMDB(tm))
	book := mkBook(t, db, "published", "Q1")
	if _, err := svc.SyncBooks(context.Background(), []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	if len(tm.calls) != 1 {
		t.Errorf("tmdb calls after 401 = %v", tm.calls)
	}
}

func mustExec(t *testing.T, db *gorm.DB, q string, args ...any) {
	t.Helper()
	if err := db.Exec(q, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func TestSyncPosters_BackfillRespectsManualRemovedAndEdited(t *testing.T) {
	plain := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {
		work("Q10", "Пустой", year(1990)), work("Q11", "Убранный", year(1991)), work("Q12", "Правленый", year(1992)), work("Q13", "Загруженный", year(1993)),
	}}}
	svc0, db := setup(t, plain)
	book := mkBook(t, db, "published", "Q1")
	if _, err := svc0.SyncBooks(context.Background(), []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, "UPDATE screen_works SET poster_source = 'removed' WHERE external_id = 'Q11'")
	mustExec(t, db, "UPDATE screen_works SET manually_edited_at = CURRENT_TIMESTAMP WHERE external_id = 'Q12'")
	mustExec(t, db, "UPDATE screen_works SET poster_source = 'manual', poster_path = 'ab/x.jpg' WHERE external_id = 'Q13'")

	withImg := map[string]*bookdata.CommonsImage{}
	var works []bookdata.ExternalScreenWork
	for _, q := range []string{"Q10", "Q11", "Q12", "Q13"} {
		w := work(q, q, year(1990))
		w.PosterFile = q + ".jpg"
		works = append(works, w)
		withImg[q+".jpg"] = freeImage("https://upload.wikimedia.org/" + q + ".jpg")
	}
	src := &fakeCommonsSource{fakeSource: &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": works}}, images: withImg}
	svc := withPosters(t, src, db)
	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeUnchanged || rep.Items[0].Posters != 1 || rep.Posters != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if s, _ := posterRow(t, db, "Q10"); s != "commons" {
		t.Errorf("Q10 = %q", s)
	}
	for q, want := range map[string]string{"Q11": "removed", "Q12": "", "Q13": "manual"} {
		if s, _ := posterRow(t, db, q); s != want {
			t.Errorf("%s = %q, want %q", q, s, want)
		}
	}
}

var (
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 32)...)
	pngBytes  = append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{2}, 32)...)
)

func setupUpload(t *testing.T) (*Service, *gorm.DB, *coverstore.MemStore, uuid.UUID, uuid.UUID) {
	t.Helper()
	_, db := setupAuthors(t, nil, nil)
	store := coverstore.NewMem()
	svc := withPosters(t, nil, db, WithPosterStore(store))
	book := mkBook(t, db, "published", "")
	res, err := svc.AddBookAdaptation(context.Background(), book, nil, newWork("Фильм"))
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, store, book, res.Item.ID
}

func TestUploadPoster_ReplaceAndCollect(t *testing.T) {
	ctx := context.Background()
	svc, db, store, _, id := setupUpload(t)

	mustExec(t, db, "UPDATE screen_works SET manually_edited_at = NULL WHERE id = ?", id)
	d, err := svc.UploadPoster(ctx, id, jpegBytes)
	if err != nil {
		t.Fatal(err)
	}
	p := d.Poster()
	if p == nil || p.Source != "manual" {
		t.Fatalf("poster = %+v", p)
	}
	first := *d.PosterPath
	if !coverstore.ValidName(first) || p.URL != "/posters/"+first {
		t.Fatalf("name = %q url = %q", first, p.URL)
	}
	if ok, _ := store.Exists(ctx, first); !ok {
		t.Fatal("file not stored")
	}
	if edited := scalar(t, db, "SELECT COALESCE(manually_edited_at, 'none') FROM screen_works WHERE id = ?", id); edited != "none" {
		t.Errorf("manually_edited_at was set: %s", edited)
	}

	// Same bytes again: the file stays.
	if _, err := svc.UploadPoster(ctx, id, jpegBytes); err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.Exists(ctx, first); !ok {
		t.Fatal("same-content upload deleted the file")
	}

	// A different image replaces it and the old file is collected.
	d, err = svc.UploadPoster(ctx, id, pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.Exists(ctx, first); ok {
		t.Error("previous file not collected")
	}
	if ok, _ := store.Exists(ctx, *d.PosterPath); !ok {
		t.Error("new file missing")
	}
}

func TestUploadPoster_Validation(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, id := setupUpload(t)
	if _, err := svc.UploadPoster(ctx, id, []byte("%PDF-1.4 not an image")); !errors.Is(err, domain.ErrPosterType) {
		t.Errorf("pdf: %v", err)
	}
	big := append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 5<<20)...)
	if _, err := svc.UploadPoster(ctx, id, big); !errors.Is(err, domain.ErrPosterTooLarge) {
		t.Errorf("big: %v", err)
	}
	if _, err := svc.UploadPoster(ctx, uuid.New(), jpegBytes); !errors.Is(err, domain.ErrWorkNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestRemovePoster_StatesAndCollect(t *testing.T) {
	ctx := context.Background()
	svc, db, store, _, id := setupUpload(t)

	// manual -> no poster, file collected.
	d, err := svc.UploadPoster(ctx, id, jpegBytes)
	if err != nil {
		t.Fatal(err)
	}
	name := *d.PosterPath
	d, err = svc.RemovePoster(ctx, id)
	if err != nil || d.Poster() != nil || d.PosterRemoved() {
		t.Fatalf("after manual removal: %+v %v", d, err)
	}
	if ok, _ := store.Exists(ctx, name); ok {
		t.Error("file not collected")
	}

	// commons -> removed, idempotent.
	mustExec(t, db, "UPDATE screen_works SET poster_source = 'commons', poster_url = 'https://upload.wikimedia.org/x.jpg' WHERE id = ?", id)
	d, err = svc.RemovePoster(ctx, id)
	if err != nil || !d.PosterRemoved() || d.Poster() != nil {
		t.Fatalf("after commons removal: %+v %v", d, err)
	}
	d, err = svc.RemovePoster(ctx, id)
	if err != nil || !d.PosterRemoved() {
		t.Fatalf("second removal: %+v %v", d, err)
	}
	// An upload after a removal sets manual again.
	d, err = svc.UploadPoster(ctx, id, jpegBytes)
	if err != nil || d.Poster() == nil || d.PosterRemoved() {
		t.Fatalf("upload after removal: %+v %v", d, err)
	}

	if _, err := svc.RemovePoster(ctx, uuid.New()); !errors.Is(err, domain.ErrWorkNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestDeleteWork_CollectsPoster(t *testing.T) {
	ctx := context.Background()
	svc, db, store, book, id := setupUpload(t)
	d, err := svc.UploadPoster(ctx, id, jpegBytes)
	if err != nil {
		t.Fatal(err)
	}
	name := *d.PosterPath
	mustExec(t, db, "DELETE FROM book_screen_works WHERE book_id = ?", book)
	if err := svc.DeleteWork(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.Exists(ctx, name); ok {
		t.Error("poster file left after the work was deleted")
	}
}

func TestUploadPoster_NoStore(t *testing.T) {
	_, db := setup(t, nil)
	svc := withPosters(t, nil, db)
	if _, err := svc.UploadPoster(context.Background(), uuid.New(), jpegBytes); !errors.Is(err, domain.ErrPostersUnavailable) {
		t.Errorf("err = %v", err)
	}
}
