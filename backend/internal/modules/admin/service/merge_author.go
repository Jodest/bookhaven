package service

import (
	"context"
	"errors"
	"fmt"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrCannotMergeIntoSelf is returned by MergeAuthors, without changing
// anything, when duplicateID and canonicalID are the same author.
var ErrCannotMergeIntoSelf = errors.New("cannot merge an author into itself")

// MergeAuthors consolidates the duplicate author (duplicateID) into the
// canonical author (canonicalID): every book_authors row referencing
// duplicateID is reassigned to canonicalID, except for books already
// linked to both authors, where the now-redundant duplicate row is
// dropped instead (GormRepository.MergeAuthors handles the de-dupe so no
// book_authors row ever violates its (book_id, author_id) composite
// primary key) - then the duplicate author row itself is deleted.
//
// Per docs/features/duplicate-author-merge.md's "No field reconciliation"
// business rule, the canonical author's own fields (name, slug,
// biography, photo_url, birth_date, death_date) are never modified by a
// merge - with one exception (author-status feature, rule 11): when the
// canonical author has no external identity (Wikidata QID) and the
// duplicate has one, the identity moves to the canonical so the next
// harvest matches it by QID instead of recreating the author. If both have
// one, the canonical keeps its own and the duplicate's is dropped (logged).
// Canonical status is never changed.
//
// Returns the canonical author (unchanged apart from a carried identity) and the number of books
// reassigned in step 1 (used by the API's books_reassigned field).
// Returns ErrCannotMergeIntoSelf if duplicateID == canonicalID, or
// ErrAuthorNotFound if either author does not exist - in both cases
// nothing is changed.
func (s *Service) MergeAuthors(ctx context.Context, duplicateID, canonicalID uuid.UUID) (*catalogDomain.Author, int64, error) {
	if duplicateID == canonicalID {
		return nil, 0, ErrCannotMergeIntoSelf
	}

	duplicate, err := s.repo.GetAuthorByID(ctx, duplicateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrAuthorNotFound
		}
		return nil, 0, fmt.Errorf("get duplicate author: %w", err)
	}

	canonical, err := s.repo.GetAuthorByID(ctx, canonicalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrAuthorNotFound
		}
		return nil, 0, fmt.Errorf("get canonical author: %w", err)
	}

	// Snapshot both rows before the merge deletes the duplicate.
	dup, result := *duplicate, *canonical

	booksReassigned, err := s.repo.MergeAuthors(ctx, duplicateID, canonicalID)
	if err != nil {
		return nil, 0, fmt.Errorf("merge authors: %w", err)
	}
	s.invalidateSimilar(ctx)

	// Mirror the repository's in-transaction identity carry-over.
	if dup.ExternalSource != nil && dup.ExternalID != nil {
		if result.ExternalSource == nil {
			result.ExternalSource = dup.ExternalSource
			result.ExternalID = dup.ExternalID
		} else {
			s.log.Info("author merge dropped duplicate external id; canonical keeps its own",
				zap.String("canonical_id", canonicalID.String()),
				zap.String("duplicate_id", duplicateID.String()),
				zap.String("duplicate_external_source", *dup.ExternalSource),
				zap.String("duplicate_external_id", *dup.ExternalID))
		}
	}

	return &result, booksReassigned, nil
}
