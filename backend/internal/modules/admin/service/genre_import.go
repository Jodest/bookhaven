package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Genre import from Wikidata (docs/features/genre-wikidata-import.md).

var (
	// ErrNotALiteraryGenre: the entity is not a literary genre (422,
	// not_a_literary_genre).
	ErrNotALiteraryGenre = errors.New("wikidata entity is not a literary genre")
	// ErrGenreImportUnavailable: no GenreWikidataSource was configured
	// (503).
	ErrGenreImportUnavailable = errors.New("genre import source is not configured")
)

// GenreWikidataSource is the Wikidata access used by the genre search and
// import. Satisfied by *bookdata.WikidataClient.
type GenreWikidataSource interface {
	SearchGenres(ctx context.Context, query string) ([]bookdata.GenreCandidate, error)
	ClassifyGenres(ctx context.Context, qids []string) (map[string]bookdata.GenreClassification, error)
	FetchGenre(ctx context.Context, qid string) (*bookdata.ExternalGenreRecord, error)
}

const (
	// defaultGenreClassifyTimeout bounds every classification call so
	// doRequest retries cannot stretch a WDQS timeout into minutes.
	defaultGenreClassifyTimeout = 8 * time.Second
	// genreSearchMaxResults is the number of candidates shown after
	// filtering (search order kept).
	genreSearchMaxResults = 10
)

// Genre search class_check values.
const (
	GenreClassCheckOK          = "ok"
	GenreClassCheckUnavailable = "unavailable"
)

// GenreSearchResult is the result of SearchWikidataGenres.
type GenreSearchResult struct {
	// ClassCheck is GenreClassCheckOK, or GenreClassCheckUnavailable when
	// the classification failed and Items are unfiltered.
	ClassCheck string
	Items      []GenreSearchItem
}

// GenreSearchItem is one Wikidata candidate of a genre search.
type GenreSearchItem struct {
	QID         string
	Label       string
	Description string
	// RuLabel is the ru label from the classification (nil when absent or
	// when the classification was unavailable).
	RuLabel *string
	// LiteraryGenre is nil when the classification was unavailable
	// (otherwise always true: non-genres are dropped).
	LiteraryGenre *bool
	// Existing is the BookHaven genre carrying the QID, nil when none (or
	// when the lookup failed).
	Existing *GenreExistingMarker
}

// GenreExistingMarker is the "already in BookHaven" marker of a search
// candidate: the canonical genre and, when the QID belongs to a merged
// alias, that alias.
type GenreExistingMarker struct {
	Genre    *catalogDomain.Genre
	ViaAlias *catalogDomain.Genre
}

// GenreImportResult is the outcome of ImportWikidataGenre.
type GenreImportResult struct {
	// Genre is the canonical (matched or created) genre.
	Genre *catalogDomain.Genre
	// Outcome is GenreOutcomeCreated, GenreOutcomeExisting or
	// GenreOutcomeRejected.
	Outcome string
	// Match is GenreMatchQID / GenreMatchSlug, "" for created.
	Match         string
	QIDBackfilled bool
	// QIDMismatch is the QID of a slug-matched genre that carries a
	// different QID (the genre is left untouched), nil otherwise.
	QIDMismatch *string
	// ViaAlias is the merged alias the match was followed through.
	ViaAlias *catalogDomain.Genre
}

func (s *Service) classifyTimeout() time.Duration {
	if s.genreClassifyTimeout > 0 {
		return s.genreClassifyTimeout
	}
	return defaultGenreClassifyTimeout
}

// classifyGenres runs one bounded classification call.
func (s *Service) classifyGenres(ctx context.Context, qids []string) (map[string]bookdata.GenreClassification, error) {
	cctx, cancel := context.WithTimeout(ctx, s.classifyTimeout())
	defer cancel()
	return s.genreSource.ClassifyGenres(cctx, qids)
}

// SearchWikidataGenres searches Wikidata for literary genres by name (rule
// 2): one wbsearchentities call (20 candidates), one classification query
// that drops non-genres and supplies ru labels, the first 10 kept, and
// "already in BookHaven" markers from one QID query (aliases followed to
// their canonical genre). A failed classification degrades to the
// unfiltered candidates with ClassCheck "unavailable"; a failed marker
// lookup degrades to nil markers. Search failures are returned as is
// (bookdata errors).
func (s *Service) SearchWikidataGenres(ctx context.Context, query string) (*GenreSearchResult, error) {
	if s.genreSource == nil {
		return nil, ErrGenreImportUnavailable
	}
	candidates, err := s.genreSource.SearchGenres(ctx, strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	result := &GenreSearchResult{ClassCheck: GenreClassCheckOK, Items: []GenreSearchItem{}}
	if len(candidates) == 0 {
		return result, nil
	}

	qids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		qids = append(qids, c.QID)
	}
	if len(qids) > bookdata.MaxGenresPerClassification {
		qids = qids[:bookdata.MaxGenresPerClassification]
		candidates = candidates[:bookdata.MaxGenresPerClassification]
	}

	classes, err := s.classifyGenres(ctx, qids)
	if err != nil {
		s.log.Warn("wikidata genre classification failed; returning unfiltered candidates",
			zap.String("query", query), zap.Error(err))
		result.ClassCheck = GenreClassCheckUnavailable
	}

	for _, c := range candidates {
		if len(result.Items) == genreSearchMaxResults {
			break
		}
		item := GenreSearchItem{QID: c.QID, Label: c.Label, Description: c.Description}
		if result.ClassCheck == GenreClassCheckOK {
			cls := classes[c.QID]
			if !cls.IsLiteraryGenre {
				continue
			}
			literary := true
			item.LiteraryGenre = &literary
			if label := strings.TrimSpace(cls.RuLabel); label != "" {
				item.RuLabel = &label
			}
		}
		result.Items = append(result.Items, item)
	}

	s.attachGenreMarkers(ctx, result.Items)
	return result, nil
}

// attachGenreMarkers sets Existing on items whose QID is in BookHaven. A
// lookup failure or a broken alias is logged and leaves the marker nil.
func (s *Service) attachGenreMarkers(ctx context.Context, items []GenreSearchItem) {
	if len(items) == 0 {
		return
	}
	qids := make([]string, 0, len(items))
	for _, it := range items {
		qids = append(qids, it.QID)
	}
	genres, err := s.repo.ListGenresByQIDs(ctx, qids)
	if err != nil {
		s.log.Error("wikidata genre search: existing genre lookup failed", zap.Error(err))
		return
	}
	markers := make(map[string]*GenreExistingMarker, len(genres))
	for i := range genres {
		g := &genres[i]
		if g.WikidataQID == nil {
			continue
		}
		target, alias, err := s.followGenreAlias(ctx, g)
		if err != nil {
			s.log.Error("wikidata genre search: existing genre marker skipped",
				zap.String("qid", *g.WikidataQID), zap.Error(err))
			continue
		}
		markers[*g.WikidataQID] = &GenreExistingMarker{Genre: target, ViaAlias: alias}
	}
	for i := range items {
		items[i].Existing = markers[items[i].QID]
	}
}

// ImportWikidataGenre imports one literary genre by Wikidata QID (rule 3):
//
//  1. normalize the QID (trim, uppercase; ErrInvalidQID otherwise);
//  2. DB first by QID, aliases followed (no Wikidata call when found);
//  3. fetch the entity (ErrWikidataEntityNotFound / bookdata errors);
//  4. on a redirect, re-check the DB with the canonical QID;
//  5. name = ru label (ErrNoRussianLabel when absent or not Russian);
//  6. literary-genre check: direct P31/P279 against the roots, else one
//     classification query (ErrNotALiteraryGenre; a failed call is a
//     bookdata.ErrSourceUnavailable-wrapped error, never skipped);
//  7. resolve with resolveGenreBySlug (harvest rules);
//  8. on a duplicate create, re-run from step 2 once.
//
// It never changes an existing genre's name, slug or status; the only
// possible write to an existing row is the slug-match QID backfill.
func (s *Service) ImportWikidataGenre(ctx context.Context, qid string) (*GenreImportResult, error) {
	qid = strings.ToUpper(strings.TrimSpace(qid))
	if !importQIDPattern.MatchString(qid) {
		return nil, ErrInvalidQID
	}

	res, err := s.resolveGenreByQID(ctx, qid)
	if err != nil {
		return nil, err
	}
	if res != nil {
		return s.genreImportResult(qid, qid, res), nil
	}

	if s.genreSource == nil {
		return nil, ErrGenreImportUnavailable
	}
	record, err := s.genreSource.FetchGenre(ctx, qid)
	if err != nil {
		switch {
		case errors.Is(err, bookdata.ErrNotFound):
			return nil, ErrWikidataEntityNotFound
		case errors.Is(err, bookdata.ErrInvalidQuery):
			return nil, ErrInvalidQID
		}
		return nil, err
	}

	canonical := strings.ToUpper(strings.TrimSpace(record.QID))
	if !importQIDPattern.MatchString(canonical) {
		canonical = qid
	}
	if canonical != qid {
		res, err := s.resolveGenreByQID(ctx, canonical)
		if err != nil {
			return nil, err
		}
		if res != nil {
			return s.genreImportResult(qid, canonical, res), nil
		}
	}

	name := strings.TrimSpace(record.Name)
	if name == "" || !IsRussianText(name) {
		return nil, ErrNoRussianLabel
	}

	if !isLiteraryGenreLocally(record) {
		classes, err := s.classifyGenres(ctx, []string{canonical})
		if err != nil {
			if errors.Is(err, bookdata.ErrRateLimited) || errors.Is(err, bookdata.ErrSourceUnavailable) {
				return nil, fmt.Errorf("classify genre %s: %w", canonical, err)
			}
			return nil, fmt.Errorf("classify genre %s: %w: %v", canonical, bookdata.ErrSourceUnavailable, err)
		}
		if !classes[canonical].IsLiteraryGenre {
			return nil, ErrNotALiteraryGenre
		}
	}

	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			res, err := s.resolveGenreByQID(ctx, canonical)
			if err != nil {
				return nil, err
			}
			if res != nil {
				return s.genreImportResult(qid, canonical, res), nil
			}
		}
		res, err := s.resolveGenreBySlug(ctx, name, canonical)
		if err == nil {
			return s.genreImportResult(qid, canonical, res), nil
		}
		duplicate := errors.Is(err, ErrDuplicateSlug) || errors.Is(err, ErrDuplicateGenreQID)
		if !duplicate || attempt >= 1 {
			return nil, err
		}
	}
}

// resolveGenreByQID is the QID step of the harvest genre rules: the genre
// carrying qid (alias rows included), followed to its canonical genre. Nil
// when no genre has the QID.
func (s *Service) resolveGenreByQID(ctx context.Context, qid string) (*GenreResolution, error) {
	genre, err := s.repo.GetGenreByQID(ctx, qid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup genre by qid %q: %w", qid, err)
	}
	target, alias, err := s.followGenreAlias(ctx, genre)
	if err != nil {
		return nil, err
	}
	return &GenreResolution{Genre: target, Outcome: genreMatchOutcome(target), Match: GenreMatchQID, ViaAlias: alias}, nil
}

// isLiteraryGenreLocally reports whether the entity's direct P31 or P279
// holds a literary-genre root (no SPARQL needed).
func isLiteraryGenreLocally(record *bookdata.ExternalGenreRecord) bool {
	for _, root := range bookdata.LiteraryGenreRoots {
		for _, c := range record.InstanceOf {
			if c == root {
				return true
			}
		}
		for _, c := range record.SubclassOf {
			if c == root {
				return true
			}
		}
	}
	return false
}

// genreImportResult logs the import and maps a resolution to the result.
// canonical is the QID the import resolved with (after a redirect).
func (s *Service) genreImportResult(qid, canonical string, res *GenreResolution) *GenreImportResult {
	result := &GenreImportResult{
		Genre:         res.Genre,
		Outcome:       res.Outcome,
		Match:         res.Match,
		QIDBackfilled: res.QIDBackfilled,
		ViaAlias:      res.ViaAlias,
	}
	if res.Match == GenreMatchSlug && res.ViaAlias == nil && res.Genre.WikidataQID != nil && *res.Genre.WikidataQID != canonical {
		mismatch := *res.Genre.WikidataQID
		result.QIDMismatch = &mismatch
	}

	viaAliasID := ""
	if res.ViaAlias != nil {
		viaAliasID = res.ViaAlias.ID.String()
	}
	s.log.Info("wikidata genre import",
		zap.String("qid", qid),
		zap.String("outcome", res.Outcome),
		zap.String("match", res.Match),
		zap.String("via_alias_id", viaAliasID),
		zap.String("genre_id", res.Genre.ID.String()))
	return result
}
