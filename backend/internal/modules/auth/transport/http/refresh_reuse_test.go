package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
)

// Handler tests for refresh token reuse detection (auth/session spec).

func TestRefreshHandler_RotatedTokenCodes(t *testing.T) {
	cases := []struct {
		name     string
		ago      time.Duration
		wantCode string
	}{
		{name: "within grace window", ago: 5 * time.Second, wantCode: "refresh_token_superseded"},
		{name: "after grace window", ago: time.Minute, wantCode: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupVerificationHandler(t)
			user := env.registerUser(t, "petr@example.com")
			env.verify(t, "petr@example.com")
			issued, err := env.svc.CreateTokenPair(context.Background(), user.ID)
			if err != nil {
				t.Fatalf("create token pair: %v", err)
			}

			c, rec := postJSON("/api/v1/auth/refresh", `{"refresh_token":"`+issued.RefreshToken+`"}`)
			if err := env.h.Refresh(c); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("first refresh: err=%v status=%d body=%s", err, rec.Code, rec.Body.String())
			}
			if err := env.db.Model(&domain.RefreshToken{}).
				Where("revoked_at IS NOT NULL").
				Update("revoked_at", time.Now().Add(-tc.ago).Unix()).Error; err != nil {
				t.Fatalf("backdate: %v", err)
			}

			c, rec = postJSON("/api/v1/auth/refresh", `{"refresh_token":"`+issued.RefreshToken+`"}`)
			if err := env.h.Refresh(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body %s)", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if tc.wantCode == "" {
				if _, ok := body["code"]; ok {
					t.Fatalf("expected no code, got %s", rec.Body.String())
				}
				if body["error"] != "invalid or expired refresh token" {
					t.Fatalf("unexpected body %s", rec.Body.String())
				}
			} else if body["code"] != tc.wantCode {
				t.Fatalf("code = %v, want %s", body["code"], tc.wantCode)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("cookies must be untouched")
			}
		})
	}
}
