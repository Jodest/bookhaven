package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Similar-books limits and pools (see docs/features/similar-books.md).
const (
	// DefaultSimilarLimit is used when the requested limit is missing or <= 0.
	DefaultSimilarLimit = 8
	// MaxSimilarLimit is the size of the computed/cached ranking and the
	// upper bound for the requested limit.
	MaxSimilarLimit = 20

	authorPool      = 40
	genrePool       = 100
	perAuthorCap    = 2
	similarCacheTTL = 6 * time.Hour
)

// Score weights. score = AuthorWeight*authorMatch + GenreWeight*jaccard +
// ratingBonus + popBonus. The quality bonus (at most RatingBonusMax +
// PopBonusMax = 0.6) never lifts a genre-only match above a same-author
// match (3 > 2 + 0.6).
const (
	AuthorWeight   = 3.0
	GenreWeight    = 2.0
	RatingBonusMax = 0.3
	MaxBookRating  = 5.0
	PopBonusFactor = 0.1
	PopBonusMax    = 0.3
)

// SimilarCache caches the ranked similar-book ID list per source book.
// Implementations must be fail-fast; the service treats any error as a miss
// (Get) or ignores it (Set).
type SimilarCache interface {
	Get(ctx context.Context, bookID uuid.UUID) (ids []uuid.UUID, ok bool, err error)
	Set(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID, ttl time.Duration) error
}

// similarScore computes the ranking score of one candidate.
func similarScore(srcGenreCount int, authorMatch bool, c repository.SimilarCandidate) float64 {
	score := 0.0
	if authorMatch {
		score += AuthorWeight
	}

	denom := srcGenreCount + c.GenreCount - c.SharedGenres
	if denom > 0 && c.SharedGenres > 0 {
		score += GenreWeight * float64(c.SharedGenres) / float64(denom)
	}

	rating := c.Rating
	if rating < 0 {
		rating = 0
	}
	if rating > MaxBookRating {
		rating = MaxBookRating
	}
	score += RatingBonusMax * rating / MaxBookRating

	if c.RatingsCount > 0 {
		score += math.Min(PopBonusMax, PopBonusFactor*math.Log1p(float64(c.RatingsCount)))
	}
	return score
}

// rankSimilar scores, sorts, applies the per-author cap and truncates the
// candidates to limit IDs. It is pure and deterministic: ties are broken by
// ratings_count DESC, then book ID string ASC. The result is never nil.
func rankSimilar(srcGenreCount int, srcAuthorIDs []uuid.UUID, cands []repository.SimilarCandidate, limit int) []uuid.UUID {
	if limit <= 0 {
		return []uuid.UUID{}
	}
	srcAuthors := make(map[uuid.UUID]struct{}, len(srcAuthorIDs))
	for _, id := range srcAuthorIDs {
		srcAuthors[id] = struct{}{}
	}

	type scored struct {
		cand  repository.SimilarCandidate
		score float64
		id    string
	}
	items := make([]scored, 0, len(cands))
	for _, c := range cands {
		authorMatch := c.SharedAuthors > 0
		if !authorMatch {
			for _, a := range c.AuthorIDs {
				if _, ok := srcAuthors[a]; ok {
					authorMatch = true
					break
				}
			}
		}
		items = append(items, scored{cand: c, score: similarScore(srcGenreCount, authorMatch, c), id: c.BookID.String()})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		if items[i].cand.RatingsCount != items[j].cand.RatingsCount {
			return items[i].cand.RatingsCount > items[j].cand.RatingsCount
		}
		return items[i].id < items[j].id
	})

	result := make([]uuid.UUID, 0, min(limit, len(items)))
	perAuthor := make(map[uuid.UUID]int)
	for _, it := range items {
		blocked := false
		for _, a := range it.cand.AuthorIDs {
			if perAuthor[a] >= perAuthorCap {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		for _, a := range it.cand.AuthorIDs {
			perAuthor[a]++
		}
		result = append(result, it.cand.BookID)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// normalizeSimilarLimit maps <= 0 to DefaultSimilarLimit and clamps to
// MaxSimilarLimit.
func normalizeSimilarLimit(limit int) int {
	if limit <= 0 {
		return DefaultSimilarLimit
	}
	if limit > MaxSimilarLimit {
		return MaxSimilarLimit
	}
	return limit
}

// ListSimilarBooks returns up to limit visible books related to the visible
// source book, ranked by shared authors/genres. Only the ranked ID list is
// cached; visibility and card data are always read fresh.
func (s *Service) ListSimilarBooks(ctx context.Context, bookID uuid.UUID, limit int) ([]domain.Book, error) {
	limit = normalizeSimilarLimit(limit)

	// Checked on every request, cache hit included, so a book hidden after
	// caching returns 404.
	src, err := s.repo.GetSimilarSource(ctx, bookID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("get similar source: %w", err)
	}

	var ids []uuid.UUID
	cached := false
	if s.similarCache != nil {
		cachedIDs, ok, cacheErr := s.similarCache.Get(ctx, bookID)
		if cacheErr == nil && ok {
			ids, cached = cachedIDs, true
		}
	}
	if !cached {
		cands, err := s.repo.ListSimilarCandidates(ctx, bookID, *src, authorPool, genrePool)
		if err != nil {
			return nil, fmt.Errorf("list similar candidates: %w", err)
		}
		ids = rankSimilar(len(src.GenreIDs), src.AuthorIDs, cands, MaxSimilarLimit)
		if s.similarCache != nil {
			// A failed cache write must never fail the request.
			_ = s.similarCache.Set(ctx, bookID, ids, similarCacheTTL)
		}
	}

	ids = ids[:min(limit, len(ids))]
	if len(ids) == 0 {
		return []domain.Book{}, nil
	}

	books, err := s.repo.GetBooksByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get similar books: %w", err)
	}
	byID := make(map[uuid.UUID]domain.Book, len(books))
	for _, b := range books {
		byID[b.ID] = b
	}
	result := make([]domain.Book, 0, len(ids))
	for _, id := range ids {
		// IDs not returned were hidden since caching; drop them.
		if b, ok := byID[id]; ok {
			result = append(result, b)
		}
	}
	return result, nil
}
