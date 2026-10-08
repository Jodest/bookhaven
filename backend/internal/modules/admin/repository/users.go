package repository

import (
	"context"
	"strings"
	"time"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserFilter defines filtering options for the admin users list
// (docs/features/admin-users.md).
type UserFilter struct {
	// Search matches email OR name, case-insensitive substring.
	Search string
	// Role, when non-empty, restricts the result to that exact role.
	Role   string
	Limit  int
	Offset int
}

// RoleDecider receives the locked target row and the ids of the locked
// admin set and returns the role to write (or an error to abort and roll
// back). It must not perform external calls: it runs inside the
// transaction while the admin rows are locked.
type RoleDecider func(target usersDomain.User, adminIDs []uuid.UUID) (usersDomain.Role, error)

func (r *GormRepository) ListUsers(ctx context.Context, filter UserFilter) ([]usersDomain.User, int64, error) {
	query := r.db.WithContext(ctx).Model(&usersDomain.User{})

	if filter.Search != "" {
		// LOWER(...) LIKE instead of ILIKE so the SQLite repository tests
		// run the same query; semantics match (case-insensitive substring).
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("(LOWER(email) LIKE ? OR LOWER(name) LIKE ?)", pattern, pattern)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var users []usersDomain.User
	if err := query.Order("created_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// ChangeUserRole locks the admin set plus the target row in one statement
// (SELECT ... WHERE role = 'admin' OR id = target ORDER BY id FOR UPDATE -
// deterministic lock order, so concurrent role changes cannot deadlock),
// asks decide for the new role and writes only role/updated_at. Under READ
// COMMITTED a waiting FOR UPDATE re-evaluates the WHERE against the
// winner's committed rows, so a concurrently demoted admin drops out of the
// admin set passed to decide. The write is skipped when decide returns the
// current role. Returns gorm.ErrRecordNotFound when the target does not
// exist; any decide error rolls the transaction back and is returned as is.
// The returned role is the target's role before the change.
func (r *GormRepository) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	var (
		result  usersDomain.User
		oldRole usersDomain.Role
	)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked []usersDomain.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("role = ? OR id = ?", usersDomain.RoleAdmin, targetID).
			Order("id").
			Find(&locked).Error; err != nil {
			return err
		}

		var target *usersDomain.User
		adminIDs := make([]uuid.UUID, 0, len(locked))
		for i := range locked {
			if locked[i].ID == targetID {
				target = &locked[i]
			}
			if locked[i].Role == usersDomain.RoleAdmin {
				adminIDs = append(adminIDs, locked[i].ID)
			}
		}
		if target == nil {
			return gorm.ErrRecordNotFound
		}

		newRole, err := decide(*target, adminIDs)
		if err != nil {
			return err
		}
		oldRole = target.Role
		result = *target
		if newRole == target.Role {
			return nil
		}

		now := time.Now()
		if err := tx.Model(&usersDomain.User{}).
			Where("id = ?", targetID).
			Updates(map[string]any{"role": newRole, "updated_at": now}).Error; err != nil {
			return err
		}
		result.Role = newRole
		result.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return &result, oldRole, nil
}
