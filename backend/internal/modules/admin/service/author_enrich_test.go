package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
)

type fakeEnrichmentSource struct {
	records    map[string]*bookdata.ExternalAuthorRecord
	images     map[string]*bookdata.CommonsImage
	fetchErr   error
	commonsErr error

	fetchCalls   [][]string
	commonsCalls [][]string
}

func (f *fakeEnrichmentSource) FetchAuthors(ctx context.Context, qids []string) (map[string]*bookdata.ExternalAuthorRecord, error) {
	f.fetchCalls = append(f.fetchCalls, append([]string(nil), qids...))
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	out := map[string]*bookdata.ExternalAuthorRecord{}
	for _, q := range qids {
		if r, ok := f.records[q]; ok {
			out[q] = r
		}
	}
	return out, nil
}

func (f *fakeEnrichmentSource) FetchCommonsImages(ctx context.Context, files []string) (map[string]*bookdata.CommonsImage, error) {
	f.commonsCalls = append(f.commonsCalls, append([]string(nil), files...))
	if f.commonsErr != nil {
		return nil, f.commonsErr
	}
	out := map[string]*bookdata.CommonsImage{}
	for _, name := range files {
		if img, ok := f.images[name]; ok {
			out[name] = img
		}
	}
	return out, nil
}

func newEnrichService() (*Service, *fakeRepository, *fakeEnrichmentSource) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	src := &fakeEnrichmentSource{records: map[string]*bookdata.ExternalAuthorRecord{}, images: map[string]*bookdata.CommonsImage{}}
	return NewService(repo, WithAuthorEnrichmentSource(src)), repo, src
}

func dayPtr(y, m, d int) *time.Time {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return &t
}

func pdImage() *bookdata.CommonsImage {
	return &bookdata.CommonsImage{
		ThumbURL: "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg",
		PageURL:  "https://commons.wikimedia.org/wiki/File:T.jpg",
		Author:   "Sergei Prokudin-Gorskii",
		License:  "Public domain",
		Free:     true,
	}
}

func tolstoyRecord() *bookdata.ExternalAuthorRecord {
	return &bookdata.ExternalAuthorRecord{
		QID: "Q7243", Name: "Лев Толстой", InstanceOf: []string{"Q5"},
		BirthDate: dayPtr(1828, 9, 9), DeathDate: dayPtr(1910, 11, 20), PhotoFile: "T.jpg",
	}
}

func TestEnrichAuthorsFromWikidata_FillsEmptyFields(t *testing.T) {
	svc, repo, src := newEnrichService()
	a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
	repo.authors = append(repo.authors, a)
	src.records["Q7243"] = tolstoyRecord()
	src.images["T.jpg"] = pdImage()

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 1 || report.Enriched != 1 {
		t.Fatalf("report = %+v", report)
	}
	item := report.Items[0]
	if item.Outcome != EnrichOutcomeEnriched || !reflect.DeepEqual(item.Filled, []string{"birth_date", "death_date", "photo"}) || len(item.Notes) != 0 {
		t.Fatalf("item = %+v", item)
	}
	got := repo.authors[0]
	if got.BirthDate == nil || !got.BirthDate.Equal(*dayPtr(1828, 9, 9)) || got.DeathDate == nil ||
		got.PhotoURL == nil || *got.PhotoURL != pdImage().ThumbURL ||
		got.PhotoAuthor == nil || *got.PhotoAuthor != "Sergei Prokudin-Gorskii" ||
		got.PhotoLicense == nil || *got.PhotoLicense != "Public domain" || got.PhotoLicenseURL != nil ||
		got.PhotoSourceURL == nil || *got.PhotoSourceURL != pdImage().PageURL {
		t.Fatalf("author not filled: %+v", got)
	}
	if len(src.fetchCalls) != 1 || len(src.commonsCalls) != 1 {
		t.Fatalf("calls: fetch=%v commons=%v", src.fetchCalls, src.commonsCalls)
	}
}

func TestEnrichAuthorsFromWikidata_KeepsExistingValues(t *testing.T) {
	svc, repo, src := newEnrichService()
	withBirth := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7243")
	withBirth.BirthDate = dayPtr(1828, 9, 10)
	manualPhoto := "https://example.com/manual.jpg"
	withPhoto := wikidataAuthor("Другой", "drugoy", catalogDomain.AuthorStatusPublished, "Q2")
	withPhoto.PhotoURL = &manualPhoto
	repo.authors = append(repo.authors, withBirth, withPhoto)
	src.records["Q7243"] = tolstoyRecord()
	src.records["Q2"] = &bookdata.ExternalAuthorRecord{QID: "Q2", PhotoFile: "T.jpg"}
	src.images["T.jpg"] = pdImage()

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{withBirth.ID, withPhoto.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(report.Items[0].Filled, []string{"death_date", "photo"}) {
		t.Fatalf("first item = %+v", report.Items[0])
	}
	if !repo.authors[0].BirthDate.Equal(*dayPtr(1828, 9, 10)) {
		t.Fatalf("birth date overwritten: %v", repo.authors[0].BirthDate)
	}
	if report.Items[1].Outcome != EnrichOutcomeUnchanged || len(report.Items[1].Filled) != 0 {
		t.Fatalf("second item = %+v", report.Items[1])
	}
	if *repo.authors[1].PhotoURL != manualPhoto || repo.authors[1].PhotoSourceURL != nil {
		t.Fatalf("manual photo or attribution touched: %+v", repo.authors[1])
	}
	// Only the author with an empty photo needs a Commons lookup.
	if len(src.commonsCalls) != 1 || !reflect.DeepEqual(src.commonsCalls[0], []string{"T.jpg"}) {
		t.Fatalf("commons calls = %v", src.commonsCalls)
	}
}

func TestEnrichAuthorsFromWikidata_SkipsAndErrors(t *testing.T) {
	svc, repo, src := newEnrichService()
	noQID := wikidataAuthor("Без QID", "bez-qid", catalogDomain.AuthorStatusPending, "")
	missing := wikidataAuthor("Пропавший", "propavshiy", catalogDomain.AuthorStatusPending, "Q404")
	repo.authors = append(repo.authors, noQID, missing)
	unknown := uuid.New()

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{noQID.ID, missing.ID, unknown, noQID.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Processed != 3 || report.Enriched != 0 {
		t.Fatalf("dedupe/processed: %+v", report)
	}
	if it := report.Items[0]; it.Outcome != EnrichOutcomeSkipped || !reflect.DeepEqual(it.Notes, []string{"no_qid"}) {
		t.Fatalf("no qid item = %+v", it)
	}
	if it := report.Items[1]; it.Outcome != EnrichOutcomeSkipped || !reflect.DeepEqual(it.Notes, []string{"entity_not_found"}) {
		t.Fatalf("missing entity item = %+v", it)
	}
	if it := report.Items[2]; it.Outcome != EnrichOutcomeError || it.Message != "author not found" {
		t.Fatalf("unknown id item = %+v", it)
	}
	if len(src.fetchCalls) != 1 || !reflect.DeepEqual(src.fetchCalls[0], []string{"Q404"}) {
		t.Fatalf("fetch calls = %v", src.fetchCalls)
	}
}

func TestEnrichAuthorsFromWikidata_PhotoRules(t *testing.T) {
	cases := []struct {
		name string
		img  *bookdata.CommonsImage
		note string
	}{
		{"missing file", nil, EnrichNotePhotoUnavailable},
		{"no thumb", &bookdata.CommonsImage{PageURL: "https://commons.wikimedia.org/wiki/File:T.jpg", Free: true}, EnrichNotePhotoUnavailable},
		{"not free", &bookdata.CommonsImage{ThumbURL: pdImage().ThumbURL, PageURL: pdImage().PageURL, License: "CC BY-NC 4.0"}, EnrichNotePhotoLicenseNotFree},
		{"cc by without author", &bookdata.CommonsImage{ThumbURL: pdImage().ThumbURL, PageURL: pdImage().PageURL, License: "CC BY 4.0", Free: true, AttributionRequired: true}, EnrichNotePhotoAttributionMiss},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, src := newEnrichService()
			a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
			repo.authors = append(repo.authors, a)
			src.records["Q7243"] = tolstoyRecord()
			if tc.img != nil {
				src.images["T.jpg"] = tc.img
			}

			report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			item := report.Items[0]
			if !reflect.DeepEqual(item.Filled, []string{"birth_date", "death_date"}) || !reflect.DeepEqual(item.Notes, []string{tc.note}) {
				t.Fatalf("item = %+v", item)
			}
			if repo.authors[0].PhotoURL != nil {
				t.Fatalf("photo must not be filled: %v", *repo.authors[0].PhotoURL)
			}
		})
	}

	t.Run("pd without author is allowed", func(t *testing.T) {
		svc, repo, src := newEnrichService()
		a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
		repo.authors = append(repo.authors, a)
		src.records["Q7243"] = &bookdata.ExternalAuthorRecord{QID: "Q7243", PhotoFile: "T.jpg"}
		img := pdImage()
		img.Author = ""
		src.images["T.jpg"] = img

		report, _ := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
		if !reflect.DeepEqual(report.Items[0].Filled, []string{"photo"}) || repo.authors[0].PhotoAuthor != nil {
			t.Fatalf("item = %+v author = %+v", report.Items[0], repo.authors[0])
		}
	})
}

func TestEnrichAuthorsFromWikidata_UnusableDateNotes(t *testing.T) {
	svc, repo, src := newEnrichService()
	a := wikidataAuthor("Автор", "avtor", catalogDomain.AuthorStatusPending, "Q1")
	repo.authors = append(repo.authors, a)
	src.records["Q1"] = &bookdata.ExternalAuthorRecord{QID: "Q1", BirthDateUnusable: true, DeathDateUnusable: true}

	report, _ := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
	item := report.Items[0]
	if item.Outcome != EnrichOutcomeUnchanged || !reflect.DeepEqual(item.Notes, []string{"birth_date_unusable", "death_date_unusable"}) {
		t.Fatalf("item = %+v", item)
	}
	if len(repo.fillCalls) != 0 {
		t.Fatalf("no fill expected, got %d", len(repo.fillCalls))
	}
}

func TestEnrichAuthorsFromWikidata_CommonsFailureStillAppliesDates(t *testing.T) {
	svc, repo, src := newEnrichService()
	a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
	repo.authors = append(repo.authors, a)
	src.records["Q7243"] = tolstoyRecord()
	src.commonsErr = bookdata.ErrSourceUnavailable

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	item := report.Items[0]
	if item.Outcome != EnrichOutcomeEnriched || !reflect.DeepEqual(item.Filled, []string{"birth_date", "death_date"}) ||
		!reflect.DeepEqual(item.Notes, []string{EnrichNoteCommonsUnavailable}) {
		t.Fatalf("item = %+v", item)
	}
}

func TestEnrichAuthorsFromWikidata_EntityFetchFailure(t *testing.T) {
	svc, repo, src := newEnrichService()
	a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
	repo.authors = append(repo.authors, a)
	src.fetchErr = bookdata.ErrRateLimited

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID})
	if !errors.Is(err, bookdata.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if report == nil || report.Items[0].Outcome != EnrichOutcomeError || report.Items[0].Notes[0] != EnrichNoteWikidataUnavailable {
		t.Fatalf("report = %+v", report)
	}
}

func TestEnrichAuthorsFromWikidata_FillErrorIsRowError(t *testing.T) {
	svc, repo, src := newEnrichService()
	a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPending, "Q7243")
	b := wikidataAuthor("Другой", "drugoy", catalogDomain.AuthorStatusPending, "Q2")
	repo.authors = append(repo.authors, a, b)
	src.records["Q7243"] = tolstoyRecord()
	src.records["Q2"] = &bookdata.ExternalAuthorRecord{QID: "Q2"}
	src.images["T.jpg"] = pdImage()
	repo.fillErr = errors.New("db down")

	report, err := svc.EnrichAuthorsFromWikidata(context.Background(), []uuid.UUID{a.ID, b.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Items[0].Outcome != EnrichOutcomeError || report.Items[0].Message == "" || report.Items[1].Outcome != EnrichOutcomeUnchanged {
		t.Fatalf("report = %+v", report)
	}
}

func TestEnrichAuthorsFromWikidata_Limits(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newEnrichService()
	ids := make([]uuid.UUID, 51)
	for i := range ids {
		ids[i] = uuid.New()
	}
	if _, err := svc.EnrichAuthorsFromWikidata(ctx, ids); !errors.Is(err, ErrAuthorEnrichmentTooLarge) {
		t.Fatalf("err = %v, want ErrAuthorEnrichmentTooLarge", err)
	}
	if _, err := svc.EnrichAuthorsFromWikidata(ctx, nil); !errors.Is(err, ErrAuthorEnrichmentEmpty) {
		t.Fatalf("err = %v, want ErrAuthorEnrichmentEmpty", err)
	}
	plain, _ := newService()
	if _, err := plain.EnrichAuthorsFromWikidata(ctx, ids[:1]); !errors.Is(err, ErrAuthorEnrichmentUnavailable) {
		t.Fatalf("err = %v, want ErrAuthorEnrichmentUnavailable", err)
	}
}

func TestEnrichHarvestAuthors_ChunksBy50(t *testing.T) {
	svc, repo, src := newEnrichService()
	var ids []uuid.UUID
	for i := 0; i < 60; i++ {
		qid := fmt.Sprintf("Q%d", i+1)
		a := wikidataAuthor("Автор "+qid, "avtor-"+qid, catalogDomain.AuthorStatusPending, qid)
		repo.authors = append(repo.authors, a)
		ids = append(ids, a.ID)
		src.records[qid] = &bookdata.ExternalAuthorRecord{QID: qid, BirthDate: dayPtr(1900, 1, 1)}
	}

	report, err := svc.EnrichHarvestAuthors(context.Background(), ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(src.fetchCalls) != 2 || len(src.fetchCalls[0]) != 50 || len(src.fetchCalls[1]) != 10 {
		t.Fatalf("fetch chunks = %d", len(src.fetchCalls))
	}
	if report.Enriched != 60 || len(src.commonsCalls) != 0 {
		t.Fatalf("report enriched = %d, commons calls = %d", report.Enriched, len(src.commonsCalls))
	}
}
