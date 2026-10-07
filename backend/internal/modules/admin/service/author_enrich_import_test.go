package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
)

func newImportEnrichService(record *bookdata.ExternalAuthorRecord, withSource bool) (*Service, *fakeRepository, *fakeEnrichmentSource) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	src := &fakeEnrichmentSource{records: map[string]*bookdata.ExternalAuthorRecord{}, images: map[string]*bookdata.CommonsImage{}}
	opts := []Option{WithAuthorFetcher(&fakeAuthorFetcher{record: record})}
	if withSource {
		opts = append(opts, WithAuthorEnrichmentSource(src))
	}
	return NewService(repo, opts...), repo, src
}

func TestImportWikidataAuthor_CreatedIsEnriched(t *testing.T) {
	svc, repo, src := newImportEnrichService(tolstoyRecord(), true)
	src.images["T.jpg"] = pdImage()

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != AuthorOutcomeCreated || res.Enrichment == nil {
		t.Fatalf("result = %+v", res)
	}
	if !reflect.DeepEqual(res.Enrichment.Filled, []string{"birth_date", "death_date", "photo"}) || res.Enrichment.Error != "" {
		t.Fatalf("enrichment = %+v", res.Enrichment)
	}
	// The entity is not fetched again: dates come from FetchAuthor's record.
	if len(src.fetchCalls) != 0 || len(src.commonsCalls) != 1 {
		t.Fatalf("calls: fetch=%v commons=%v", src.fetchCalls, src.commonsCalls)
	}
	if res.Author.PhotoURL == nil || repo.authors[0].BirthDate == nil || repo.authors[0].PhotoSourceURL == nil {
		t.Fatalf("author not enriched: result=%+v stored=%+v", res.Author, repo.authors[0])
	}
}

func TestImportWikidataAuthor_EnrichmentFailureKeepsCreated(t *testing.T) {
	svc, repo, src := newImportEnrichService(tolstoyRecord(), true)
	src.images["T.jpg"] = pdImage()
	repo.fillErr = errors.New("db down")

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil {
		t.Fatalf("import must not fail: %v", err)
	}
	if res.Outcome != AuthorOutcomeCreated || res.Enrichment == nil || res.Enrichment.Error == "" || len(res.Enrichment.Filled) != 0 {
		t.Fatalf("result = %+v enrichment = %+v", res, res.Enrichment)
	}
}

func TestImportWikidataAuthor_ExistingIsNotEnriched(t *testing.T) {
	svc, repo, src := newImportEnrichService(tolstoyRecord(), true)
	repo.authors = append(repo.authors, wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7243"))

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != AuthorOutcomeExisting || res.Enrichment != nil || len(src.commonsCalls) != 0 || len(repo.fillCalls) != 0 {
		t.Fatalf("existing must not be enriched: %+v", res)
	}
}

func TestImportWikidataAuthor_NoSourceAppliesDatesOnly(t *testing.T) {
	svc, repo, _ := newImportEnrichService(tolstoyRecord(), false)

	res, err := svc.ImportWikidataAuthor(context.Background(), "Q7243")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Enrichment == nil || !reflect.DeepEqual(res.Enrichment.Filled, []string{"birth_date", "death_date"}) || len(res.Enrichment.Notes) != 0 {
		t.Fatalf("enrichment = %+v", res.Enrichment)
	}
	if repo.authors[0].PhotoURL != nil {
		t.Fatal("photo must not be set without a source")
	}
}

func TestUpdateAuthor_PhotoChangeClearsAttribution(t *testing.T) {
	photo := "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg"
	attributed := func() catalogDomain.Author {
		a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7243")
		a.PhotoURL = strPtr(photo)
		a.PhotoAuthor = strPtr("Sergei Prokudin-Gorskii")
		a.PhotoLicense = strPtr("Public domain")
		a.PhotoSourceURL = strPtr("https://commons.wikimedia.org/wiki/File:T.jpg")
		return a
	}
	cases := []struct {
		name      string
		photo     *string
		wantClear bool
	}{
		{"unchanged keeps attribution", strPtr(" " + photo + " "), false},
		{"changed clears attribution", strPtr("https://example.com/other.jpg"), true},
		{"cleared clears attribution", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newService()
			a := attributed()
			repo.authors = append(repo.authors, a)

			updated, err := svc.UpdateAuthor(context.Background(), a.ID, AuthorInput{Name: a.Name, Slug: a.Slug, PhotoURL: tc.photo})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cleared := updated.PhotoAuthor == nil && updated.PhotoLicense == nil && updated.PhotoLicenseURL == nil && updated.PhotoSourceURL == nil
			if cleared != tc.wantClear {
				t.Fatalf("cleared = %v, want %v (%+v)", cleared, tc.wantClear, updated)
			}
		})
	}
}

func TestResolveHarvestAuthorRefs_ReportsCreatedWithQID(t *testing.T) {
	svc, repo := newService()
	existing := wikidataAuthor("Лев Толстой", slugify("Лев Толстой"), catalogDomain.AuthorStatusPublished, "Q7243")
	repo.authors = append(repo.authors, existing)

	res, err := svc.ResolveHarvestAuthorRefs(context.Background(), []AuthorRef{
		{Name: "Лев Толстой", QID: "Q7243"},  // existing → linked, not reported
		{Name: "Антон Чехов", QID: "Q5685"},  // created with QID → reported
		{Name: "Неизвестный Автор", QID: ""}, // created without QID → not reported
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.IDs) != 3 || len(res.CreatedWithQID) != 1 {
		t.Fatalf("resolution = %+v", res)
	}
	created := findAuthorByID(repo.authors, res.CreatedWithQID[0])
	if created == nil || created.Name != "Антон Чехов" {
		t.Fatalf("unexpected created author %+v", created)
	}
}

func findAuthorByID(authors []catalogDomain.Author, id uuid.UUID) *catalogDomain.Author {
	for i := range authors {
		if authors[i].ID == id {
			return &authors[i]
		}
	}
	return nil
}
