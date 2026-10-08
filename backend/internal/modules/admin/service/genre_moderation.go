package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Genre moderation queue (docs/features/genre-moderation-queue.md).

const (
	genreQueueDefaultLimit = 20
	genreQueueMaxLimit     = 50
	// bulkGenreStatusCap bounds one BulkSetGenreStatus call.
	bulkGenreStatusCap = 100
	// maxGenreHints bounds the similar-name hints per queue row.
	maxGenreHints = 5
	// minGenreStemRunes is the shortest stem genreStem leaves behind.
	minGenreStemRunes = 4
	// minSharedWordRunes is the shortest token stem a shared_word hint
	// may be based on.
	minSharedWordRunes = 5
)

// Genre similar-name hint match kinds, strongest first.
const (
	GenreMatchExactName  = "exact_name"
	GenreMatchSameStem   = "same_stem"
	GenreMatchSharedWord = "shared_word"
)

// genreSuffixes is the fixed suffix list genreStem strips (rule 7).
var genreSuffixes = []string{
	"ами", "ями", "ого", "его", "ому", "ему", "ыми", "ими",
	"ая", "яя", "ое", "ее", "ые", "ие", "ый", "ий", "ой", "ов", "ев", "ей", "ам", "ям", "ах", "ях",
	"ы", "и", "а", "я", "е", "о", "у", "ю",
}

// GenreHint is a display-only "possibly the same genre" suggestion.
type GenreHint struct {
	Genre catalogDomain.Genre
	Match string
}

// GenreQueueItem is one row of the genre moderation queue.
type GenreQueueItem struct {
	Genre        catalogDomain.Genre
	BooksTotal   int64
	BooksVisible int64
	SampleBooks  []repository.SampleBook
	Similar      []GenreHint
}

// GenreBulkStatusReport is the JSON report of POST
// /admin/genres/bulk-status. Returned with HTTP 200 regardless of per-row
// outcomes.
type GenreBulkStatusReport struct {
	Processed int                       `json:"processed"`
	Updated   int                       `json:"updated"`
	Errors    []GenreBulkStatusRowError `json:"errors"`
}

// GenreBulkStatusRowError describes one genre whose status was not changed.
type GenreBulkStatusRowError struct {
	GenreID string `json:"genre_id"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ListGenreModerationQueue returns one page of the genre moderation queue
// (default status pending, newest first, aliases never listed) with
// per-genre book context and similar-name hints. Sample books and hint
// candidates take one query each per page; hints are recomputed on every
// call and never persisted.
func (s *Service) ListGenreModerationQueue(ctx context.Context, filter repository.GenreModerationFilter, page, limit int) ([]GenreQueueItem, int64, error) {
	if filter.Status == "" {
		filter.Status = catalogDomain.GenreStatusPending
	}
	if !catalogDomain.IsValidGenreStatus(filter.Status) {
		return nil, 0, fmt.Errorf("%w: status must be pending, published or rejected", ErrInvalidModerationFilter)
	}
	switch filter.QID {
	case "", "with", "without":
	default:
		return nil, 0, fmt.Errorf("%w: qid must be with or without", ErrInvalidModerationFilter)
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = genreQueueDefaultLimit
	}
	if limit > genreQueueMaxLimit {
		limit = genreQueueMaxLimit
	}
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	rows, total, err := s.repo.ListGenresForModeration(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list genres for moderation: %w", err)
	}
	items := make([]GenreQueueItem, 0, len(rows))
	if len(rows) == 0 {
		return items, total, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Genre.ID)
	}

	samples, err := s.repo.ListGenreSampleBooks(ctx, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("list genre sample books: %w", err)
	}

	// Candidate pool: every non-rejected, non-alias genre (one query; the
	// vocabulary is small) plus the page's own rows, so two pending
	// duplicates on the same page hint at each other.
	candidates, err := s.repo.ListGenreHintCandidates(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list genre hint candidates: %w", err)
	}
	for _, row := range rows {
		candidates = append(candidates, row.Genre)
	}

	for _, row := range rows {
		sample := samples[row.Genre.ID]
		if sample == nil {
			sample = []repository.SampleBook{}
		}
		items = append(items, GenreQueueItem{
			Genre:        row.Genre,
			BooksTotal:   row.BooksTotal,
			BooksVisible: row.BooksVisible,
			SampleBooks:  sample,
			Similar:      buildGenreHints(row.Genre, candidates),
		})
	}
	return items, total, nil
}

// BulkSetGenreStatus sets status on every genre in ids (deduplicated, at
// most bulkGenreStatusCap). Each id is processed independently: an unknown
// or alias id ("genre not found") or a failed update is a row error and
// the rest proceed. Only status changes; book_genres links are kept.
func (s *Service) BulkSetGenreStatus(ctx context.Context, ids []uuid.UUID, status string) (GenreBulkStatusReport, error) {
	if !catalogDomain.IsValidGenreStatus(status) {
		return GenreBulkStatusReport{}, ErrInvalidGenreStatus
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) > bulkGenreStatusCap {
		return GenreBulkStatusReport{}, ErrBulkStatusTooLarge
	}

	report := GenreBulkStatusReport{Processed: len(unique), Errors: []GenreBulkStatusRowError{}}
	for _, id := range unique {
		found, err := s.repo.SetGenreStatus(ctx, id, status)
		switch {
		case err != nil:
			report.Errors = append(report.Errors, GenreBulkStatusRowError{GenreID: id.String(), Message: err.Error()})
		case !found:
			report.Errors = append(report.Errors, GenreBulkStatusRowError{GenreID: id.String(), Message: ErrGenreNotFound.Error()})
		default:
			report.Updated++
		}
	}
	return report, nil
}

// normalizeGenreName lowercases name, folds ё to е, turns every rune that
// is not a letter or digit into a space, collapses spaces and trims.
func normalizeGenreName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// genreStem strips the longest matching suffix of genreSuffixes from a
// normalized token, but only when at least minGenreStemRunes runes remain;
// otherwise the token is returned unchanged.
func genreStem(token string) string {
	longest := ""
	for _, suf := range genreSuffixes {
		if strings.HasSuffix(token, suf) && len(suf) > len(longest) {
			longest = suf
		}
	}
	if longest == "" {
		return token
	}
	stem := strings.TrimSuffix(token, longest)
	if utf8.RuneCountInString(stem) < minGenreStemRunes {
		return token
	}
	return stem
}

// genreStems returns the stems of the normalized name's tokens, in order.
func genreStems(name string) []string {
	tokens := strings.Fields(normalizeGenreName(name))
	stems := make([]string, 0, len(tokens))
	for _, t := range tokens {
		stems = append(stems, genreStem(t))
	}
	return stems
}

// genreMatchKind classifies how candidate's name matches name, or "" when
// they do not match.
func genreMatchKind(name, candidate string) string {
	a, b := normalizeGenreName(name), normalizeGenreName(candidate)
	if a == "" || b == "" {
		return ""
	}
	if a == b {
		return GenreMatchExactName
	}
	sa, sb := genreStems(name), genreStems(candidate)
	if strings.Join(sa, " ") == strings.Join(sb, " ") {
		return GenreMatchSameStem
	}
	words := make(map[string]struct{}, len(sa))
	for _, s := range sa {
		if utf8.RuneCountInString(s) >= minSharedWordRunes {
			words[s] = struct{}{}
		}
	}
	for _, s := range sb {
		if _, ok := words[s]; ok {
			return GenreMatchSharedWord
		}
	}
	return ""
}

func genreMatchRank(kind string) int {
	switch kind {
	case GenreMatchExactName:
		return 0
	case GenreMatchSameStem:
		return 1
	default:
		return 2
	}
}

// buildGenreHints returns at most maxGenreHints candidates matching genre
// (self, rejected and alias rows excluded), ordered by match kind, then
// published first, then name, then id. Never nil.
func buildGenreHints(genre catalogDomain.Genre, candidates []catalogDomain.Genre) []GenreHint {
	hints := []GenreHint{}
	seen := make(map[uuid.UUID]struct{}, len(candidates))
	for _, c := range candidates {
		if c.ID == genre.ID || c.Status == catalogDomain.GenreStatusRejected || c.IsAlias() {
			continue
		}
		if _, dup := seen[c.ID]; dup {
			continue
		}
		kind := genreMatchKind(genre.Name, c.Name)
		if kind == "" {
			continue
		}
		seen[c.ID] = struct{}{}
		hints = append(hints, GenreHint{Genre: c, Match: kind})
	}
	sort.SliceStable(hints, func(i, j int) bool {
		a, b := hints[i], hints[j]
		if ra, rb := genreMatchRank(a.Match), genreMatchRank(b.Match); ra != rb {
			return ra < rb
		}
		if pa, pb := a.Genre.IsPublished(), b.Genre.IsPublished(); pa != pb {
			return pa
		}
		if a.Genre.Name != b.Genre.Name {
			return a.Genre.Name < b.Genre.Name
		}
		return a.Genre.ID.String() < b.Genre.ID.String()
	})
	if len(hints) > maxGenreHints {
		hints = hints[:maxGenreHints]
	}
	return hints
}
