package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authservice "github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	gateSecret = "test-secret"
	gateIssuer = "bookhaven-test"
)

// stubUsersRepo serves a single user by id.
type stubUsersRepo struct {
	user *domain.User
}

func (r *stubUsersRepo) Create(ctx context.Context, user *domain.User) error        { return nil }
func (r *stubUsersRepo) UpdateProfile(ctx context.Context, user *domain.User) error { return nil }
func (r *stubUsersRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *stubUsersRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	u := *r.user
	return &u, nil
}

func gateFixture(t *testing.T, verified bool) (*authservice.Service, *stubUsersRepo, string) {
	t.Helper()
	user := &domain.User{ID: uuid.New(), Email: "anna@example.com", Role: domain.RoleUser}
	if verified {
		now := time.Now()
		user.EmailVerifiedAt = &now
	}
	svc := authservice.NewService(authservice.Config{
		JWTSecret:      gateSecret,
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         gateIssuer,
	}, nil, nil, nil, authservice.ResendLimiters{}, authservice.ResendLimiters{}, authservice.LoginLimiters{}, authservice.RegisterLimiters{}, nil)

	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID.String(),
		"iat": now.Unix(),
		"exp": now.Add(time.Minute).Unix(),
		"iss": gateIssuer,
		"typ": "access",
	}).SignedString([]byte(gateSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return svc, &stubUsersRepo{user: user}, token
}

func gateContext(token string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestAuth_UnverifiedRejected(t *testing.T) {
	svc, repo, token := gateFixture(t, false)
	c, rec := gateContext(token)

	nextCalled := false
	err := Auth(svc, repo)(func(c echo.Context) error {
		nextCalled = true
		return nil
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextCalled {
		t.Fatal("next must not be called for an unverified user")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != "email_not_verified" || body["error"] == "" {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestAuth_VerifiedAllowed(t *testing.T) {
	svc, repo, token := gateFixture(t, true)
	c, _ := gateContext(token)

	nextCalled := false
	err := Auth(svc, repo)(func(c echo.Context) error {
		nextCalled = true
		if _, ok := c.Get("user").(*domain.User); !ok {
			t.Fatal("expected user in context")
		}
		return nil
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Fatal("next must be called for a verified user")
	}
}

func TestOptionalAuth_UnverifiedIsAnonymous(t *testing.T) {
	svc, repo, token := gateFixture(t, false)
	c, _ := gateContext(token)

	nextCalled := false
	err := OptionalAuth(svc, repo)(func(c echo.Context) error {
		nextCalled = true
		if c.Get("user") != nil || c.Get("user_id") != nil {
			t.Fatal("unverified user must be treated as anonymous")
		}
		return nil
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Fatal("next must be called")
	}
}

func TestOptionalAuth_VerifiedSetsUser(t *testing.T) {
	svc, repo, token := gateFixture(t, true)
	c, _ := gateContext(token)

	err := OptionalAuth(svc, repo)(func(c echo.Context) error {
		if _, ok := c.Get("user").(*domain.User); !ok {
			t.Fatal("expected user in context")
		}
		return nil
	})(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
