package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Admin users management errors (docs/features/admin-users.md).
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidRole         = errors.New("role is invalid")
	ErrCannotChangeOwnRole = errors.New("cannot change own role")
	ErrLastAdmin           = errors.New("cannot remove the last admin")
	// ErrActorNotAdmin: the acting admin was demoted before the role
	// change transaction ran.
	ErrActorNotAdmin = errors.New("actor is no longer an admin")
)

// ListUsers returns a paginated, optionally filtered list of users. A
// non-empty role must be a valid user_role value (ErrInvalidRole).
func (s *Service) ListUsers(ctx context.Context, search, role string, page, limit int) ([]usersDomain.User, int64, error) {
	if role != "" && !usersDomain.IsValidRole(role) {
		return nil, 0, ErrInvalidRole
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListUsers(ctx, repository.UserFilter{
		Search: search,
		Role:   role,
		Limit:  limit,
		Offset: (page - 1) * limit,
	})
}

// ChangeUserRole sets targetID's role on behalf of actorID. Rules: the role
// must be valid; an admin cannot change their own role; the actor must
// still be an admin at write time; the last admin cannot be demoted.
// Setting the current role is a no-op (no write, no log line).
func (s *Service) ChangeUserRole(ctx context.Context, actorID, targetID uuid.UUID, role string) (*usersDomain.User, error) {
	if !usersDomain.IsValidRole(role) {
		return nil, ErrInvalidRole
	}
	if actorID == targetID {
		return nil, ErrCannotChangeOwnRole
	}
	newRole := usersDomain.Role(role)

	user, oldRole, err := s.repo.ChangeUserRole(ctx, targetID, func(target usersDomain.User, adminIDs []uuid.UUID) (usersDomain.Role, error) {
		actorIsAdmin := false
		for _, id := range adminIDs {
			if id == actorID {
				actorIsAdmin = true
				break
			}
		}
		if !actorIsAdmin {
			return "", ErrActorNotAdmin
		}
		if target.Role == usersDomain.RoleAdmin && newRole != usersDomain.RoleAdmin && len(adminIDs) <= 1 {
			return "", ErrLastAdmin
		}
		return newRole, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrUserNotFound
		case errors.Is(err, ErrActorNotAdmin), errors.Is(err, ErrLastAdmin):
			return nil, err
		default:
			return nil, fmt.Errorf("change user role: %w", err)
		}
	}

	if oldRole != newRole {
		s.log.Info("admin changed user role",
			zap.String("actor_id", actorID.String()),
			zap.String("target_id", targetID.String()),
			zap.String("old_role", string(oldRole)),
			zap.String("new_role", string(newRole)),
		)
	}
	return user, nil
}
