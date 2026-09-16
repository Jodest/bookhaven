package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ResolveOrCreateAuthorIDsByNames resolves each display name to an existing
// author (matched by slugify(name), the existing normalized-name key) or
// creates a new author when no match exists. Empty/whitespace-only names are
// skipped. Order is not guaranteed to match the input order; duplicates
// (by resolved ID) are collapsed.
//
// This is the harvest handler's author-matching strategy: exact
// normalized-name match, autocreate on miss, no fuzzy matching (business
// analysis Decision 3.a), implemented via the existing slug-uniqueness
// infrastructure rather than a new comparison method.
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
