package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Read helpers for the AI genre suggestions (openspec capability
// catalog-moderation/ai-genre-suggestions). They only read.

// maxHintBucketFull is the largest bucket whose members are all paired with
// each other; larger buckets are chained (consecutive members only), which is
// enough to keep them in one batch without a quadratic number of pairs.
const maxHintBucketFull = 8

// GenreAnalysisCandidate is one genre of the AI analysis pool.
type GenreAnalysisCandidate struct {
	Genre      catalogDomain.Genre
	BooksTotal int64
}

// GenreState is the current state of a genre, alias and rejected rows
// included.
type GenreState struct {
	ID           uuid.UUID
	Name         string
	Status       string
	MergedIntoID *uuid.UUID
	WikidataQID  *string
	BooksTotal   int64
}

// GenreHintPair is a similar-name hint between two genres of a set.
type GenreHintPair struct {
	A     uuid.UUID
	B     uuid.UUID
	Match string
}

// ListGenreAnalysisCandidates returns the non-alias pending/published genres
// with their linked-books counts.
func (s *Service) ListGenreAnalysisCandidates(ctx context.Context) ([]GenreAnalysisCandidate, error) {
	rows, err := s.repo.ListGenreAnalysisCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list genre analysis candidates: %w", err)
	}
	out := make([]GenreAnalysisCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, GenreAnalysisCandidate{Genre: row.Genre, BooksTotal: row.BooksTotal})
	}
	return out, nil
}

// ListGenreSampleBooks returns up to 3 linked books per genre (by title).
func (s *Service) ListGenreSampleBooks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]repository.SampleBook, error) {
	samples, err := s.repo.ListGenreSampleBooks(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list genre sample books: %w", err)
	}
	return samples, nil
}

// GenresByIDs returns the current state of the genres among ids; missing ids
// are absent from the map.
func (s *Service) GenresByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]GenreState, error) {
	rows, err := s.repo.GenresByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("genres by ids: %w", err)
	}
	out := make(map[uuid.UUID]GenreState, len(rows))
	for _, row := range rows {
		out[row.ID] = GenreState(row)
	}
	return out, nil
}

// GenreHintPairs returns the similar-name hint pairs (same rules as the genre
// queue: exact_name, same_stem, shared_word) among genres. Names are
// normalized once and bucketed, so a catalog of thousands of genres stays
// cheap. Each unordered pair appears once with its strongest match kind.
func (s *Service) GenreHintPairs(_ context.Context, genres []catalogDomain.Genre) []GenreHintPair {
	type entry struct {
		id    uuid.UUID
		norm  string
		stems string
		words []string
	}
	entries := make([]entry, 0, len(genres))
	for _, g := range genres {
		norm := normalizeGenreName(g.Name)
		if norm == "" {
			continue
		}
		stems := genreStems(g.Name)
		var words []string
		seen := map[string]struct{}{}
		for _, st := range stems {
			if utf8.RuneCountInString(st) < minSharedWordRunes {
				continue
			}
			if _, dup := seen[st]; dup {
				continue
			}
			seen[st] = struct{}{}
			words = append(words, st)
		}
		entries = append(entries, entry{id: g.ID, norm: norm, stems: strings.Join(stems, " "), words: words})
	}

	exact := map[string][]uuid.UUID{}
	stem := map[string][]uuid.UUID{}
	word := map[string][]uuid.UUID{}
	for _, e := range entries {
		exact[e.norm] = append(exact[e.norm], e.id)
		stem[e.stems] = append(stem[e.stems], e.id)
		for _, w := range e.words {
			word[w] = append(word[w], e.id)
		}
	}

	type key struct{ a, b uuid.UUID }
	found := map[key]string{}
	var order []key
	emit := func(buckets map[string][]uuid.UUID, kind string) {
		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		add := func(x, y uuid.UUID) {
			if x == y {
				return
			}
			if x.String() > y.String() {
				x, y = y, x
			}
			k := key{x, y}
			if _, ok := found[k]; ok {
				return
			}
			found[k] = kind
			order = append(order, k)
		}
		for _, k := range keys {
			members := buckets[k]
			if len(members) < 2 {
				continue
			}
			if len(members) <= maxHintBucketFull {
				for i := 0; i < len(members); i++ {
					for j := i + 1; j < len(members); j++ {
						add(members[i], members[j])
					}
				}
				continue
			}
			for i := 1; i < len(members); i++ {
				add(members[i-1], members[i])
			}
		}
	}
	emit(exact, GenreMatchExactName)
	emit(stem, GenreMatchSameStem)
	emit(word, GenreMatchSharedWord)

	out := make([]GenreHintPair, 0, len(order))
	for _, k := range order {
		out = append(out, GenreHintPair{A: k.a, B: k.b, Match: found[k]})
	}
	return out
}
