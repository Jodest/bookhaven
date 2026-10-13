package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	aimodDomain "github.com/bookhaven/backend/internal/modules/aimoderation/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// AI pre-moderation verdicts (openspec capability
// catalog-moderation/ai-verdicts). The admin module owns the verdict table
// and the queue join; the aimoderation module computes verdicts through
// the methods below and never touches book fields.

// AIFilterNone is the ai queue filter value for books without a verdict.
const AIFilterNone = "none"

// reviewDescriptionMaxRunes bounds the description sent to the model.
const reviewDescriptionMaxRunes = 1500

// ErrInvalidAIVerdict is returned by SaveBookAIVerdict for a verdict that
// does not match the stored format (a programming error in the caller).
var ErrInvalidAIVerdict = errors.New("invalid AI verdict")

// BookReviewAuthor is an author as the model sees it.
type BookReviewAuthor struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// BookReviewHint is a duplicate hint as the model sees it.
type BookReviewHint struct {
	Match         string  `json:"match"`
	Title         string  `json:"title"`
	Status        string  `json:"status"`
	ISBN          *string `json:"isbn"`
	PublishedYear *int    `json:"published_year"`
}

// BookReviewData is the book's own model input: everything the model sees
// except duplicate hints. Its canonical JSON is hashed (InputHash), so any
// change here makes stored verdicts stale.
type BookReviewData struct {
	Title         string             `json:"title"`
	Authors       []BookReviewAuthor `json:"authors"`
	PublishedYear *int               `json:"published_year"`
	ISBN          *string            `json:"isbn"`
	Language      string             `json:"language"`
	Description   string             `json:"description"`
	Genres        []string           `json:"genres"`
	Source        string             `json:"source"`
	ExternalID    *string            `json:"external_id"`
}

// BookReviewInput is the full model input: the book's own data plus its
// current duplicate hints (not part of the hash).
type BookReviewInput struct {
	BookReviewData
	Similar []BookReviewHint `json:"similar"`
}

// BookReviewTarget is one book prepared for an AI review.
type BookReviewTarget struct {
	BookID    uuid.UUID
	Status    string
	Input     BookReviewInput
	InputHash string
}

// BookAIVerdictRecord is a computed verdict to store.
type BookAIVerdictRecord struct {
	BookID        uuid.UUID
	Verdict       string
	ReasonCodes   []string
	Reason        string
	Confidence    float64
	Model         string
	PromptVersion int
	InputHash     string
	InputTokens   int
	OutputTokens  int
}

// BookAIVerdictView is a stored verdict as shown in the queue. Stale is
// true when the prompt version or the book's own input changed since.
type BookAIVerdictView struct {
	Verdict       string
	ReasonCodes   []string
	Reason        string
	Confidence    float64
	Model         string
	PromptVersion int
	CreatedAt     time.Time
	Stale         bool
}

// BookReviewDataOf builds the book's own model input from a book with
// authors and genres preloaded. Authors and genres are sorted by name,
// then id, so the result (and its hash) does not depend on load order.
func BookReviewDataOf(b catalogDomain.Book) BookReviewData {
	authors := append([]catalogDomain.Author(nil), b.Authors...)
	sort.SliceStable(authors, func(i, j int) bool {
		if authors[i].Name != authors[j].Name {
			return authors[i].Name < authors[j].Name
		}
		return authors[i].ID.String() < authors[j].ID.String()
	})
	genres := append([]catalogDomain.Genre(nil), b.Genres...)
	sort.SliceStable(genres, func(i, j int) bool {
		if genres[i].Name != genres[j].Name {
			return genres[i].Name < genres[j].Name
		}
		return genres[i].ID.String() < genres[j].ID.String()
	})

	data := BookReviewData{
		Title:         b.Title,
		Authors:       make([]BookReviewAuthor, 0, len(authors)),
		PublishedYear: b.PublishedYear,
		ISBN:          b.ISBN,
		Language:      b.Language,
		Genres:        make([]string, 0, len(genres)),
		Source:        "manual",
		ExternalID:    b.ExternalID,
	}
	for _, a := range authors {
		data.Authors = append(data.Authors, BookReviewAuthor{Name: a.Name, Status: a.Status})
	}
	for _, g := range genres {
		data.Genres = append(data.Genres, g.Name)
	}
	if b.Description != nil {
		data.Description = truncateRunes(*b.Description, reviewDescriptionMaxRunes)
	}
	if b.ExternalSource != nil && *b.ExternalSource != "" {
		data.Source = *b.ExternalSource
	}
	return data
}

// InputHash is the hex SHA-256 of the canonical JSON of the book's own
// model input. The worker stores it with a verdict and the queue compares
// it to flag stale verdicts; both go through this one function.
func InputHash(data BookReviewData) string {
	raw, err := json.Marshal(data)
	if err != nil {
		// Only strings, ints and slices of them: Marshal cannot fail.
		panic(fmt.Sprintf("marshal review data: %v", err))
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// BookReviewInputs prepares ids for an AI review, in the order of ids
// (unknown and repeated ids are dropped). Books of any status are returned
// with their status, so the caller can skip those no longer pending.
// Duplicate hints are computed exactly as in the queue.
func (s *Service) BookReviewInputs(ctx context.Context, ids []uuid.UUID) ([]BookReviewTarget, error) {
	if len(ids) == 0 {
		return []BookReviewTarget{}, nil
	}
	loaded, err := s.repo.LoadBooksForReview(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load books for review: %w", err)
	}
	byID := make(map[uuid.UUID]catalogDomain.Book, len(loaded))
	for _, b := range loaded {
		byID[b.ID] = b
	}
	books := make([]catalogDomain.Book, 0, len(loaded))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		b, ok := byID[id]
		if _, dup := seen[id]; !ok || dup {
			continue
		}
		seen[id] = struct{}{}
		books = append(books, b)
	}

	hints, err := s.bookHintsFor(ctx, books)
	if err != nil {
		return nil, err
	}
	out := make([]BookReviewTarget, 0, len(books))
	for i, b := range books {
		data := BookReviewDataOf(b)
		similar := make([]BookReviewHint, 0, len(hints[i]))
		for _, h := range hints[i] {
			similar = append(similar, BookReviewHint{
				Match: h.Match, Title: h.Book.Title, Status: h.Book.Status,
				ISBN: h.Book.ISBN, PublishedYear: h.Book.PublishedYear,
			})
		}
		out = append(out, BookReviewTarget{
			BookID:    b.ID,
			Status:    b.Status,
			Input:     BookReviewInput{BookReviewData: data, Similar: similar},
			InputHash: InputHash(data),
		})
	}
	return out, nil
}

// SaveBookAIVerdict stores (or replaces) a book's verdict. It writes only
// the verdict table: no book, author or genre field changes. Confidence is
// rounded to 2 decimals.
func (s *Service) SaveBookAIVerdict(ctx context.Context, v BookAIVerdictRecord) error {
	if !aimodDomain.IsVerdict(v.Verdict) || v.Confidence < 0 || v.Confidence > 1 || v.Model == "" || v.InputHash == "" {
		return ErrInvalidAIVerdict
	}
	codes := v.ReasonCodes
	if codes == nil {
		codes = []string{}
	}
	for _, c := range codes {
		if !aimodDomain.IsReasonCode(c) {
			return fmt.Errorf("%w: unknown reason code %q", ErrInvalidAIVerdict, c)
		}
	}
	raw, err := json.Marshal(codes)
	if err != nil {
		return fmt.Errorf("marshal reason codes: %w", err)
	}
	if err := s.repo.UpsertBookAIVerdict(ctx, repository.BookAIVerdict{
		BookID:        v.BookID,
		Verdict:       v.Verdict,
		ReasonCodes:   string(raw),
		Reason:        v.Reason,
		Confidence:    math.Round(v.Confidence*100) / 100,
		Model:         v.Model,
		PromptVersion: v.PromptVersion,
		InputHash:     v.InputHash,
		InputTokens:   v.InputTokens,
		OutputTokens:  v.OutputTokens,
	}); err != nil {
		return fmt.Errorf("save AI verdict: %w", err)
	}
	return nil
}

// ListAIReviewCandidates returns up to max pending books the periodic
// sweep should check (no verdict first, then oldest first).
func (s *Service) ListAIReviewCandidates(ctx context.Context, max, promptVersion int) ([]uuid.UUID, error) {
	ids, err := s.repo.ListAIReviewCandidateIDs(ctx, max, promptVersion)
	if err != nil {
		return nil, fmt.Errorf("list AI review candidates: %w", err)
	}
	return ids, nil
}

// CountAIReviewCandidates counts the pending books the periodic sweep
// would pick (no verdict, an older prompt version or updated after it).
func (s *Service) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	n, err := s.repo.CountAIReviewCandidates(ctx, promptVersion)
	if err != nil {
		return 0, fmt.Errorf("count AI review candidates: %w", err)
	}
	return n, nil
}

// aiVerdictViews loads the verdicts of books (one query) and flags stale
// ones against the current prompt version and input hash.
func (s *Service) aiVerdictViews(ctx context.Context, books []catalogDomain.Book) (map[uuid.UUID]*BookAIVerdictView, error) {
	ids := make([]uuid.UUID, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID)
	}
	rows, err := s.repo.GetBookAIVerdicts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get AI verdicts: %w", err)
	}
	out := make(map[uuid.UUID]*BookAIVerdictView, len(rows))
	for _, b := range books {
		row, ok := rows[b.ID]
		if !ok {
			continue
		}
		codes := []string{}
		if row.ReasonCodes != "" {
			if err := json.Unmarshal([]byte(row.ReasonCodes), &codes); err != nil || codes == nil {
				codes = []string{}
			}
		}
		out[b.ID] = &BookAIVerdictView{
			Verdict:       row.Verdict,
			ReasonCodes:   codes,
			Reason:        row.Reason,
			Confidence:    row.Confidence,
			Model:         row.Model,
			PromptVersion: row.PromptVersion,
			CreatedAt:     row.CreatedAt,
			Stale:         row.PromptVersion != aimodDomain.PromptVersion || row.InputHash != InputHash(BookReviewDataOf(b)),
		}
	}
	return out, nil
}
