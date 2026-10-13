package service

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeAuthorFetcher struct {
	record *bookdata.ExternalAuthorRecord
	err    error
	calls  []string
}

func (f *fakeAuthorFetcher) FetchAuthor(ctx context.Context, qid string) (*bookdata.ExternalAuthorRecord, error) {
	f.calls = append(f.calls, qid)
	if f.err != nil {
		return nil, f.err
	}
	return f.record, nil
}

func newWikidataImportService(record *bookdata.ExternalAuthorRecord) (*Service, *fakeRepository, *fakeAuthorFetcher) {
	repo := &fakeRepository{authorsExist: true, genresExist: true}
	fetcher := &fakeAuthorFetcher{record: record}
	return NewService(repo, WithAuthorFetcher(fetcher)), repo, fetcher
}

func person(qid, name string) *bookdata.ExternalAuthorRecord {
	return &bookdata.ExternalAuthorRecord{QID: qid, Name: name, InstanceOf: []string{"Q5"}}
}

func TestImportWikidataAuthor_QIDMatchShortCircuits(t *testing.T) {
	ctx := context.Background()
	for _, status := range []string{catalogDomain.AuthorStatusPublished, catalogDomain.AuthorStatusPending} {
		t.Run(status, func(t *testing.T) {
			svc, repo, fetcher := newWikidataImportService(person("Q7200", "Лев Толстой"))
			existing := wikidataAuthor("Лев Толстой", "lev-tolstoy", status, "Q7200")
			repo.authors = append(repo.authors, existing)

			res, err := svc.ImportWikidataAuthor(ctx, "Q7200")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Outcome != AuthorOutcomeExisting || res.Match != AuthorMatchQID || res.Author.ID != existing.ID {
				t.Fatalf("unexpected result %+v", res)
			}
			if len(fetcher.calls) != 0 {
				t.Fatalf("fetcher must not be called, got %v", fetcher.calls)
			}
		})
	}

	t.Run("rejected", func(t *testing.T) {
		svc, repo, fetcher := newWikidataImportService(person("Q1", "Мусор"))
		repo.authors = append(repo.authors, wikidataAuthor("Мусор", "musor", catalogDomain.AuthorStatusRejected, "Q1"))

		res, err := svc.ImportWikidataAuthor(ctx, "Q1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeRejected || res.Match != AuthorMatchQID {
			t.Fatalf("unexpected result %+v", res)
		}
		if len(fetcher.calls) != 0 || len(repo.authors) != 1 {
			t.Fatalf("expected no fetch and nothing created, calls=%v authors=%d", fetcher.calls, len(repo.authors))
		}
	})

	t.Run("nil fetcher still short-circuits", func(t *testing.T) {
		svc, repo := newService()
		repo.authors = append(repo.authors, wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7200"))
		res, err := svc.ImportWikidataAuthor(ctx, "Q7200")
		if err != nil || res.Outcome != AuthorOutcomeExisting {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestImportWikidataAuthor_CreatesPendingWithQID(t *testing.T) {
	svc, repo, fetcher := newWikidataImportService(person("Q7200", "Лев Толстой"))

	res, err := svc.ImportWikidataAuthor(context.Background(), "  q7200 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != "Q7200" {
		t.Fatalf("expected normalized fetch of Q7200, got %v", fetcher.calls)
	}
	if res.Outcome != AuthorOutcomeCreated || res.Match != "" || res.NamesakeOf != nil {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(repo.authors) != 1 {
		t.Fatalf("expected one author, got %d", len(repo.authors))
	}
	a := repo.authors[0]
	if a.Name != "Лев Толстой" || a.Status != catalogDomain.AuthorStatusPending ||
		a.ExternalSource == nil || *a.ExternalSource != "wikidata" || a.ExternalID == nil || *a.ExternalID != "Q7200" {
		t.Fatalf("unexpected created author %+v", a)
	}
}

func TestImportWikidataAuthor_SlugMatch(t *testing.T) {
	ctx := context.Background()

	t.Run("without QID backfills", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q7200", "Лев Толстой"))
		existing := wikidataAuthor("Лев Толстой", slugify("Лев Толстой"), catalogDomain.AuthorStatusPublished, "")
		repo.authors = append(repo.authors, existing)

		res, err := svc.ImportWikidataAuthor(ctx, "Q7200")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeExisting || res.Match != AuthorMatchSlug || !res.QIDBackfilled || res.Author.ID != existing.ID {
			t.Fatalf("unexpected result %+v", res)
		}
		a := repo.authors[0]
		if a.ExternalID == nil || *a.ExternalID != "Q7200" || a.Status != catalogDomain.AuthorStatusPublished || len(repo.authors) != 1 {
			t.Fatalf("expected backfill with unchanged status, got %+v", a)
		}
	})

	t.Run("rejected backfills and is refused", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q9", "Иван Петров"))
		repo.authors = append(repo.authors, wikidataAuthor("Иван Петров", slugify("Иван Петров"), catalogDomain.AuthorStatusRejected, ""))

		res, err := svc.ImportWikidataAuthor(ctx, "Q9")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeRejected || res.Match != AuthorMatchSlug || !res.QIDBackfilled {
			t.Fatalf("unexpected result %+v", res)
		}
		if len(repo.authors) != 1 || *repo.authors[0].ExternalID != "Q9" || repo.authors[0].Status != catalogDomain.AuthorStatusRejected {
			t.Fatalf("unexpected authors %+v", repo.authors)
		}
	})

	t.Run("namesake with different QID", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q12345", "Иван Петров"))
		other := wikidataAuthor("Иван Петров", slugify("Иван Петров"), catalogDomain.AuthorStatusPublished, "Q100")
		repo.authors = append(repo.authors, other)

		res, err := svc.ImportWikidataAuthor(ctx, "Q12345")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeCreated || res.NamesakeOf == nil || res.NamesakeOf.ID != other.ID {
			t.Fatalf("unexpected result %+v", res)
		}
		if res.Author.Slug != slugify("Иван Петров")+"-q12345" || res.Author.Status != catalogDomain.AuthorStatusPending {
			t.Fatalf("unexpected namesake %+v", res.Author)
		}
	})
}

func TestImportWikidataAuthor_RedirectUsesCanonicalQID(t *testing.T) {
	ctx := context.Background()

	t.Run("canonical QID stored on create", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q200", "Автор Новый"))
		res, err := svc.ImportWikidataAuthor(ctx, "Q100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeCreated || *repo.authors[0].ExternalID != "Q200" {
			t.Fatalf("expected author with canonical Q200, got %+v", repo.authors[0])
		}
	})

	t.Run("canonical QID matched in DB", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q200", "Автор Новый"))
		existing := wikidataAuthor("Другое Имя", "drugoe-imya", catalogDomain.AuthorStatusPublished, "Q200")
		repo.authors = append(repo.authors, existing)

		res, err := svc.ImportWikidataAuthor(ctx, "Q100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeExisting || res.Match != AuthorMatchQID || res.Author.ID != existing.ID || len(repo.authors) != 1 {
			t.Fatalf("unexpected result %+v", res)
		}
	})
}

func TestImportWikidataAuthor_Refusals(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		record *bookdata.ExternalAuthorRecord
		want   error
	}{
		{"no ru label", &bookdata.ExternalAuthorRecord{QID: "Q1", InstanceOf: []string{"Q5"}}, ErrNoRussianLabel},
		{"latin label", &bookdata.ExternalAuthorRecord{QID: "Q1", Name: "Leo Tolstoy", InstanceOf: []string{"Q5"}}, ErrNoRussianLabel},
		{"not a person", &bookdata.ExternalAuthorRecord{QID: "Q1", Name: "Война и мир", InstanceOf: []string{"Q7725634"}}, ErrNotAPerson},
		{"no P31", &bookdata.ExternalAuthorRecord{QID: "Q1", Name: "Война и мир"}, ErrNotAPerson},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, _ := newWikidataImportService(tc.record)
			if _, err := svc.ImportWikidataAuthor(ctx, "Q1"); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(repo.authors) != 0 {
				t.Fatalf("nothing must be created, got %+v", repo.authors)
			}
		})
	}

	for _, p31 := range []string{"Q5", "Q61002", "Q16017119"} {
		t.Run("accepts "+p31, func(t *testing.T) {
			svc, _, _ := newWikidataImportService(&bookdata.ExternalAuthorRecord{QID: "Q1", Name: "Козьма Прутков", InstanceOf: []string{"Q123", p31}})
			res, err := svc.ImportWikidataAuthor(ctx, "Q1")
			if err != nil || res.Outcome != AuthorOutcomeCreated {
				t.Fatalf("res=%+v err=%v", res, err)
			}
		})
	}
}

func TestImportWikidataAuthor_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid QID", func(t *testing.T) {
		svc, _, fetcher := newWikidataImportService(nil)
		for _, qid := range []string{"", "  ", "7200", "QX", "P31", "Q7200 Q1"} {
			if _, err := svc.ImportWikidataAuthor(ctx, qid); !errors.Is(err, ErrInvalidQID) {
				t.Errorf("ImportWikidataAuthor(%q) err = %v, want ErrInvalidQID", qid, err)
			}
		}
		if len(fetcher.calls) != 0 {
			t.Fatalf("fetcher must not be called, got %v", fetcher.calls)
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		svc, _, fetcher := newWikidataImportService(nil)
		fetcher.err = bookdata.ErrNotFound
		if _, err := svc.ImportWikidataAuthor(ctx, "Q1"); !errors.Is(err, ErrWikidataEntityNotFound) {
			t.Fatalf("err = %v, want ErrWikidataEntityNotFound", err)
		}
	})

	t.Run("rate limited passes through", func(t *testing.T) {
		svc, _, fetcher := newWikidataImportService(nil)
		fetcher.err = bookdata.ErrRateLimited
		if _, err := svc.ImportWikidataAuthor(ctx, "Q1"); !errors.Is(err, bookdata.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})

	t.Run("nil fetcher", func(t *testing.T) {
		svc, _ := newService()
		if _, err := svc.ImportWikidataAuthor(ctx, "Q1"); !errors.Is(err, ErrAuthorImportUnavailable) {
			t.Fatalf("err = %v, want ErrAuthorImportUnavailable", err)
		}
	})
}

func TestImportWikidataAuthor_DuplicateRetry(t *testing.T) {
	ctx := context.Background()
	dup := &pgconn.PgError{Code: "23505"}

	t.Run("retry finds the concurrently created author", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q7200", "Лев Толстой"))
		repo.createAuthorErrs = []error{dup}
		var concurrent catalogDomain.Author
		repo.onCreateAuthorErr = func(f *fakeRepository) {
			concurrent = wikidataAuthor("Лев Толстой", slugify("Лев Толстой"), catalogDomain.AuthorStatusPending, "Q7200")
			f.authors = append(f.authors, concurrent)
		}

		res, err := svc.ImportWikidataAuthor(ctx, "Q7200")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != AuthorOutcomeExisting || res.Match != AuthorMatchQID || res.Author.ID != concurrent.ID {
			t.Fatalf("unexpected result %+v", res)
		}
	})

	t.Run("second duplicate fails", func(t *testing.T) {
		svc, repo, _ := newWikidataImportService(person("Q7200", "Лев Толстой"))
		repo.createAuthorErrs = []error{dup, dup}
		if _, err := svc.ImportWikidataAuthor(ctx, "Q7200"); !errors.Is(err, ErrDuplicateSlug) {
			t.Fatalf("err = %v, want ErrDuplicateSlug", err)
		}
	})
}

func TestFindAuthorsByWikidataQIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("maps by QID in one query", func(t *testing.T) {
		svc, repo := newService()
		a := wikidataAuthor("Лев Толстой", "lev-tolstoy", catalogDomain.AuthorStatusPublished, "Q7200")
		b := wikidataAuthor("Мусор", "musor", catalogDomain.AuthorStatusRejected, "Q1")
		repo.authors = append(repo.authors, a, b, wikidataAuthor("Без QID", "bez-qid", catalogDomain.AuthorStatusPublished, ""))

		got, err := svc.FindAuthorsByWikidataQIDs(ctx, []string{"Q7200", "Q1", "Q999"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got["Q7200"].ID != a.ID || got["Q1"].ID != b.ID {
			t.Fatalf("unexpected map %+v", got)
		}
		if repo.listByExternalIDsCalls != 1 {
			t.Fatalf("expected one repository call, got %d", repo.listByExternalIDsCalls)
		}
	})

	t.Run("empty input skips the query", func(t *testing.T) {
		svc, repo := newService()
		got, err := svc.FindAuthorsByWikidataQIDs(ctx, nil)
		if err != nil || len(got) != 0 || repo.listByExternalIDsCalls != 0 {
			t.Fatalf("got=%v err=%v calls=%d", got, err, repo.listByExternalIDsCalls)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		svc, repo := newService()
		repo.listByExternalIDsErr = errors.New("db down")
		if _, err := svc.FindAuthorsByWikidataQIDs(ctx, []string{"Q1"}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestIsImportableAuthorRecord(t *testing.T) {
	tests := []struct {
		name   string
		record *bookdata.ExternalAuthorRecord
		want   bool
	}{
		{"human", &bookdata.ExternalAuthorRecord{QID: "Q991", Name: "Фёдор Достоевский", InstanceOf: []string{"Q5"}}, true},
		{"collective pseudonym", &bookdata.ExternalAuthorRecord{QID: "Q1", Name: "Козьма Прутков", InstanceOf: []string{"Q16017119"}}, true},
		{"pseudonym", &bookdata.ExternalAuthorRecord{QID: "Q2", Name: "Пётр Иванов", InstanceOf: []string{"Q61002"}}, true},
		{"novel", &bookdata.ExternalAuthorRecord{QID: "Q3", Name: "Обломов", InstanceOf: []string{"Q7725634"}}, false},
		{"empty ru label", &bookdata.ExternalAuthorRecord{QID: "Q4", Name: "  ", InstanceOf: []string{"Q5"}}, false},
		{"latin-only label", &bookdata.ExternalAuthorRecord{QID: "Q5", Name: "Ursula K. Le Guin", InstanceOf: []string{"Q5"}}, false},
		{"no instance of", &bookdata.ExternalAuthorRecord{QID: "Q6", Name: "Иван Гончаров"}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		if got := IsImportableAuthorRecord(tt.record); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
