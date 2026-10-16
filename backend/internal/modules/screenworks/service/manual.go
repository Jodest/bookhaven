package service

import (
	"context"
	"errors"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
)

// Manual editing of screen works (openspec change add-screen-works-admin-editing).

// Work list paging.
const (
	DefaultWorkLimit = 20
	MaxWorkLimit     = 100
)

// AddLinkResult is a created manual link as the admin item plus whether the
// work was newly created.
type AddLinkResult struct {
	Item        *domain.LinkedWork
	WorkCreated bool
}

// WorkList is one page of the admin work list.
type WorkList struct {
	Items []domain.WorkListItem
	Total int64
	Page  int
	Limit int
}

// buildRef checks that exactly one of workID and work is given and validates
// the work data before any repository call.
func buildRef(workID *uuid.UUID, work *domain.WorkFields) (domain.WorkRef, error) {
	if (workID == nil) == (work == nil) {
		return domain.WorkRef{}, domain.ErrLinkRefRequired
	}
	if workID != nil {
		return domain.WorkRef{WorkID: workID}, nil
	}
	f, err := domain.NormalizeWorkFields(*work)
	if err != nil {
		return domain.WorkRef{}, err
	}
	return domain.WorkRef{Fields: &f}, nil
}

// AddBookAdaptation links an existing or a new work to a book by hand.
func (s *Service) AddBookAdaptation(ctx context.Context, bookID uuid.UUID, workID *uuid.UUID, work *domain.WorkFields) (*AddLinkResult, error) {
	ref, err := buildRef(workID, work)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.BookStatus(ctx, bookID); err != nil {
		return nil, err
	}
	id, created, err := s.repo.AddManualBookLink(ctx, bookID, ref)
	if err != nil {
		return nil, err
	}
	links, err := s.repo.ListAdminByBook(ctx, bookID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].ID == id {
			return &AddLinkResult{Item: &links[i], WorkCreated: created}, nil
		}
	}
	return nil, domain.ErrLinkNotFound
}

// AddAuthorFilm links an existing or a new work to an author by hand.
func (s *Service) AddAuthorFilm(ctx context.Context, authorID uuid.UUID, workID *uuid.UUID, work *domain.WorkFields, relation string) (*AddLinkResult, error) {
	if !domain.IsValidRelation(relation) {
		return nil, domain.ErrInvalidRelation
	}
	ref, err := buildRef(workID, work)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AuthorStatus(ctx, authorID); err != nil {
		return nil, err
	}
	id, created, err := s.repo.AddManualAuthorLink(ctx, authorID, ref, relation)
	if err != nil {
		return nil, err
	}
	links, err := s.repo.ListAdminByAuthor(ctx, authorID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].ID == id {
			return &AddLinkResult{Item: &links[i], WorkCreated: created}, nil
		}
	}
	return nil, domain.ErrAuthorLinkNotFound
}

// DeleteBookAdaptation deletes a manual adaptation link.
func (s *Service) DeleteBookAdaptation(ctx context.Context, bookID, workID uuid.UUID) error {
	return s.repo.DeleteManualBookLink(ctx, bookID, workID)
}

// DeleteAuthorFilm deletes a manual author film link.
func (s *Service) DeleteAuthorFilm(ctx context.Context, authorID, workID uuid.UUID) error {
	return s.repo.DeleteManualAuthorLink(ctx, authorID, workID)
}

// UpdateAuthorFilm sets the hidden flag, the relation or both of a link and
// returns the updated item.
func (s *Service) UpdateAuthorFilm(ctx context.Context, authorID, workID uuid.UUID, hidden *bool, relation *string) (*domain.LinkedWork, error) {
	if hidden == nil && relation == nil {
		return nil, domain.ErrLinkPatchEmpty
	}
	if relation != nil && !domain.IsValidRelation(*relation) {
		return nil, domain.ErrInvalidRelation
	}
	if _, err := s.repo.AuthorStatus(ctx, authorID); err != nil {
		if errors.Is(err, domain.ErrAuthorNotFound) {
			return nil, domain.ErrAuthorLinkNotFound
		}
		return nil, err
	}
	if err := s.repo.UpdateAuthorLink(ctx, authorID, workID, hidden, relation); err != nil {
		return nil, err
	}
	links, err := s.repo.ListAdminByAuthor(ctx, authorID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].ID == workID {
			return &links[i], nil
		}
	}
	return nil, domain.ErrAuthorLinkNotFound
}

// ListWorks returns a page of the admin work list; paging values clamp.
func (s *Service) ListWorks(ctx context.Context, f domain.WorkFilter) (*WorkList, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = DefaultWorkLimit
	}
	if f.Limit > MaxWorkLimit {
		f.Limit = MaxWorkLimit
	}
	items, total, err := s.repo.ListWorks(ctx, f)
	if err != nil {
		return nil, err
	}
	return &WorkList{Items: items, Total: total, Page: f.Page, Limit: f.Limit}, nil
}

// GetWork returns a work with its linked books and authors.
func (s *Service) GetWork(ctx context.Context, id uuid.UUID) (*domain.WorkDetail, error) {
	return s.repo.GetWork(ctx, id)
}

// UpdateWork replaces the work data (marking it edited by hand) and returns
// the detail.
func (s *Service) UpdateWork(ctx context.Context, id uuid.UUID, work domain.WorkFields) (*domain.WorkDetail, error) {
	f, err := domain.NormalizeWorkFields(work)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateWork(ctx, id, f); err != nil {
		return nil, err
	}
	return s.repo.GetWork(ctx, id)
}

// DeleteWork deletes a work without links.
func (s *Service) DeleteWork(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteWorkIfUnlinked(ctx, id)
}
