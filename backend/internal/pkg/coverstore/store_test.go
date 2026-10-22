package coverstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func name(hexChar string, ext string) string {
	h := strings.Repeat(hexChar, 64)
	return h[:2] + "/" + h + "." + ext
}

func TestValidName(t *testing.T) {
	good := []string{name("a", "jpg"), name("0", "png"), name("f", "webp")}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("%q must be valid", n)
		}
	}
	bad := []string{
		"", "../x", "../../etc/passwd", name("A", "jpg"), name("a", "gif"), name("a", "jpeg"),
		"ab/" + strings.Repeat("a", 63) + ".jpg", "ab/" + strings.Repeat("c", 64) + ".jpg", // shard mismatch
		"/" + name("a", "jpg"), name("a", "jpg") + "/", "aa/../" + strings.Repeat("a", 64) + ".jpg",
		strings.ReplaceAll(name("a", "jpg"), "/", `\`), ".bookhaven-covers",
	}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("%q must be invalid", n)
		}
	}
}

func contract(t *testing.T, s Store) {
	ctx := context.Background()
	n1, n2 := name("a", "jpg"), name("b", "png")

	if ok, err := s.Exists(ctx, n1); err != nil || ok {
		t.Fatalf("exists before put: %v %v", ok, err)
	}
	if _, _, err := s.Open(ctx, n1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open missing: %v", err)
	}
	if err := s.Put(ctx, n1, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, n2, []byte("two")); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Exists(ctx, n1); !ok {
		t.Fatal("exists after put")
	}
	r, info, err := s.Open(ctx, n1)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	_ = r.Close()
	if string(b) != "one" || info.Size != 3 || info.ModTime.IsZero() {
		t.Fatalf("open: %q %+v", b, info)
	}
	// Overwrite is allowed (content-addressed, idempotent).
	if err := s.Put(ctx, n1, []byte("one")); err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	if err := s.List(ctx, func(n string, mt time.Time) error {
		if mt.IsZero() {
			t.Errorf("zero modtime for %s", n)
		}
		got[n] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[n1] || !got[n2] {
		t.Fatalf("list: %v", got)
	}

	if err := s.Delete(ctx, n1); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, n1); err != nil {
		t.Fatalf("delete missing must be nil: %v", err)
	}
	if ok, _ := s.Exists(ctx, n1); ok {
		t.Fatal("exists after delete")
	}

	for _, bad := range []string{"../x", name("A", "jpg"), name("a", "gif"), ""} {
		if err := s.Put(ctx, bad, []byte("x")); err == nil {
			t.Errorf("put %q must fail", bad)
		}
		if _, _, err := s.Open(ctx, bad); err == nil {
			t.Errorf("open %q must fail", bad)
		}
		if err := s.Delete(ctx, bad); err == nil {
			t.Errorf("delete %q must fail", bad)
		}
	}

	created, err := s.EnsureMarker(ctx)
	if err != nil || !created {
		t.Fatalf("first marker: %v %v", created, err)
	}
	created, err = s.EnsureMarker(ctx)
	if err != nil || created {
		t.Fatalf("second marker: %v %v", created, err)
	}
	// The marker is never listed as a cover.
	count := 0
	_ = s.List(ctx, func(string, time.Time) error { count++; return nil })
	if count != 1 {
		t.Fatalf("list after marker: %d", count)
	}
}

func TestMemStoreContract(t *testing.T) { contract(t, NewMem()) }

func TestFSStoreContract(t *testing.T) { contract(t, NewFS(t.TempDir()+"/covers")) }

func TestFSStoreListMissingRoot(t *testing.T) {
	s := NewFS(t.TempDir() + "/nope")
	if err := s.List(context.Background(), func(string, time.Time) error { t.Fatal("unexpected"); return nil }); err != nil {
		t.Fatal(err)
	}
}
