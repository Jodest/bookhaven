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
	// similarCacheEmptyTTL is short so an empty list (typically a book whose
	// neighbours are not published yet) heals quickly.
	similarCacheEmptyTTL = 5 * time.Minute
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
	// Generation returns the current invalidation generation. The service
	// reads it once per request, before Get and the live compute, and passes
	// it to Get and Set so that a list computed from pre-invalidation data
	// can only land under the old (unreachable) generation.
	Generation(ctx context.Context) (int64, error)
	Get(ctx context.Context, gen int64, bookID uuid.UUID) (ids []uuid.UUID, ok bool, err error)
	Set(ctx context.Context, gen int64, bookID uuid.UUID, ids []uuid.UUID, ttl time.Duration) error
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

// rankedItem is one scored candidate ready for ordering and the per-author
// cap. It is shared by the stage-1 live ranking and the stage-2 blend.
type rankedItem struct {
	ID           uuid.UUID
	Score        float64
	RatingsCount int
	AuthorIDs    []uuid.UUID
}

// lessRanked is the shared ranking order: score DESC, then ratings_count
// DESC, then book ID string ASC (deterministic).
func lessRanked(a, b rankedItem) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.RatingsCount != b.RatingsCount {
		return a.RatingsCount > b.RatingsCount
	}
	return a.ID.String() < b.ID.String()
}

// sortRanked sorts items in place with lessRanked.
func sortRanked(items []rankedItem) {
	sort.SliceStable(items, func(i, j int) bool { return lessRanked(items[i], items[j]) })
}

// selectWithAuthorCap greedily walks items (already sorted with lessRanked)
// and keeps an item only while none of its authors already has perAuthorCap
// kept books. It stops at limit items. The result is never nil.
func selectWithAuthorCap(items []rankedItem, limit int) []rankedItem {
	if limit <= 0 {
		return []rankedItem{}
	}
	result := make([]rankedItem, 0, min(limit, len(items)))
	perAuthor := make(map[uuid.UUID]int)
	for _, it := range items {
		blocked := false
		for _, a := range it.AuthorIDs {
			if perAuthor[a] >= perAuthorCap {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		for _, a := range it.AuthorIDs {
			perAuthor[a]++
		}
		result = append(result, it)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// authorSet builds a lookup set of the source book's authors.
func authorSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	set := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// isAuthorMatch reports whether the candidate shares an author with the
// source (from the SQL count or the candidate's author IDs).
func isAuthorMatch(srcAuthors map[uuid.UUID]struct{}, c repository.SimilarCandidate) bool {
	if c.SharedAuthors > 0 {
		return true
	}
	for _, a := range c.AuthorIDs {
		if _, ok := srcAuthors[a]; ok {
			return true
		}
	}
	return false
}

// rankSimilar scores, sorts, applies the per-author cap and truncates the
// candidates to limit IDs. It is pure and deterministic: ties are broken by
// ratings_count DESC, then book ID string ASC. The result is never nil.
func rankSimilar(srcGenreCount int, srcAuthorIDs []uuid.UUID, cands []repository.SimilarCandidate, limit int) []uuid.UUID {
	if limit <= 0 {
		return []uuid.UUID{}
	}
	srcAuthors := authorSet(srcAuthorIDs)

	items := make([]rankedItem, 0, len(cands))
	for _, c := range cands {
		items = append(items, rankedItem{
			ID:           c.BookID,
			Score:        similarScore(srcGenreCount, isAuthorMatch(srcAuthors, c), c),
			RatingsCount: c.RatingsCount,
			AuthorIDs:    c.AuthorIDs,
		})
	}
	sortRanked(items)

	selected := selectWithAuthorCap(items, limit)
	result := make([]uuid.UUID, 0, len(selected))
	for _, it := range selected {
		result = append(result, it.ID)
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
// source book. The precomputed stage-2 list (book_similarities) is used when
// it has rows for the book; otherwise the stage-1 ranking by shared
// authors/genres (Redis-cached ID list, live compute on miss). Visibility and
// card data are always read fresh.
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
	// Stage 2: the precomputed (co-reading blend) list wins when it exists.
	// It is checked before Redis so a warm stage-1 entry never hides it, and
	// a read error is a 500, not a silent fallback.
	if s.precomputed != nil {
		pre, err := s.precomputed.ListPrecomputedSimilar(ctx, bookID, MaxSimilarLimit)
		if err != nil {
			return nil, fmt.Errorf("list precomputed similar: %w", err)
		}
		ids = pre
	}
	cached := len(ids) > 0
	var gen int64
	genOK := false
	if !cached && s.similarCache != nil {
		// A failed generation read disables both read and write for this
		// request (the list is just computed live).
		if g, genErr := s.similarCache.Generation(ctx); genErr == nil {
			gen, genOK = g, true
			cachedIDs, ok, cacheErr := s.similarCache.Get(ctx, gen, bookID)
			if cacheErr == nil && ok {
				ids, cached = cachedIDs, true
			}
		}
	}
	if !cached {
		cands, err := s.repo.ListSimilarCandidates(ctx, bookID, *src, authorPool, genrePool)
		if err != nil {
			return nil, fmt.Errorf("list similar candidates: %w", err)
		}
		ids = rankSimilar(len(src.GenreIDs), src.AuthorIDs, cands, MaxSimilarLimit)
		if s.similarCache != nil && genOK {
			// A failed cache write must never fail the request.
			ttl := similarCacheTTL
			if len(ids) == 0 {
				ttl = similarCacheEmptyTTL
			}
			_ = s.similarCache.Set(ctx, gen, bookID, ids, ttl)
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
