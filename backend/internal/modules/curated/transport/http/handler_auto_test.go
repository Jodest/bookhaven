package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (v *env) shelved(t *testing.T, book, typ string) {
	t.Helper()
	shelf := uuid.NewString()
	if err := v.db.Exec("INSERT INTO shelves (id, user_id, name, type) VALUES (?, ?, ?, ?)", shelf, uuid.NewString(), typ, typ).Error; err != nil {
		t.Fatal(err)
	}
	if err := v.db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, book, time.Now()).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAdmin_CreateAutoWithDefaults(t *testing.T) {
	v := newEnv(t)
	b := v.book(t, "published")
	v.shelved(t, b, "read")
	code, out := v.do(http.MethodPost, "/admin/collections", `{"title":"Лучшее","size":5,"kind":"auto","rule":{"metric":"weighted_rating"}}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %v", code, out)
	}
	rule := out["rule"].(map[string]any)
	if out["kind"] != "auto" || rule["metric"] != "weighted_rating" || rule["min_ratings"].(float64) != 3 ||
		rule["confidence"].(float64) != 10 || rule["period_days"] != nil {
		t.Fatalf("rule: %v", out)
	}
	if out["computed_at"] == nil || out["baseline_at"] != nil {
		t.Fatalf("snapshot times: %v", out)
	}
	_, out = v.do(http.MethodGet, "/admin/collections", "")
	if out["items"].([]any)[0].(map[string]any)["computed_at"] == nil {
		t.Fatalf("list computed_at: %v", out)
	}
}

func TestAdmin_CreateAutoValidation(t *testing.T) {
	v := newEnv(t)
	cases := map[string]string{
		"rule: required for automatic collections": `{"title":"Лучшее","size":5,"kind":"auto"}`,
		"rule: only for automatic collections":     `{"title":"Лучшее","size":5,"rule":{"metric":"most_read"}}`,
		"rule.metric: unknown metric":              `{"title":"Лучшее","size":5,"kind":"auto","rule":{"metric":"zzz"}}`,
		"kind: must be manual or auto":             `{"title":"Лучшее","size":5,"kind":"smart"}`,
	}
	for want, body := range cases {
		code, out := v.do(http.MethodPost, "/admin/collections", body)
		if code != 400 || out["error"] != want {
			t.Errorf("%s: %d %v", want, code, out)
		}
	}
	if _, out := v.do(http.MethodGet, "/admin/collections", ""); out["total"].(float64) != 0 {
		t.Fatal("nothing must be created")
	}
}

func TestAdmin_PatchKindRejected(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Ручная","size":2}`)
	code, out := v.do(http.MethodPatch, "/admin/collections/"+id, `{"kind":"auto"}`)
	if code != 400 || out["error"] != "kind: cannot be changed" {
		t.Fatalf("patch kind: %d %v", code, out)
	}
}

func TestAdmin_RecomputeStatuses(t *testing.T) {
	v := newEnv(t)
	manual := v.collection(t, `{"title":"Ручная","size":2}`)
	auto := v.collection(t, `{"title":"Авто","size":2,"kind":"auto","rule":{"metric":"most_read"}}`)

	code, out := v.do(http.MethodPost, "/admin/collections/"+auto+"/recompute", "")
	if code != 200 || out["baseline_at"] == nil || out["computed_at"] == nil {
		t.Fatalf("recompute auto: %d %v", code, out)
	}
	code, out = v.do(http.MethodPost, "/admin/collections/"+manual+"/recompute", "")
	if code != 422 || out["error"] != "collection is not automatic" {
		t.Fatalf("recompute manual: %d %v", code, out)
	}
	code, out = v.do(http.MethodPost, "/admin/collections/"+uuid.NewString()+"/recompute", "")
	if code != 404 || out["error"] != "collection not found" {
		t.Fatalf("recompute unknown: %d %v", code, out)
	}
	if code, _ = v.do(http.MethodPost, "/admin/collections/bad/recompute", ""); code != 400 {
		t.Fatalf("bad id: %d", code)
	}
}

func TestAdmin_BookEditingOnAutoIs422(t *testing.T) {
	v := newEnv(t)
	id := v.collection(t, `{"title":"Авто","size":2,"kind":"auto","rule":{"metric":"most_read"}}`)
	b := v.book(t, "published")
	base := "/admin/collections/" + id
	calls := []struct{ method, path, body string }{
		{http.MethodPost, base + "/books", `{"book_id":"` + b + `"}`},
		{http.MethodPost, base + "/books/paste/preview", `{"text":"Мать"}`},
		{http.MethodPost, base + "/books/bulk", `{"book_ids":["` + b + `"]}`},
		{http.MethodDelete, base + "/books/" + b, ""},
		{http.MethodPut, base + "/books/" + b + "/position", `{"position":1}`},
	}
	for _, c := range calls {
		code, out := v.do(c.method, c.path, c.body)
		if code != 422 || out["error"] != "collection is automatic" {
			t.Errorf("%s %s: %d %v", c.method, c.path, code, out)
		}
	}
}

func TestPublic_MovementNullForManualAndSetForAuto(t *testing.T) {
	v := newEnv(t)
	mid := v.collection(t, `{"title":"Ручная","size":2,"slug":"manual","is_published":true}`)
	mb := v.book(t, "published")
	if code, out := v.do(http.MethodPost, "/admin/collections/"+mid+"/books", `{"book_id":"`+mb+`"}`); code != 201 {
		t.Fatalf("add: %d %v", code, out)
	}
	_, out := v.do(http.MethodGet, "/collections/manual", "")
	item := out["items"].([]any)[0].(map[string]any)
	if mv, ok := item["movement"]; !ok || mv != nil {
		t.Fatalf("manual movement must be null: %v", item)
	}
	if coll := out["collection"].(map[string]any); coll["rule"] != nil || coll["computed_at"] != nil {
		t.Fatalf("manual rule/computed_at: %v", coll)
	}

	a, b := v.book(t, "published"), v.book(t, "published")
	v.shelved(t, a, "read")
	v.shelved(t, a, "read")
	v.shelved(t, b, "read")
	aid := v.collection(t, `{"title":"Авто","size":5,"slug":"auto","is_published":true,"kind":"auto","rule":{"metric":"most_read"}}`)
	_, out = v.do(http.MethodGet, "/collections/auto", "")
	if out["items"].([]any)[0].(map[string]any)["movement"] != nil {
		t.Fatal("no baseline yet: movement must be null")
	}
	coll := out["collection"].(map[string]any)
	if coll["rule"].(map[string]any)["metric"] != "most_read" || coll["computed_at"] == nil {
		t.Fatalf("auto collection: %v", coll)
	}

	// b climbs past a, c is new.
	c := v.book(t, "published")
	v.shelved(t, c, "read")
	v.shelved(t, b, "read")
	v.shelved(t, b, "read")
	if code, o := v.do(http.MethodPost, "/admin/collections/"+aid+"/recompute", ""); code != 200 {
		t.Fatalf("recompute: %d %v", code, o)
	}
	_, out = v.do(http.MethodGet, "/collections/auto", "")
	items := out["items"].([]any)
	first := items[0].(map[string]any)
	if first["book"].(map[string]any)["id"] != b {
		t.Fatalf("order: %v", items)
	}
	if mv := first["movement"].(map[string]any); mv["direction"] != "up" || mv["by"].(float64) != 1 {
		t.Fatalf("up movement: %v", mv)
	}
	if mv := items[2].(map[string]any)["movement"].(map[string]any); mv["direction"] != "new" {
		t.Fatalf("new movement: %v", mv)
	}

	// The admin detail carries score and movement.
	_, out = v.do(http.MethodGet, "/admin/collections/"+aid, "")
	ai := out["items"].([]any)[0].(map[string]any)
	if ai["score"].(float64) != 3 || ai["movement"].(map[string]any)["direction"] != "up" || out["baseline_at"] == nil {
		t.Fatalf("admin detail: %v", out)
	}
}
