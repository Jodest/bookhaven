package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Author moderation queue (docs/features/author-moderation-queue.md).

const (
	authorQueueDefaultLimit = 20
	authorQueueMaxLimit     = 50
	// bulkAuthorStatusCap bounds one BulkSetAuthorStatus call (the
	// selection of one queue page, with headroom).
	bulkAuthorStatusCap = 100
	// maxAuthorHints bounds the similar-name hints per queue row.
	maxAuthorHints = 5
	// minSurnameRunes: shorter surnames produce no hints.
	minSurnameRunes = 3
)

// Similar-name hint match kinds, strongest first.
const (
	MatchExactName      = "exact_name"
	MatchSurnameInitial = "surname_initial"
	MatchSurname        = "surname"
)

var (
	// ErrInvalidModerationFilter is returned by ListAuthorModerationQueue
	// for an unknown status / qid / books filter value (HTTP 400).
	ErrInvalidModerationFilter = errors.New("invalid moderation filter")
	// ErrBulkStatusTooLarge is returned by BulkSetAuthorStatus and
	// BulkSetGenreStatus, without changing anything, for more distinct ids
	// than the cap (bulkAuthorStatusCap / bulkGenreStatusCap, both 100), so
	// the message is entity-neutral.
	ErrBulkStatusTooLarge = fmt.Errorf("too many ids: at most %d per request", bulkAuthorStatusCap)
)

// AuthorHint is a display-only "possibly the same person" suggestion.
type AuthorHint struct {
	Author catalogDomain.Author
	Match  string
}

// AuthorQueueItem is one row of the author moderation queue.
type AuthorQueueItem struct {
	Author       catalogDomain.Author
	BooksTotal   int64
	BooksVisible int64
	SampleBooks  []repository.SampleBook
	Similar      []AuthorHint
}

// BulkStatusReport is the JSON report of POST /admin/authors/bulk-status,
// mirroring BulkPublishReport. Returned with HTTP 200 regardless of
// per-row outcomes.
type BulkStatusReport struct {
	Processed int                  `json:"processed"`
	Updated   int                  `json:"updated"`
	Errors    []BulkStatusRowError `json:"errors"`
}

// BulkStatusRowError describes one author whose status was not changed.
type BulkStatusRowError struct {
	AuthorID string `json:"author_id"`
	Name     string `json:"name"`
	Message  string `json:"message"`
}

// ListAuthorModerationQueue returns one page of the author moderation queue
// (default status pending, newest first) with per-author book context and
// similar-name hints. Hints use at most two extra queries per page (sample
// books, hint candidates) and are recomputed on every call.
func (s *Service) ListAuthorModerationQueue(ctx context.Context, filter repository.AuthorModerationFilter, page, limit int) ([]AuthorQueueItem, int64, error) {
	if filter.Status == "" {
		filter.Status = catalogDomain.AuthorStatusPending
	}
	if !catalogDomain.IsValidAuthorStatus(filter.Status) {
		return nil, 0, fmt.Errorf("%w: status must be pending, published or rejected", ErrInvalidModerationFilter)
	}
	switch filter.QID {
	case "", "with", "without":
	default:
		return nil, 0, fmt.Errorf("%w: qid must be with or without", ErrInvalidModerationFilter)
	}
	switch filter.Books {
	case "", "none":
	default:
		return nil, 0, fmt.Errorf("%w: books must be none", ErrInvalidModerationFilter)
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = authorQueueDefaultLimit
	}
	if limit > authorQueueMaxLimit {
		limit = authorQueueMaxLimit
	}
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	rows, total, err := s.repo.ListAuthorsForModeration(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list authors for moderation: %w", err)
	}
	items := make([]AuthorQueueItem, 0, len(rows))
	if len(rows) == 0 {
		return items, total, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	surnameSet := make(map[string]struct{}, len(rows))
	surnames := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Author.ID)
		if sn := authorSurname(row.Author.Name); sn != "" {
			if _, ok := surnameSet[sn]; !ok {
				surnameSet[sn] = struct{}{}
				surnames = append(surnames, sn)
			}
		}
	}

	samples, err := s.repo.ListAuthorSampleBooks(ctx, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("list author sample books: %w", err)
	}

	// Candidate pool: authors outside the page (one prefilter query) plus
	// the page's own rows, so two pending duplicates on the same page hint
	// at each other. buildAuthorHints re-checks everything.
	var candidates []catalogDomain.Author
	if len(surnames) > 0 {
		found, err := s.repo.FindAuthorsBySurnames(ctx, surnames, ids)
		if err != nil {
			return nil, 0, fmt.Errorf("find hint candidates: %w", err)
		}
		candidates = append(candidates, found...)
		for _, row := range rows {
			candidates = append(candidates, row.Author)
		}
	}

	for _, row := range rows {
		sample := samples[row.Author.ID]
		if sample == nil {
			sample = []repository.SampleBook{}
		}
		items = append(items, AuthorQueueItem{
			Author:       row.Author,
			BooksTotal:   row.BooksTotal,
			BooksVisible: row.BooksVisible,
			SampleBooks:  sample,
			Similar:      buildAuthorHints(row.Author, candidates),
		})
	}
	return items, total, nil
}

// BulkSetAuthorStatus sets status on every author in ids (deduplicated).
// Each id is processed independently: an unknown id or a failed update is
// a row error and the rest proceed. Only status (and updated_at) change.
func (s *Service) BulkSetAuthorStatus(ctx context.Context, ids []uuid.UUID, status string) (BulkStatusReport, error) {
	if !catalogDomain.IsValidAuthorStatus(status) {
		return BulkStatusReport{}, ErrInvalidAuthorStatus
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
	if len(unique) > bulkAuthorStatusCap {
		return BulkStatusReport{}, ErrBulkStatusTooLarge
	}

	report := BulkStatusReport{Processed: len(unique), Errors: []BulkStatusRowError{}}
	for _, id := range unique {
		found, err := s.repo.SetAuthorStatus(ctx, id, status)
		switch {
		case err != nil:
			report.Errors = append(report.Errors, BulkStatusRowError{AuthorID: id.String(), Message: err.Error()})
		case !found:
			report.Errors = append(report.Errors, BulkStatusRowError{AuthorID: id.String(), Message: ErrAuthorNotFound.Error()})
		default:
			report.Updated++
		}
	}
	return report, nil
}

// normalizeAuthorName lowercases name, folds ё to е, turns every rune that
// is not a letter, digit or space into a space, and collapses spaces.
func normalizeAuthorName(name string) string {
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

// authorNameParts splits a name into its normalized surname and the
// remaining (given-name) tokens. "Толстой, Лев" → ("толстой", ["лев"]);
// "Лев Николаевич Толстой" → ("толстой", ["лев", "николаевич"]).
func authorNameParts(name string) (string, []string) {
	if i := strings.Index(name, ","); i >= 0 {
		return normalizeAuthorName(name[:i]), strings.Fields(normalizeAuthorName(name[i+1:]))
	}
	tokens := strings.Fields(normalizeAuthorName(name))
	if len(tokens) == 0 {
		return "", nil
	}
	return tokens[len(tokens)-1], tokens[:len(tokens)-1]
}

// authorSurname returns the normalized surname used for hints, or "" when
// it is shorter than minSurnameRunes.
func authorSurname(name string) string {
	surname, _ := authorNameParts(name)
	if utf8.RuneCountInString(surname) < minSurnameRunes {
		return ""
	}
	return surname
}

// firstInitial returns the first rune of the first non-surname token, or 0.
func firstInitial(name string) rune {
	_, rest := authorNameParts(name)
	if len(rest) == 0 {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(rest[0])
	return r
}

// matchKind classifies how candidate's name matches name, or "" when the
// surnames differ (or are too short).
func matchKind(name, candidate string) string {
	surname := authorSurname(name)
	if surname == "" || surname != authorSurname(candidate) {
		return ""
	}
	if normalizeAuthorName(name) == normalizeAuthorName(candidate) {
		return MatchExactName
	}
	if i := firstInitial(name); i != 0 && i == firstInitial(candidate) {
		return MatchSurnameInitial
	}
	return MatchSurname
}

func matchRank(kind string) int {
	switch kind {
	case MatchExactName:
		return 0
	case MatchSurnameInitial:
		return 1
	default:
		return 2
	}
}

// buildAuthorHints returns at most maxAuthorHints candidates matching
// author (self and rejected excluded), ordered by match kind, then
// published before pending, then name. Never nil.
func buildAuthorHints(author catalogDomain.Author, candidates []catalogDomain.Author) []AuthorHint {
	hints := []AuthorHint{}
	seen := make(map[uuid.UUID]struct{}, len(candidates))
	for _, c := range candidates {
		if c.ID == author.ID || c.Status == catalogDomain.AuthorStatusRejected {
			continue
		}
		if _, dup := seen[c.ID]; dup {
			continue
		}
		kind := matchKind(author.Name, c.Name)
		if kind == "" {
			continue
		}
		seen[c.ID] = struct{}{}
		hints = append(hints, AuthorHint{Author: c, Match: kind})
	}
	sort.SliceStable(hints, func(i, j int) bool {
		a, b := hints[i], hints[j]
		if ra, rb := matchRank(a.Match), matchRank(b.Match); ra != rb {
			return ra < rb
		}
		if pa, pb := a.Author.IsPublished(), b.Author.IsPublished(); pa != pb {
			return pa
		}
		if a.Author.Name != b.Author.Name {
			return a.Author.Name < b.Author.Name
		}
		return a.Author.ID.String() < b.Author.ID.String()
	})
	if len(hints) > maxAuthorHints {
		hints = hints[:maxAuthorHints]
	}
	return hints
}
