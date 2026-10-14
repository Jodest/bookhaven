package http

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// titled inserts a book with a lower-case title (the SQLite fallback only
// folds ASCII case) and an optional author.
func (v *env) titled(t *testing.T, title, status, author string) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now()
	if err := v.db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", id, title, id, status, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if author != "" {
		aid := uuid.NewString()
		if err := v.db.Exec("INSERT INTO authors (id, name, slug, status) VALUES (?, ?, ?, 'published')", aid, author, aid).Error; err != nil {
			t.Fatal(err)
		}
		if err := v.db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", id, aid).Error; err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestPastePreview_ReportAndErrors(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Топ","size":3,"slug":"top"}`)
	gorky := v.titled(t, "мать", "published", "Максим Горький")
	base := "/admin/collections/" + id + "/books/paste/preview"

	code, out := v.do(http.MethodPost, base, `{"text":"1. Мать — Горький\n\nНет такой"}`)
	if code != 200 {
		t.Fatalf("preview: %d %v", code, out)
	}
	lines := out["lines"].([]any)
	first, second := lines[0].(map[string]any), lines[1].(map[string]any)
	if len(lines) != 2 || first["status"] != "found" || first["line"].(float64) != 1 ||
		first["book"].(map[string]any)["id"] != gorky || second["status"] != "not_found" || second["line"].(float64) != 3 {
		t.Fatalf("lines: %v", lines)
	}
	if second["book"] != nil || len(second["candidates"].([]any)) != 0 || second["duplicate_of"] != nil {
		t.Fatalf("not_found shape: %v", second)
	}
	counts := out["counts"].(map[string]any)
	if counts["found"].(float64) != 1 || counts["not_found"].(float64) != 1 || out["free_slots"].(float64) != 1000 {
		t.Fatalf("counts: %v", out)
	}
	// preview writes nothing
	if _, got := v.do(http.MethodGet, "/admin/collections/"+id, ""); got["books_count"].(float64) != 0 {
		t.Fatalf("collection changed: %v", got)
	}

	if code, out := v.do(http.MethodPost, base, `{"text":"  "}`); code != 400 || !strings.HasPrefix(out["error"].(string), "text:") {
		t.Errorf("blank: %d %v", code, out)
	}
	if code, out := v.do(http.MethodPost, base, `{"text":"`+strings.Repeat("я", 100001)+`"}`); code != 400 || !strings.HasPrefix(out["error"].(string), "text:") {
		t.Errorf("long: %d %v", code, out)
	}
	if code, out := v.do(http.MethodPost, base, `{"text":"`+strings.Repeat(`а\n`, 501)+`"}`); code != 422 || out["error"] != "too many lines (max 500)" {
		t.Errorf("too many: %d %v", code, out)
	}
	if code, out := v.do(http.MethodPost, "/admin/collections/"+uuid.NewString()+"/books/paste/preview", `{"text":"а"}`); code != 404 || out["error"] != "collection not found" {
		t.Errorf("unknown: %d %v", code, out)
	}
	if code, _ := v.do(http.MethodPost, "/admin/collections/not-a-uuid/books/paste/preview", `{"text":"а"}`); code != 400 {
		t.Errorf("bad uuid: %d", code)
	}
}

func TestBulkAdd_OrderSkipsAndErrors(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Топ","size":5,"slug":"top"}`)
	base := "/admin/collections/" + id + "/books/bulk"
	a, b, c := v.book(t, "published"), v.book(t, "pending"), v.book(t, "published")
	rejected := v.book(t, "rejected")
	if code, _ := v.do(http.MethodPost, "/admin/collections/"+id+"/books", `{"book_id":"`+a+`"}`); code != 201 {
		t.Fatal("seed")
	}

	body := `{"book_ids":["` + b + `","` + rejected + `","` + a + `","` + c + `"],"position":1}`
	code, out := v.do(http.MethodPost, base, body)
	if code != 200 {
		t.Fatalf("bulk: %d %v", code, out)
	}
	added := out["added"].([]any)
	if len(added) != 2 || added[0].(map[string]any)["book_id"] != b || added[0].(map[string]any)["position"].(float64) != 1 ||
		added[1].(map[string]any)["book_id"] != c || added[1].(map[string]any)["position"].(float64) != 2 || out["books_count"].(float64) != 3 {
		t.Fatalf("added: %v", out)
	}
	skipped := out["skipped"].([]any)
	if len(skipped) != 2 || skipped[0].(map[string]any)["reason"] != "rejected" || skipped[1].(map[string]any)["reason"] != "already_in_collection" {
		t.Fatalf("skipped: %v", skipped)
	}

	for name, tc := range map[string]string{
		"empty":        `{"book_ids":[]}`,
		"bad uuid":     `{"book_ids":["x"]}`,
		"bad position": `{"book_ids":["` + a + `"],"position":0}`,
		"too many":     `{"book_ids":["` + strings.Repeat(a+`","`, 500) + a + `"]}`,
	} {
		if code, out := v.do(http.MethodPost, base, tc); code != 400 {
			t.Errorf("%s: %d %v", name, code, out)
		}
	}
	if code, _ := v.do(http.MethodPost, "/admin/collections/"+uuid.NewString()+"/books/bulk", `{"book_ids":["`+a+`"]}`); code != 404 {
		t.Errorf("unknown collection: %d", code)
	}
}

func TestBulkAdd_FullIs422AndWritesNothing(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Топ","size":5,"slug":"top"}`)
	now := time.Now()
	for i := 0; i < 995; i++ {
		if err := v.db.Exec("INSERT INTO curated_collection_items (collection_id, book_id, position, added_at) VALUES (?, ?, ?, ?)",
			id, uuid.NewString(), i+1, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids := make([]string, 6)
	for i := range ids {
		ids[i] = `"` + v.book(t, "published") + `"`
	}
	code, out := v.do(http.MethodPost, "/admin/collections/"+id+"/books/bulk", `{"book_ids":[`+strings.Join(ids, ",")+`]}`)
	if code != 422 || out["error"] != "collection is full" {
		t.Fatalf("full: %d %v", code, out)
	}
	var n int64
	v.db.Table("curated_collection_items").Where("collection_id = ?", id).Count(&n)
	if n != 995 {
		t.Fatalf("items = %d", n)
	}
}
