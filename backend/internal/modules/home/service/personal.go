package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrAllPersonalBlocksFailed is returned when every block of the personal
// home payload failed.
var ErrAllPersonalBlocksFailed = errors.New("all personal home blocks failed")

// System shelf types read by the personal blocks.
const (
	shelfTypeReading    = "reading"
	shelfTypeWantToRead = "want_to_read"
)

// ContinueReading is the caller's "reading" shelf preview.
type ContinueReading struct {
	ShelfID uuid.UUID
	Total   int // visible books on the shelf
	Books   []catalogdomain.Book
}

// WantToReadPick is one daily book from the caller's "want_to_read" shelf.
type WantToReadPick struct {
	ShelfID uuid.UUID
	Total   int                 // visible books on the shelf
	Book    *catalogdomain.Book // nil when the shelf has no visible books
}

// PersonalHome is the caller's personal home payload. ContinueReading and
// WantToReadPick are nil when the system shelf is not materialized yet;
// Recommendations is never nil.
type PersonalHome struct {
	ContinueReading *ContinueReading
	Recommendations []catalogdomain.Book
	WantToReadPick  *WantToReadPick
}

// personalBlockCount is the number of independently computed personal
// blocks.
const personalBlockCount = 3

// personalSelection is the ID-level personal payload before hydration.
type personalSelection struct {
	reading         *ContinueReading // Books filled at hydration
	readingIDs      []uuid.UUID
	recommendations []uuid.UUID
	wantToRead      *WantToReadPick // Book filled at hydration
	wantToReadID    *uuid.UUID
}

// GetPersonalHome returns the personal blocks of userID (always taken from
// the token by the caller). It is never cached. A failed block is logged
// and comes back empty; when every block fails it returns
// ErrAllPersonalBlocksFailed.
func (s *Service) GetPersonalHome(ctx context.Context, userID uuid.UUID) (*PersonalHome, error) {
	today := civilDate(s.now())
	sel := &personalSelection{recommendations: []uuid.UUID{}}
	failed := 0
	fail := func(block string, err error) {
		failed++
		s.logger.Error("home personal block failed", zap.String("block", block), zap.Error(err))
	}

	if ref, ids, err := s.selectContinueReading(ctx, userID); err != nil {
		fail("continue_reading", err)
	} else {
		sel.reading, sel.readingIDs = ref, ids
	}

	if ids, err := s.selectRecommendations(ctx, userID); err != nil {
		fail("recommendations", err)
	} else {
		sel.recommendations = ids
	}

	if pick, id, err := s.selectWantToReadPick(ctx, userID, today); err != nil {
		fail("want_to_read_pick", err)
	} else {
		sel.wantToRead, sel.wantToReadID = pick, id
	}

	if failed == personalBlockCount {
		return nil, ErrAllPersonalBlocksFailed
	}
	return s.hydratePersonal(ctx, sel)
}

func (s *Service) selectContinueReading(ctx context.Context, userID uuid.UUID) (*ContinueReading, []uuid.UUID, error) {
	ref, err := s.repo.GetSystemShelf(ctx, userID, shelfTypeReading)
	if err != nil {
		return nil, nil, fmt.Errorf("get reading shelf: %w", err)
	}
	if ref == nil {
		return nil, nil, nil // not materialized yet; never created here
	}
	ids, err := s.repo.ListShelfBookIDs(ctx, ref.ID, ContinueReadingLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("list reading shelf books: %w", err)
	}
	return &ContinueReading{ShelfID: ref.ID, Total: ref.Total, Books: []catalogdomain.Book{}}, ids, nil
}

// selectRecommendations aggregates precomputed similarities over the
// caller's seeds (tier 1) and tops up from live similar lists of the most
// recent seeds (tier 2) when tier 1 is short.
func (s *Service) selectRecommendations(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	seeds, err := s.repo.ListRecommendationSeeds(ctx, userID, RecommendationSeeds)
	if err != nil {
		return nil, fmt.Errorf("list recommendation seeds: %w", err)
	}
	if len(seeds) == 0 {
		return []uuid.UUID{}, nil
	}

	exclude, err := s.repo.ListUserExcludedBookIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list excluded books: %w", err)
	}
	if exclude == nil {
		exclude = make(map[uuid.UUID]struct{}, len(seeds))
	}
	for _, id := range seeds {
		exclude[id] = struct{}{}
	}

	scored, err := s.repo.AggregatePrecomputedSimilar(ctx, userID, seeds, RecommendationsLimit)
	if err != nil {
		return nil, fmt.Errorf("aggregate precomputed similar: %w", err)
	}
	tier1 := make([]uuid.UUID, 0, len(scored))
	for _, row := range scored {
		tier1 = append(tier1, row.ID)
	}

	var perSeed [][]uuid.UUID
	if len(tier1) < RecommendationsTopUpBelow && s.similar != nil {
		topUp := seeds
		if len(topUp) > RecommendationsTopUpSeeds {
			topUp = topUp[:RecommendationsTopUpSeeds]
		}
		for _, seed := range topUp {
			books, err := s.similar.ListSimilarBooks(ctx, seed, RecommendationsTopUpPerSeed)
			if err != nil {
				// A seed hidden mid-request is expected; any other failure
				// only costs the top-up for that seed, tier 1 is kept.
				if !errors.Is(err, catalogservice.ErrBookNotFound) {
					s.logger.Warn("home recommendations top-up failed", zap.String("seed", seed.String()), zap.Error(err))
				}
				continue
			}
			list := make([]uuid.UUID, 0, len(books))
			for _, b := range books {
				list = append(list, b.ID)
			}
			perSeed = append(perSeed, list)
		}
	}

	return mergeRecommendations(tier1, perSeed, exclude, RecommendationsLimit), nil
}

func (s *Service) selectWantToReadPick(ctx context.Context, userID uuid.UUID, today time.Time) (*WantToReadPick, *uuid.UUID, error) {
	ref, err := s.repo.GetSystemShelf(ctx, userID, shelfTypeWantToRead)
	if err != nil {
		return nil, nil, fmt.Errorf("get want_to_read shelf: %w", err)
	}
	if ref == nil {
		return nil, nil, nil
	}
	ids, err := s.repo.ListShelfBookIDs(ctx, ref.ID, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("list want_to_read shelf books: %w", err)
	}
	pick := &WantToReadPick{ShelfID: ref.ID, Total: ref.Total}
	id, ok := pickDaily(ids, today, wantToReadSaltPrefix+userID.String())
	if !ok {
		return pick, nil, nil
	}
	return pick, &id, nil
}

// hydratePersonal loads every personal book in one query and drops books
// hidden since the selection.
func (s *Service) hydratePersonal(ctx context.Context, sel *personalSelection) (*PersonalHome, error) {
	all := make([]uuid.UUID, 0, len(sel.readingIDs)+len(sel.recommendations)+1)
	seen := make(map[uuid.UUID]struct{})
	add := func(id uuid.UUID) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			all = append(all, id)
		}
	}
	for _, id := range sel.readingIDs {
		add(id)
	}
	for _, id := range sel.recommendations {
		add(id)
	}
	if sel.wantToReadID != nil {
		add(*sel.wantToReadID)
	}

	byID := make(map[uuid.UUID]catalogdomain.Book, len(all))
	if len(all) > 0 {
		books, err := s.books.GetBooksByIDs(ctx, all)
		if err != nil {
			return nil, fmt.Errorf("hydrate personal home books: %w", err)
		}
		for _, b := range books {
			byID[b.ID] = b
		}
	}
	pick := func(list []uuid.UUID) []catalogdomain.Book {
		result := make([]catalogdomain.Book, 0, len(list))
		for _, id := range list {
			if b, ok := byID[id]; ok {
				result = append(result, b)
			}
		}
		return result
	}

	home := &PersonalHome{Recommendations: pick(sel.recommendations)}
	if sel.reading != nil {
		sel.reading.Books = pick(sel.readingIDs)
		home.ContinueReading = sel.reading
	}
	if sel.wantToRead != nil {
		if sel.wantToReadID != nil {
			if b, ok := byID[*sel.wantToReadID]; ok {
				sel.wantToRead.Book = &b
			}
		}
		home.WantToReadPick = sel.wantToRead
	}
	return home, nil
}
