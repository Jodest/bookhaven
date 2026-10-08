package repository

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// usersDDL recreates the users table with SQLite-native types (the
// production schema is Postgres-only). The SQLite dialect drops FOR UPDATE,
// so row locking itself is not exercised here.
const usersDDL = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT,
	name TEXT,
	avatar_url TEXT,
	role TEXT NOT NULL DEFAULT 'user',
	profile_type TEXT NOT NULL DEFAULT 'reader',
	email_verified_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);
`

type usersFixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
	base time.Time
	n    int
}

func newUsersRepo(t *testing.T) *usersFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(usersDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &usersFixture{t: t, repo: &GormRepository{db: db}, db: db, base: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)}
}

// user inserts a user; each call is one minute newer than the last.
func (f *usersFixture) user(email string, name *string, role usersDomain.Role) uuid.UUID {
	f.t.Helper()
	f.n++
	id := uuid.New()
	ts := f.base.Add(time.Duration(f.n) * time.Minute)
	if err := f.db.Exec(
		"INSERT INTO users (id, email, password_hash, name, role, profile_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, email, "secret-hash", name, string(role), "reader", ts, ts,
	).Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *usersFixture) get(id uuid.UUID) usersDomain.User {
	f.t.Helper()
	var u usersDomain.User
	if err := f.db.First(&u, "id = ?", id).Error; err != nil {
		f.t.Fatalf("get user: %v", err)
	}
	return u
}

func strp(s string) *string { return &s }

func TestListUsers_SearchRoleOrderTotal(t *testing.T) {
	f := newUsersRepo(t)
	ann := f.user("ann@example.com", strp("Анна"), usersDomain.RoleUser)
	bob := f.user("BOB@Example.com", strp("Bob Smith"), usersDomain.RoleModerator)
	carl := f.user("carl@example.com", nil, usersDomain.RoleModerator)
	dina := f.user("dina@other.org", strp("Dina SMITHSON"), usersDomain.RoleAdmin)
	ctx := context.Background()

	ids := func(users []usersDomain.User) []uuid.UUID {
		out := make([]uuid.UUID, len(users))
		for i, u := range users {
			out[i] = u.ID
		}
		return out
	}
	eq := func(got, want []uuid.UUID) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// No filter: newest first, total counts everything.
	users, total, err := f.repo.ListUsers(ctx, UserFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 || !eq(ids(users), []uuid.UUID{dina, carl, bob, ann}) {
		t.Fatalf("unexpected list: total=%d ids=%v", total, ids(users))
	}

	// Email match, case-insensitive.
	users, total, _ = f.repo.ListUsers(ctx, UserFilter{Search: "bob@EXAMPLE"})
	if total != 1 || !eq(ids(users), []uuid.UUID{bob}) {
		t.Fatalf("email search: total=%d ids=%v", total, ids(users))
	}

	// Name match, case-insensitive (ASCII; SQLite LOWER is ASCII-only).
	users, total, _ = f.repo.ListUsers(ctx, UserFilter{Search: "smith"})
	if total != 2 || !eq(ids(users), []uuid.UUID{dina, bob}) {
		t.Fatalf("name search: total=%d ids=%v", total, ids(users))
	}

	// Role filter.
	users, total, _ = f.repo.ListUsers(ctx, UserFilter{Role: "moderator"})
	if total != 2 || !eq(ids(users), []uuid.UUID{carl, bob}) {
		t.Fatalf("role filter: total=%d ids=%v", total, ids(users))
	}

	// Search + role, with pagination: total ignores limit/offset.
	users, total, _ = f.repo.ListUsers(ctx, UserFilter{Search: "example.com", Role: "moderator", Limit: 1, Offset: 1})
	if total != 2 || !eq(ids(users), []uuid.UUID{bob}) {
		t.Fatalf("paged: total=%d ids=%v", total, ids(users))
	}
}

func TestChangeUserRole_WritesOnlyRole(t *testing.T) {
	f := newUsersRepo(t)
	admin := f.user("admin@example.com", strp("Admin"), usersDomain.RoleAdmin)
	target := f.user("t@example.com", strp("Target"), usersDomain.RoleUser)
	before := f.get(target)

	var gotTarget usersDomain.User
	var gotAdmins []uuid.UUID
	user, old, err := f.repo.ChangeUserRole(context.Background(), target, func(u usersDomain.User, adminIDs []uuid.UUID) (usersDomain.Role, error) {
		gotTarget, gotAdmins = u, adminIDs
		return usersDomain.RoleModerator, nil
	})
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if old != usersDomain.RoleUser || user.Role != usersDomain.RoleModerator {
		t.Fatalf("old=%q new=%q", old, user.Role)
	}
	if gotTarget.ID != target || gotTarget.Role != usersDomain.RoleUser {
		t.Fatalf("decide got target %+v", gotTarget)
	}
	if len(gotAdmins) != 1 || gotAdmins[0] != admin {
		t.Fatalf("decide got admins %v", gotAdmins)
	}

	after := f.get(target)
	if after.Role != usersDomain.RoleModerator {
		t.Fatalf("role not written: %q", after.Role)
	}
	if after.Email != before.Email || *after.Name != *before.Name || after.PasswordHash == nil || *after.PasswordHash != "secret-hash" ||
		after.ProfileType != before.ProfileType || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("other columns changed: before=%+v after=%+v", before, after)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at not bumped: %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestChangeUserRole_AdminSetIncludesAllAdmins(t *testing.T) {
	f := newUsersRepo(t)
	a1 := f.user("a1@example.com", nil, usersDomain.RoleAdmin)
	a2 := f.user("a2@example.com", nil, usersDomain.RoleAdmin)
	f.user("m@example.com", nil, usersDomain.RoleModerator)

	var gotAdmins []uuid.UUID
	_, old, err := f.repo.ChangeUserRole(context.Background(), a2, func(u usersDomain.User, adminIDs []uuid.UUID) (usersDomain.Role, error) {
		gotAdmins = adminIDs
		return u.Role, nil
	})
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if old != usersDomain.RoleAdmin {
		t.Fatalf("old = %q", old)
	}
	want := []uuid.UUID{a1, a2}
	sort.Slice(want, func(i, j int) bool { return want[i].String() < want[j].String() })
	if len(gotAdmins) != 2 || gotAdmins[0] != want[0] || gotAdmins[1] != want[1] {
		t.Fatalf("admins = %v, want %v (ordered by id)", gotAdmins, want)
	}
}

func TestChangeUserRole_SameRole_NoWrite(t *testing.T) {
	f := newUsersRepo(t)
	f.user("admin@example.com", nil, usersDomain.RoleAdmin)
	target := f.user("t@example.com", nil, usersDomain.RoleContributor)
	before := f.get(target)

	user, old, err := f.repo.ChangeUserRole(context.Background(), target, func(u usersDomain.User, _ []uuid.UUID) (usersDomain.Role, error) {
		return u.Role, nil
	})
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if old != usersDomain.RoleContributor || user.Role != usersDomain.RoleContributor {
		t.Fatalf("old=%q new=%q", old, user.Role)
	}
	if after := f.get(target); !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("expected no write, updated_at %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestChangeUserRole_DecideErrorRollsBack(t *testing.T) {
	f := newUsersRepo(t)
	f.user("admin@example.com", nil, usersDomain.RoleAdmin)
	target := f.user("t@example.com", nil, usersDomain.RoleAdmin)
	boom := errors.New("rule violated")

	_, _, err := f.repo.ChangeUserRole(context.Background(), target, func(usersDomain.User, []uuid.UUID) (usersDomain.Role, error) {
		return "", boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected decide error, got %v", err)
	}
	if after := f.get(target); after.Role != usersDomain.RoleAdmin {
		t.Fatalf("expected role unchanged, got %q", after.Role)
	}
}

func TestChangeUserRole_NotFound(t *testing.T) {
	f := newUsersRepo(t)
	f.user("admin@example.com", nil, usersDomain.RoleAdmin)
	called := false
	_, _, err := f.repo.ChangeUserRole(context.Background(), uuid.New(), func(usersDomain.User, []uuid.UUID) (usersDomain.Role, error) {
		called = true
		return usersDomain.RoleUser, nil
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
	if called {
		t.Fatal("decide must not run for a missing target")
	}
}
