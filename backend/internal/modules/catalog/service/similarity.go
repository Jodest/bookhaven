package service

import (
	"bytes"
	"math"
	"sort"

	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
)

// Stage-2 co-reading constants (see docs/features/similar-books-stage2.md).
const (
	// MinCoReaders (K) is the minimum number of distinct readers a pair of
	// books must share to be used anywhere. It is a constant on purpose
	// (privacy floor); raise it to 5 as the user base grows.
	MinCoReaders = 3
	// MaxUserInteractions: users with more interactions (bulk importers,
	// bots) are ignored entirely. Also bounds the O(n²) pair counting.
	MaxUserInteractions = 500
	// CoWeightMax is the co-reading weight once the source book has
	// CoSupportFull or more readers.
	CoWeightMax = 0.6
	// CoSupportFull is the reader count at which the co weight saturates.
	CoSupportFull = 20
	// MaxContentScore is the maximum stage-1 similarScore, used to
	// normalize it to [0, 1].
	MaxContentScore = AuthorWeight + GenreWeight + RatingBonusMax + PopBonusMax
)

// CoPartner is a book co-read with the source by at least MinCoReaders
// readers.
type CoPartner struct {
	BookID uuid.UUID
	Co     int     // co(a,b): distinct shared readers
	Score  float64 // cosine: co / sqrt(n(a)·n(b))
}

type bookPair struct{ a, b uuid.UUID }

func uuidLess(a, b uuid.UUID) bool { return bytes.Compare(a[:], b[:]) < 0 }

// coReading is pure. It deduplicates the interactions, drops users with
// more than maxPerUser or fewer than 2 interactions entirely (they count
// neither in pairs nor in n(x)), counts co-read pairs, keeps pairs with
// co >= minCo and computes the cosine score. It returns the partners per
// book (both directions, sorted by score DESC, co DESC, ID ASC), n(x) per
// book, and the number of users that contributed.
func coReading(inter []repository.Interaction, minCo, maxPerUser int) (partners map[uuid.UUID][]CoPartner, readers map[uuid.UUID]int, usersUsed int) {
	partners = make(map[uuid.UUID][]CoPartner)
	readers = make(map[uuid.UUID]int)

	byUser := make(map[uuid.UUID]map[uuid.UUID]struct{})
	for _, in := range inter {
		books, ok := byUser[in.UserID]
		if !ok {
			books = make(map[uuid.UUID]struct{})
			byUser[in.UserID] = books
		}
		books[in.BookID] = struct{}{}
	}

	pairs := make(map[bookPair]int)
	for _, set := range byUser {
		if len(set) < 2 || len(set) > maxPerUser {
			continue
		}
		usersUsed++
		books := make([]uuid.UUID, 0, len(set))
		for id := range set {
			books = append(books, id)
			readers[id]++
		}
		sort.Slice(books, func(i, j int) bool { return uuidLess(books[i], books[j]) })
		for i := 0; i < len(books); i++ {
			for j := i + 1; j < len(books); j++ {
				pairs[bookPair{books[i], books[j]}]++
			}
		}
	}

	for p, co := range pairs {
		if co < minCo {
			continue
		}
		score := float64(co) / math.Sqrt(float64(readers[p.a])*float64(readers[p.b]))
		partners[p.a] = append(partners[p.a], CoPartner{BookID: p.b, Co: co, Score: score})
		partners[p.b] = append(partners[p.b], CoPartner{BookID: p.a, Co: co, Score: score})
	}
	for _, list := range partners {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Score != list[j].Score {
				return list[i].Score > list[j].Score
			}
			if list[i].Co != list[j].Co {
				return list[i].Co > list[j].Co
			}
			return list[i].BookID.String() < list[j].BookID.String()
		})
	}
	return partners, readers, usersUsed
}

// coWeight is w = CoWeightMax · min(1, n(a) / CoSupportFull).
func coWeight(nSource int) float64 {
	if nSource <= 0 {
		return 0
	}
	return CoWeightMax * math.Min(1, float64(nSource)/CoSupportFull)
}

// blendSimilar is pure. It merges the stage-1 candidates of the source book
// srcID (scored with similarScore and normalized by MaxContentScore) with
// its co-reading partners, computes final = (1−w)·contentNorm + w·coScore,
// sorts with lessRanked, applies selectWithAuthorCap and returns at most maxRows
// rows (positions 0..n−1) with the score breakdown. Co-only partners take
// ratings_count from ratingsCount and author IDs from authorIDs; a missing
// component is 0. The source book never appears in its own list.
func blendSimilar(srcID uuid.UUID, src repository.SimilarSource, cands []repository.SimilarCandidate,
	partners []CoPartner, nSource int, ratingsCount map[uuid.UUID]int,
	authorIDs map[uuid.UUID][]uuid.UUID, maxRows int) []repository.SimilarityRow {
	if maxRows <= 0 {
		return nil
	}

	type entry struct {
		content, co  float64
		coReaders    int
		ratingsCount int
		authorIDs    []uuid.UUID
	}
	srcAuthors := authorSet(src.AuthorIDs)
	entries := make(map[uuid.UUID]*entry, len(cands)+len(partners))
	order := make([]uuid.UUID, 0, len(cands)+len(partners))

	for _, c := range cands {
		if c.BookID == srcID {
			continue
		}
		if _, dup := entries[c.BookID]; dup {
			continue
		}
		entries[c.BookID] = &entry{
			content:      similarScore(len(src.GenreIDs), isAuthorMatch(srcAuthors, c), c) / MaxContentScore,
			ratingsCount: c.RatingsCount,
			authorIDs:    c.AuthorIDs,
		}
		order = append(order, c.BookID)
	}
	for _, p := range partners {
		if p.BookID == srcID {
			continue
		}
		e, ok := entries[p.BookID]
		if !ok {
			e = &entry{ratingsCount: ratingsCount[p.BookID], authorIDs: authorIDs[p.BookID]}
			entries[p.BookID] = e
			order = append(order, p.BookID)
		}
		e.co = p.Score
		e.coReaders = p.Co
	}

	w := coWeight(nSource)
	items := make([]rankedItem, 0, len(order))
	for _, id := range order {
		e := entries[id]
		items = append(items, rankedItem{
			ID:           id,
			Score:        (1-w)*e.content + w*e.co,
			RatingsCount: e.ratingsCount,
			AuthorIDs:    e.authorIDs,
		})
	}
	sortRanked(items)
	selected := selectWithAuthorCap(items, maxRows)

	rows := make([]repository.SimilarityRow, 0, len(selected))
	for i, it := range selected {
		e := entries[it.ID]
		rows = append(rows, repository.SimilarityRow{
			BookID:        srcID,
			SimilarBookID: it.ID,
			Position:      i,
			Score:         it.Score,
			ContentScore:  e.content,
			CoScore:       e.co,
			CoReaders:     e.coReaders,
		})
	}
	return rows
}
