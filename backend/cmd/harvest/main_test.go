package main

import (
	"errors"
	"testing"
)

func TestBuildQuery(t *testing.T) {
	fakeRead := func(contents string, err error) func(string) ([]byte, error) {
		return func(path string) ([]byte, error) {
			if err != nil {
				return nil, err
			}
			return []byte(contents), nil
		}
	}

	t.Run("uses -query flag directly", func(t *testing.T) {
		query, err := buildQuery("SELECT * WHERE {}", "", fakeRead("", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if query != "SELECT * WHERE {}" {
			t.Errorf("query = %q", query)
		}
	})

	t.Run("reads -query-file and trims whitespace", func(t *testing.T) {
		query, err := buildQuery("", "example.rq", fakeRead("  SELECT * WHERE { ?s ?p ?o }  \n", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if query != "SELECT * WHERE { ?s ?p ?o }" {
			t.Errorf("query = %q", query)
		}
	})

	t.Run("errors when both flags set", func(t *testing.T) {
		_, err := buildQuery("a", "b.rq", fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("errors when neither flag set", func(t *testing.T) {
		_, err := buildQuery("", "", fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("errors when query file is empty", func(t *testing.T) {
		_, err := buildQuery("", "empty.rq", fakeRead("   \n", nil))
		if err == nil {
			t.Fatal("expected error for empty query file")
		}
	})

	t.Run("propagates file read errors", func(t *testing.T) {
		readErr := errors.New("no such file")
		_, err := buildQuery("", "missing.rq", fakeRead("", readErr))
		if !errors.Is(err, readErr) {
			t.Fatalf("expected wrapped read error, got %v", err)
		}
	})
}
