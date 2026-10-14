package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

func TestResolve(t *testing.T) {
	wd := &fakeWikidata{
		hits: map[string][]bookdata.AuthorCandidate{
			"Иван Гончаров": {
				{QID: "Q192279", Label: "Иван Гончаров", Description: "русский писатель"},
				{QID: "Q900", Label: "Обломов", Description: "роман"},
			},
			"Иван Петров": {
				{QID: "Q1", Description: "поэт"},
				{QID: "Q2", Description: "прозаик"},
			},
			"Редирект": {
				{QID: "Q10", Description: "a"},
				{QID: "Q11", Description: "b"},
			},
			"Роман": {{QID: "Q900", Description: "роман"}},
		},
		records: map[string]*bookdata.ExternalAuthorRecord{
			"Q192279": person("Q192279", "Иван Александрович Гончаров"),
			"Q900":    {QID: "Q900", Name: "Обломов", InstanceOf: []string{"Q7725634"}},
			"Q1":      person("Q1", "Иван Петров"),
			"Q2":      person("Q2", "Иван Петров"),
			// Q11 is a redirect to Q10.
			"Q10": person("Q10", "Пётр Редиректов"),
			"Q11": person("Q10", "Пётр Редиректов"),
		},
	}
	r := NewResolver(wd, wd, eligible)
	ctx := context.Background()

	status, opts, err := r.Resolve(ctx, "Иван Гончаров")
	if err != nil || status != domain.MatchMatched || len(opts) != 1 ||
		opts[0] != (domain.Option{QID: "Q192279", Label: "Иван Александрович Гончаров", Description: "русский писатель"}) {
		t.Errorf("matched: %s %+v %v", status, opts, err)
	}

	status, opts, _ = r.Resolve(ctx, "Иван Петров")
	if status != domain.MatchAmbiguous || len(opts) != 2 || opts[0].QID != "Q1" || opts[1].QID != "Q2" {
		t.Errorf("ambiguous: %s %+v", status, opts)
	}

	status, opts, _ = r.Resolve(ctx, "Редирект")
	if status != domain.MatchMatched || len(opts) != 1 || opts[0].QID != "Q10" {
		t.Errorf("redirect: %s %+v", status, opts)
	}

	status, opts, _ = r.Resolve(ctx, "Роман")
	if status != domain.MatchNotFound || opts == nil || len(opts) != 0 {
		t.Errorf("only non-persons: %s %+v", status, opts)
	}

	before := wd.fetchCalls
	status, opts, _ = r.Resolve(ctx, "Выдуманный Автор")
	if status != domain.MatchNotFound || opts == nil || wd.fetchCalls != before {
		t.Errorf("no hits: %s %+v fetchCalls=%d", status, opts, wd.fetchCalls-before)
	}
}

func TestResolve_Errors(t *testing.T) {
	wd := &fakeWikidata{searchErr: map[string]error{
		"bad":  fmt.Errorf("%w: empty", bookdata.ErrInvalidQuery),
		"down": bookdata.ErrSourceUnavailable,
		"rate": bookdata.ErrRateLimited,
	}}
	r := NewResolver(wd, wd, eligible)
	if status, _, err := r.Resolve(context.Background(), "bad"); err != nil || status != domain.MatchNotFound {
		t.Errorf("invalid query: %s %v", status, err)
	}
	for _, q := range []string{"down", "rate"} {
		if _, _, err := r.Resolve(context.Background(), q); err == nil {
			t.Errorf("%s: expected error", q)
		}
	}

	wd2 := &fakeWikidata{
		hits:     map[string][]bookdata.AuthorCandidate{"x": {{QID: "Q1"}}},
		fetchErr: bookdata.ErrSourceUnavailable,
	}
	if _, _, err := NewResolver(wd2, wd2, eligible).Resolve(context.Background(), "x"); !errors.Is(err, bookdata.ErrSourceUnavailable) {
		t.Errorf("fetch unavailable: %v", err)
	}
}
