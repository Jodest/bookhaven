package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func newRoleChangeContext(id, body string) (echo.Context, *httptest.ResponseRecorder) {
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/users/"+id+"/role", []byte(body))
	c.SetParamNames("id")
	c.SetParamValues(id)
	return c, rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return got
}

func TestListUsers_OKEnvelopeWithoutPasswordHash(t *testing.T) {
	h, svc := setupHandler()
	hash := "secret-hash"
	name := "Анна"
	verified := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	svc.users = []usersDomain.User{
		{ID: uuid.New(), Email: "ann@example.com", PasswordHash: &hash, Name: &name, Role: usersDomain.RoleModerator, ProfileType: usersDomain.ProfileTypeReader, EmailVerifiedAt: &verified},
		{ID: uuid.New(), Email: "bob@example.com", PasswordHash: &hash, Role: usersDomain.RoleUser, ProfileType: usersDomain.ProfileTypeOther},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/users?search=ann&role=moderator&page=2&limit=5", nil)
	setAdminUser(c)
	if err := h.ListUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastUserSearch != "ann" || svc.lastUserRole != "moderator" || svc.lastUserPage != 2 || svc.lastUserLimit != 5 {
		t.Fatalf("unexpected args: %q %q %d %d", svc.lastUserSearch, svc.lastUserRole, svc.lastUserPage, svc.lastUserLimit)
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), hash) {
		t.Fatalf("response leaks password hash: %s", rec.Body.String())
	}

	got := decodeMap(t, rec)
	if got["total"] != float64(2) || got["page"] != float64(2) || got["limit"] != float64(5) {
		t.Fatalf("unexpected envelope: %v", got)
	}
	items, _ := got["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %v", got["items"])
	}
	first, _ := items[0].(map[string]interface{})
	if first["email"] != "ann@example.com" || first["name"] != "Анна" || first["role"] != "moderator" ||
		first["profile_type"] != "reader" || first["email_verified_at"] == nil {
		t.Fatalf("unexpected first item: %v", first)
	}
	second, _ := items[1].(map[string]interface{})
	if v, ok := second["name"]; !ok || v != nil {
		t.Fatalf("expected name:null, got %v", second)
	}
	if v, ok := second["email_verified_at"]; !ok || v != nil {
		t.Fatalf("expected email_verified_at:null, got %v", second)
	}
}

func TestListUsers_InvalidRole(t *testing.T) {
	h, svc := setupHandler()
	svc.listUsersErr = adminService.ErrInvalidRole

	c, rec := newContext(http.MethodGet, "/api/v1/admin/users?role=root", nil)
	setAdminUser(c)
	if err := h.ListUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest || decodeMap(t, rec)["code"] != "invalid_role" {
		t.Fatalf("expected 400 invalid_role, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUserRole_OK(t *testing.T) {
	h, svc := setupHandler()
	actorID, targetID := uuid.New(), uuid.New()
	svc.changeRoleUser = &usersDomain.User{ID: targetID, Email: "t@example.com", Role: usersDomain.RoleModerator, ProfileType: usersDomain.ProfileTypeReader}

	c, rec := newRoleChangeContext(targetID.String(), `{"role":"moderator"}`)
	c.Set("user", &usersDomain.User{ID: actorID, Role: usersDomain.RoleAdmin})
	if err := h.ChangeUserRole(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastActorID != actorID || svc.lastTargetID != targetID || svc.lastNewRole != "moderator" {
		t.Fatalf("unexpected args: %v %v %q", svc.lastActorID, svc.lastTargetID, svc.lastNewRole)
	}
	got := decodeMap(t, rec)
	if got["id"] != targetID.String() || got["role"] != "moderator" {
		t.Fatalf("unexpected body: %v", got)
	}
	if _, ok := got["password_hash"]; ok {
		t.Fatalf("response leaks password_hash: %v", got)
	}
}

func TestChangeUserRole_InvalidID(t *testing.T) {
	h, svc := setupHandler()
	c, rec := newRoleChangeContext("not-a-uuid", `{"role":"user"}`)
	setAdminUser(c)
	if err := h.ChangeUserRole(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest || decodeMap(t, rec)["error"] != "invalid user id" {
		t.Fatalf("expected 400 invalid user id, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.changeRoleCall {
		t.Fatal("service must not be called")
	}
}

func TestChangeUserRole_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{adminService.ErrInvalidRole, http.StatusBadRequest, "invalid_role"},
		{adminService.ErrUserNotFound, http.StatusNotFound, "user_not_found"},
		{adminService.ErrCannotChangeOwnRole, http.StatusConflict, "cannot_change_own_role"},
		{adminService.ErrLastAdmin, http.StatusConflict, "last_admin"},
		{adminService.ErrActorNotAdmin, http.StatusForbidden, ""},
		{errors.New("db down"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.changeRoleErr = tc.err

		c, rec := newRoleChangeContext(uuid.NewString(), `{"role":"user"}`)
		setAdminUser(c)
		if err := h.ChangeUserRole(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != tc.status {
			t.Fatalf("%v: expected %d, got %d: %s", tc.err, tc.status, rec.Code, rec.Body.String())
		}
		got := decodeMap(t, rec)
		if tc.code != "" && got["code"] != tc.code {
			t.Fatalf("%v: expected code %q, got %v", tc.err, tc.code, got)
		}
		if tc.err == adminService.ErrActorNotAdmin && got["error"] != "forbidden" {
			t.Fatalf("expected forbidden body, got %v", got)
		}
	}
}

func TestChangeUserRole_NoUserInContext(t *testing.T) {
	h, svc := setupHandler()
	c, rec := newRoleChangeContext(uuid.NewString(), `{"role":"user"}`)
	if err := h.ChangeUserRole(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if svc.changeRoleCall {
		t.Fatal("service must not be called")
	}
}
