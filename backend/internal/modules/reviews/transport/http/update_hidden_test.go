package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/infrastructure/validator"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/modules/reviews/repository"
	"github.com/bookhaven/backend/internal/modules/reviews/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// hiddenReviewRepo serves one review by ID; any other repository call panics
// through the nil embedded interface, which proves the handler stops early.
type hiddenReviewRepo struct {
	repository.Repository
	review reviewsDomain.Review
}

func (r *hiddenReviewRepo) GetByID(_ context.Context, _ uuid.UUID) (*reviewsDomain.Review, error) {
	review := r.review
	return &review, nil
}

func patchReview(t *testing.T, review reviewsDomain.Review, callerID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(service.NewService(&hiddenReviewRepo{review: review}))
	e := echo.New()
	e.Validator = validator.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/reviews/"+review.ID.String(), strings.NewReader(`{"content":"Исправленный текст"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/reviews/:id")
	c.SetParamNames("id")
	c.SetParamValues(review.ID.String())
	c.Set("user_id", callerID.String())
	if err := h.UpdateReview(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return rec
}

func TestUpdateReview_HiddenOwnReviewIsConflict(t *testing.T) {
	author := uuid.New()
	rec := patchReview(t, reviewsDomain.Review{ID: uuid.New(), UserID: author, IsHidden: true}, author)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "review is hidden" {
		t.Fatalf("unexpected body %q (err %v)", rec.Body.String(), err)
	}
}

func TestUpdateReview_HiddenReviewOfAnotherUserIsForbidden(t *testing.T) {
	rec := patchReview(t, reviewsDomain.Review{ID: uuid.New(), UserID: uuid.New(), IsHidden: true}, uuid.New())

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
