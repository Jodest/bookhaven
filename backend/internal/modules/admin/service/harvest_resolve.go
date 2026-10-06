package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ResolveOrCreateAuthorIDsByNames resolves each display name to an existing
// author (matched by slugify(name), the existing normalized-name key) or
// creates a new author when no match exists. Empty/whitespace-only names are
// skipped. Order is not guaranteed to match the input order; duplicates
// (by resolved ID) are collapsed.
//
// This is the admin JSON/CSV import's author-matching strategy: exact
// normalized-name match, autocreate on miss, no fuzzy matching (business
// analysis Decision 3.a). Import is admin-curated input, so missing authors
// are created as published, and a slug-matched author is linked whatever
// its status (see docs/features/author-status.md rule 5). Harvest uses
// ResolveOrCreateHarvestAuthors instead.
func (s *Service) ResolveOrCreateAuthorIDsByNames(ctx context.Context, names []string) ([]string, error) {
	seen := make(map[string]struct{}, len(names))
	ids := make([]string, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		slug := slugify(name)
		author, err := s.repo.GetAuthorBySlug(ctx, slug)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("lookup author %q: %w", name, err)
			}
			created, createErr := s.CreateAuthor(ctx, AuthorInput{Name: name})
			if createErr != nil {
				return nil, fmt.Errorf("create author %q: %w", name, createErr)
			}
			author = created
		}

		id := author.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}

// AuthorRef is one harvested author reference: a display name plus an
// optional bare Wikidata QID (empty when the source reported none).
type AuthorRef struct {
	Name string
	QID  string
}

// ResolveOrCreateHarvestAuthors resolves harvested author refs to author IDs
// per docs/features/author-status.md rule 4:
//
//  1. QID present → match by (wikidata, QID). A rejected match is a
//     tombstone: the ref is skipped (no link, nothing created). A
//     pending/published match is linked.
//  2. Otherwise / no QID match → match by slugify(name).
//     - rejected → skipped; if it has no QID and one was given, the QID is
//     backfilled so future harvests match the tombstone by QID.
//     - no QID on the author, QID given → QID backfilled (status
//     unchanged) and linked.
//     - same QID or no QID given → linked.
//     - a different QID (namesake) → a new pending author is created with
//     slug slugify(name)+"-"+lower(QID) and linked; the existing one is
//     left untouched.
//  3. No match → a new pending author (with the QID, if any) is created and
//     linked.
//
// Empty names are skipped; duplicates (by resolved ID) are collapsed. When
// every ref is skipped the result is empty (the caller still creates the
// hidden book) and an info line with the skipped QIDs is logged.
func (s *Service) ResolveOrCreateHarvestAuthors(ctx context.Context, refs []AuthorRef) ([]string, error) {
	seen := make(map[string]struct{}, len(refs))
	ids := make([]string, 0, len(refs))
	var skipped []string

	for _, ref := range refs {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		qid := strings.TrimSpace(ref.QID)

		author, err := s.resolveHarvestAuthor(ctx, name, qid)
		if err != nil {
			return nil, err
		}
		if author == nil {
			skipped = append(skipped, name+" "+qid)
			continue
		}

		id := author.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(skipped) > 0 {
		s.log.Info("harvest skipped rejected (tombstoned) authors",
			zap.Strings("authors", skipped),
			zap.Bool("all_skipped", len(ids) == 0))
	}
	return ids, nil
}

// resolveHarvestAuthor applies rule 4 to a single (trimmed, non-empty)
// name/QID. A nil author with a nil error means "skip" (tombstone).
func (s *Service) resolveHarvestAuthor(ctx context.Context, name, qid string) (*catalogDomain.Author, error) {
	if qid != "" {
		author, err := s.repo.GetAuthorByExternalID(ctx, catalogDomain.AuthorExternalSourceWikidata, qid)
		switch {
		case err == nil:
			if author.Status == catalogDomain.AuthorStatusRejected {
				return nil, nil
			}
			return author, nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return nil, fmt.Errorf("lookup author by qid %q: %w", qid, err)
		}
	}

	slug := slugify(name)
	author, err := s.repo.GetAuthorBySlug(ctx, slug)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("lookup author %q: %w", name, err)
		}
		return s.createHarvestAuthor(ctx, name, "", qid)
	}

	hasQID := author.ExternalSource != nil && author.ExternalID != nil
	if qid != "" && !hasQID {
		if err := s.repo.SetAuthorExternalID(ctx, author.ID, catalogDomain.AuthorExternalSourceWikidata, qid); err != nil {
			return nil, fmt.Errorf("backfill author %q qid %q: %w", name, qid, err)
		}
		source := catalogDomain.AuthorExternalSourceWikidata
		author.ExternalSource = &source
		author.ExternalID = &qid
		hasQID = true
	}

	if author.Status == catalogDomain.AuthorStatusRejected {
		return nil, nil
	}

	if qid != "" && hasQID && (*author.ExternalSource != catalogDomain.AuthorExternalSourceWikidata || *author.ExternalID != qid) {
		// Namesake: a different entity already owns this slug.
		return s.createHarvestAuthor(ctx, name, slug+"-"+strings.ToLower(qid), qid)
	}
	return author, nil
}

// createHarvestAuthor creates a pending author (slug "" = derived from the
// name) carrying the Wikidata QID when one is given.
func (s *Service) createHarvestAuthor(ctx context.Context, name, slug, qid string) (*catalogDomain.Author, error) {
	identity := authorIdentity{status: catalogDomain.AuthorStatusPending}
	if qid != "" {
		source := catalogDomain.AuthorExternalSourceWikidata
		extID := qid
		identity.externalSource = &source
		identity.externalID = &extID
	}
	created, err := s.createAuthor(ctx, AuthorInput{Name: name, Slug: slug}, identity)
	if err != nil {
		return nil, fmt.Errorf("create author %q: %w", name, err)
	}
	return created, nil
}

// ResolveOrCreateGenreIDsByNames resolves each genre name to an existing
// genre (matched by slugify(name)) or creates a new genre when no match
// exists. Empty/whitespace-only names are skipped. Duplicates (by resolved
// ID) are collapsed. See ResolveOrCreateAuthorIDsByNames for the matching
// policy this mirrors.
func (s *Service) ResolveOrCreateGenreIDsByNames(ctx context.Context, names []string) ([]string, error) {
	seen := make(map[string]struct{}, len(names))
	ids := make([]string, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		slug := slugify(name)
		genre, err := s.repo.GetGenreBySlug(ctx, slug)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("lookup genre %q: %w", name, err)
			}
			created, createErr := s.CreateGenre(ctx, GenreInput{Name: name})
			if createErr != nil {
				return nil, fmt.Errorf("create genre %q: %w", name, createErr)
			}
			genre = created
		}

		id := genre.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}
