package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
)

// fakeUsersRepository overrides the users methods of fakeRepository with an
// in-memory model of the locked role-change transaction.
type fakeUsersRepository struct {
	*fakeRepository

	users       map[uuid.UUID]*usersDomain.User
	changeCalls int
	writes      int
	lastFilter  repository.UserFilter
	listCalls   int
}

func newUsersService(users ...usersDomain.User) (*Service, *fakeUsersRepository) {
	repo := &fakeUsersRepository{
		fakeRepository: &fakeRepository{},
		users:          map[uuid.UUID]*usersDomain.User{},
	}
	for i := range users {
		u := users[i]
		repo.users[u.ID] = &u
	}
	return NewService(repo), repo
}

func (f *fakeUsersRepository) ListUsers(ctx context.Context, filter repository.UserFilter) ([]usersDomain.User, int64, error) {
	f.listCalls++
	f.lastFilter = filter
	return nil, 0, nil
}

func (f *fakeUsersRepository) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide repository.RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	f.changeCalls++
	target, ok := f.users[targetID]
	if !ok {
		return nil, "", gorm.ErrRecordNotFound
	}
	var adminIDs []uuid.UUID
	for id, u := range f.users {
		if u.Role == usersDomain.RoleAdmin {
			adminIDs = append(adminIDs, id)
		}
	}
	newRole, err := decide(*target, adminIDs)
	if err != nil {
		return nil, "", err
	}
	old := target.Role
	if newRole != old {
		f.writes++
		target.Role = newRole
	}
	out := *target
	return &out, old, nil
}

func testUser(role usersDomain.Role) usersDomain.User {
	return usersDomain.User{ID: uuid.New(), Email: uuid.NewString() + "@example.com", Role: role}
}

func TestChangeUserRole_InvalidRole_NoRepoCall(t *testing.T) {
	actor, target := testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleUser)
	svc, repo := newUsersService(actor, target)
	for _, role := range []string{"", "superadmin", "Admin"} {
		if _, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, role); !errors.Is(err, ErrInvalidRole) {
			t.Fatalf("role %q: expected ErrInvalidRole, got %v", role, err)
		}
	}
	if repo.changeCalls != 0 {
		t.Fatalf("expected no repo call, got %d", repo.changeCalls)
	}
}

func TestChangeUserRole_Self_NoRepoCall(t *testing.T) {
	actor := testUser(usersDomain.RoleAdmin)
	svc, repo := newUsersService(actor)
	for _, role := range []string{"user", "admin"} {
		if _, err := svc.ChangeUserRole(context.Background(), actor.ID, actor.ID, role); !errors.Is(err, ErrCannotChangeOwnRole) {
			t.Fatalf("role %q: expected ErrCannotChangeOwnRole, got %v", role, err)
		}
	}
	if repo.changeCalls != 0 {
		t.Fatalf("expected no repo call, got %d", repo.changeCalls)
	}
}

func TestChangeUserRole_TargetNotFound(t *testing.T) {
	actor := testUser(usersDomain.RoleAdmin)
	svc, _ := newUsersService(actor)
	if _, err := svc.ChangeUserRole(context.Background(), actor.ID, uuid.New(), "moderator"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestChangeUserRole_ActorNotAdmin(t *testing.T) {
	// The actor was demoted concurrently: it is no longer in the admin set.
	actor, other, target := testUser(usersDomain.RoleModerator), testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleUser)
	svc, repo := newUsersService(actor, other, target)
	if _, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "moderator"); !errors.Is(err, ErrActorNotAdmin) {
		t.Fatalf("expected ErrActorNotAdmin, got %v", err)
	}
	if repo.writes != 0 || repo.users[target.ID].Role != usersDomain.RoleUser {
		t.Fatal("expected no write")
	}
}

func TestChangeUserRole_LastAdmin(t *testing.T) {
	// Unreachable through the endpoint today (the actor is an admin
	// distinct from the target, so an admin target means >= 2 admins); the
	// defence-in-depth invariant is reached with a contrived locked admin
	// set of size 1 that contains the actor.
	actorID := uuid.New()
	target := testUser(usersDomain.RoleAdmin)
	repo := &lastAdminRepo{fakeRepository: &fakeRepository{}, target: target, adminIDs: []uuid.UUID{actorID}}
	svc := NewService(repo)
	if _, err := svc.ChangeUserRole(context.Background(), actorID, target.ID, "user"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
	if repo.wrote {
		t.Fatal("expected no write")
	}
}

// lastAdminRepo feeds decide a fixed admin set (to reach the defence-in-
// depth last-admin rule).
type lastAdminRepo struct {
	*fakeRepository
	target   usersDomain.User
	adminIDs []uuid.UUID
	wrote    bool
}

func (f *lastAdminRepo) ChangeUserRole(ctx context.Context, targetID uuid.UUID, decide repository.RoleDecider) (*usersDomain.User, usersDomain.Role, error) {
	newRole, err := decide(f.target, f.adminIDs)
	if err != nil {
		return nil, "", err
	}
	f.wrote = newRole != f.target.Role
	out := f.target
	out.Role = newRole
	return &out, f.target.Role, nil
}

func TestChangeUserRole_PromoteUserToModerator(t *testing.T) {
	actor, target := testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleUser)
	svc, repo := newUsersService(actor, target)
	got, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "moderator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Role != usersDomain.RoleModerator || repo.users[target.ID].Role != usersDomain.RoleModerator || repo.writes != 1 {
		t.Fatalf("expected moderator written once, got %q (writes %d)", got.Role, repo.writes)
	}
}

func TestChangeUserRole_DemoteAdminWhenTwoAdmins(t *testing.T) {
	actor, target := testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleAdmin)
	svc, repo := newUsersService(actor, target)
	got, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Role != usersDomain.RoleUser || repo.writes != 1 {
		t.Fatalf("expected demotion, got %q (writes %d)", got.Role, repo.writes)
	}
}

func TestChangeUserRole_SameRole_NoWrite(t *testing.T) {
	actor, target := testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleContributor)
	svc, repo := newUsersService(actor, target)
	got, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "contributor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Role != usersDomain.RoleContributor || repo.writes != 0 {
		t.Fatalf("expected no write, got %q (writes %d)", got.Role, repo.writes)
	}
}

func TestListUsers_PassesFilter(t *testing.T) {
	svc, repo := newUsersService()
	if _, _, err := svc.ListUsers(context.Background(), "ann", "moderator", 3, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := repository.UserFilter{Search: "ann", Role: "moderator", Limit: 10, Offset: 20}
	if repo.lastFilter != want {
		t.Fatalf("filter = %+v, want %+v", repo.lastFilter, want)
	}

	if _, _, err := svc.ListUsers(context.Background(), "", "", 0, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := (repository.UserFilter{Limit: 100, Offset: 0}); repo.lastFilter != want {
		t.Fatalf("filter = %+v, want %+v", repo.lastFilter, want)
	}
}

func TestListUsers_InvalidRole(t *testing.T) {
	svc, repo := newUsersService()
	if _, _, err := svc.ListUsers(context.Background(), "", "root", 1, 20); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
	if repo.listCalls != 0 {
		t.Fatal("expected no repo call")
	}
}

func TestChangeUserRole_LogsOnlyRealChanges(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	actor, target := testUser(usersDomain.RoleAdmin), testUser(usersDomain.RoleUser)
	_, repo := newUsersService(actor, target)
	svc := NewService(repo, WithLogger(zap.New(core)))

	if _, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs.Len() != 0 {
		t.Fatalf("expected no log line for same role, got %d", logs.Len())
	}

	if _, err := svc.ChangeUserRole(context.Background(), actor.ID, target.ID, "moderator"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := logs.FilterMessage("admin changed user role").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["actor_id"] != actor.ID.String() || fields["target_id"] != target.ID.String() ||
		fields["old_role"] != "user" || fields["new_role"] != "moderator" {
		t.Fatalf("unexpected log fields: %v", fields)
	}
}
