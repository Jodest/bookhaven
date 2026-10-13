package http

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestUpdateAuthor_AutoHarvest(t *testing.T) {
	stamp := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		withQID  bool
		initial  bool
		body     string
		wantCode int
		wantFlag bool
		wantNil  bool
	}{
		{name: "set with QID", withQID: true, body: `{"name":"A","auto_harvest":true}`, wantCode: http.StatusOK, wantFlag: true},
		{name: "clear", withQID: true, initial: true, body: `{"name":"A","auto_harvest":false}`, wantCode: http.StatusOK, wantFlag: false},
		{name: "omitted keeps", withQID: true, initial: true, body: `{"name":"A"}`, wantCode: http.StatusOK, wantFlag: true, wantNil: true},
		{name: "set without QID", body: `{"name":"A","auto_harvest":true}`, wantCode: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			a := catalogDomain.Author{ID: id, Name: "A", Slug: "a", Status: catalogDomain.AuthorStatusPublished, AutoHarvest: tt.initial, LastAutoHarvestedAt: &stamp}
			if tt.withQID {
				src, qid := catalogDomain.AuthorExternalSourceWikidata, "Q7243"
				a.ExternalSource, a.ExternalID = &src, &qid
			}
			svc.authors = []catalogDomain.Author{a}

			c, rec := newContext(http.MethodPatch, "/api/v1/admin/authors/"+id.String(), []byte(tt.body))
			c.SetPath("/api/v1/admin/authors/:id")
			c.SetParamNames("id")
			c.SetParamValues(id.String())
			setAdminUser(c)

			if err := h.UpdateAuthor(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantNil && svc.lastAuthorInput.AutoHarvest != nil {
				t.Fatalf("expected nil auto_harvest input, got %v", *svc.lastAuthorInput.AutoHarvest)
			}
			if tt.wantCode != http.StatusOK {
				var body map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "author has no Wikidata QID" {
					t.Fatalf("unexpected error body %s", rec.Body.String())
				}
				return
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if body["auto_harvest"] != tt.wantFlag {
				t.Fatalf("auto_harvest = %v, want %v", body["auto_harvest"], tt.wantFlag)
			}
			if body["last_auto_harvested_at"] != "2026-10-13T04:00:00Z" {
				t.Fatalf("last_auto_harvested_at = %v", body["last_auto_harvested_at"])
			}
		})
	}
}

func TestAuthorResponse_NeverAutoHarvested(t *testing.T) {
	raw, err := json.Marshal(toAuthorResponse(catalogDomain.Author{ID: uuid.New(), Name: "A", Slug: "a"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := body["last_auto_harvested_at"]; !ok || v != nil {
		t.Fatalf("last_auto_harvested_at must be present and null, got %v (present=%v)", v, ok)
	}
	if body["auto_harvest"] != false {
		t.Fatalf("auto_harvest = %v, want false", body["auto_harvest"])
	}
}

func TestCreateAuthor_IgnoresAutoHarvest(t *testing.T) {
	h, svc := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors", []byte(`{"name":"Tolstoy","auto_harvest":true}`))
	setAdminUser(c)

	if err := h.CreateAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if svc.lastCreateAuthorInput == nil || svc.lastCreateAuthorInput.AutoHarvest != nil {
		t.Fatalf("create must not pass auto_harvest to the service, got %+v", svc.lastCreateAuthorInput)
	}
}
