package service

import (
	"context"
	"errors"
	"strings"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// AuthorSearcher is the Wikidata author search (wbsearchentities, at most
// 10 hits; satisfied by *bookdata.WikidataClient).
type AuthorSearcher interface {
	SearchAuthors(ctx context.Context, query string) ([]bookdata.AuthorCandidate, error)
}

// AuthorsFetcher fetches Wikidata entities in one batch (satisfied by
// *bookdata.WikidataClient).
type AuthorsFetcher interface {
	FetchAuthors(ctx context.Context, qids []string) (map[string]*bookdata.ExternalAuthorRecord, error)
}

// AuthorEligibility reports whether a fetched entity is an importable
// author (adminservice.IsImportableAuthorRecord in production).
type AuthorEligibility func(record *bookdata.ExternalAuthorRecord) bool

// Resolver resolves a proposed name to Wikidata people.
type Resolver struct {
	searcher AuthorSearcher
	fetcher  AuthorsFetcher
	eligible AuthorEligibility
}

// NewResolver builds a Resolver. Pacing comes from the shared
// WikidataClient's limiter; there is no extra sleep.
func NewResolver(searcher AuthorSearcher, fetcher AuthorsFetcher, eligible AuthorEligibility) *Resolver {
	return &Resolver{searcher: searcher, fetcher: fetcher, eligible: eligible}
}

// Resolve searches name, fetches the hits in one call and keeps the
// importable people, deduplicated by canonical QID in search order: 0
// options → not_found, 1 → matched, 2+ → ambiguous. bookdata.ErrInvalidQuery
// means not_found; any other error (rate limited, source unavailable) is
// returned so the task can retry. The returned options are never nil.
func (r *Resolver) Resolve(ctx context.Context, name string) (string, []domain.Option, error) {
	hits, err := r.searcher.SearchAuthors(ctx, name)
	if err != nil {
		if errors.Is(err, bookdata.ErrInvalidQuery) {
			return domain.MatchNotFound, []domain.Option{}, nil
		}
		return "", nil, err
	}
	if len(hits) == 0 {
		return domain.MatchNotFound, []domain.Option{}, nil
	}
	qids := make([]string, 0, len(hits))
	for _, h := range hits {
		qids = append(qids, h.QID)
	}
	records, err := r.fetcher.FetchAuthors(ctx, qids)
	if err != nil {
		if errors.Is(err, bookdata.ErrInvalidQuery) {
			return domain.MatchNotFound, []domain.Option{}, nil
		}
		return "", nil, err
	}

	options := make([]domain.Option, 0, len(hits))
	seen := make(map[string]struct{}, len(hits))
	for _, h := range hits {
		rec := records[h.QID]
		if rec == nil || !r.eligible(rec) {
			continue
		}
		qid := strings.TrimSpace(rec.QID)
		if qid == "" {
			qid = h.QID
		}
		if _, dup := seen[qid]; dup {
			continue
		}
		seen[qid] = struct{}{}
		options = append(options, domain.Option{
			QID:         qid,
			Label:       strings.TrimSpace(rec.Name),
			Description: h.Description,
		})
	}
	switch len(options) {
	case 0:
		return domain.MatchNotFound, options, nil
	case 1:
		return domain.MatchMatched, options, nil
	default:
		return domain.MatchAmbiguous, options, nil
	}
}
