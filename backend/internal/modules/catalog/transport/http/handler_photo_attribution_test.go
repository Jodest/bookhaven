package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func getAuthorJSON(t *testing.T, author domain.Author) map[string]interface{} {
	t.Helper()
	h, repo := setupHandler()
	repo.authors = []domain.Author{author}

	c, rec := newContext(http.MethodGet, "/api/v1/authors/"+author.Slug)
	c.SetPath("/api/v1/authors/:slug")
	c.SetParamNames("slug")
	c.SetParamValues(author.Slug)
	if err := h.GetAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestGetAuthor_PhotoAttribution(t *testing.T) {
	str := func(s string) *string { return &s }

	t.Run("present with commons source", func(t *testing.T) {
		body := getAuthorJSON(t, domain.Author{
			ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy",
			PhotoURL:        str("https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg"),
			PhotoAuthor:     str("Grondin"),
			PhotoLicense:    str("CC BY-SA 3.0"),
			PhotoLicenseURL: str("https://creativecommons.org/licenses/by-sa/3.0"),
			PhotoSourceURL:  str("https://commons.wikimedia.org/wiki/File:T.jpg"),
		})
		attr, ok := body["photo_attribution"].(map[string]interface{})
		if !ok {
			t.Fatalf("photo_attribution missing: %v", body)
		}
		if attr["author"] != "Grondin" || attr["license"] != "CC BY-SA 3.0" ||
			attr["license_url"] != "https://creativecommons.org/licenses/by-sa/3.0" ||
			attr["source_url"] != "https://commons.wikimedia.org/wiki/File:T.jpg" {
			t.Fatalf("unexpected attribution: %v", attr)
		}
	})

	for name, author := range map[string]domain.Author{
		"manual photo": {ID: uuid.New(), Name: "А", Slug: "a", PhotoURL: str("https://example.com/p.jpg")},
		"no photo":     {ID: uuid.New(), Name: "Б", Slug: "b", PhotoSourceURL: str("https://commons.wikimedia.org/wiki/File:T.jpg")},
	} {
		t.Run(name+" → null", func(t *testing.T) {
			body := getAuthorJSON(t, author)
			if v, ok := body["photo_attribution"]; !ok || v != nil {
				t.Fatalf("expected photo_attribution:null, got %v", body)
			}
		})
	}
}
