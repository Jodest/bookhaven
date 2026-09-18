package main

import (
	"errors"
	"strings"
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
		query, err := buildQuery("SELECT * WHERE {}", "", "", "", 0, false, fakeRead("", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if query != "SELECT * WHERE {}" {
			t.Errorf("query = %q", query)
		}
	})

	t.Run("reads -query-file and trims whitespace", func(t *testing.T) {
		query, err := buildQuery("", "example.rq", "", "", 0, false, fakeRead("  SELECT * WHERE { ?s ?p ?o }  \n", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if query != "SELECT * WHERE { ?s ?p ?o }" {
			t.Errorf("query = %q", query)
		}
	})

	t.Run("errors when both -query and -query-file set", func(t *testing.T) {
		_, err := buildQuery("a", "b.rq", "", "", 0, false, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("errors when no mode set", func(t *testing.T) {
		_, err := buildQuery("", "", "", "", 0, false, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("errors when query file is empty", func(t *testing.T) {
		_, err := buildQuery("", "empty.rq", "", "", 0, false, fakeRead("   \n", nil))
		if err == nil {
			t.Fatal("expected error for empty query file")
		}
	})

	t.Run("propagates file read errors", func(t *testing.T) {
		readErr := errors.New("no such file")
		_, err := buildQuery("", "missing.rq", "", "", 0, false, fakeRead("", readErr))
		if !errors.Is(err, readErr) {
			t.Fatalf("expected wrapped read error, got %v", err)
		}
	})

	t.Run("-authors builds a curated-author-list query", func(t *testing.T) {
		query, err := buildQuery("", "", "Q7200,Q1183296", "", 0, false, fakeRead("", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "wd:Q7200") || !strings.Contains(query, "wd:Q1183296") {
			t.Errorf("expected query to contain both QIDs, got: %s", query)
		}
	})

	t.Run("-authors-file builds a curated-author-list query, ignoring comments and blank lines", func(t *testing.T) {
		fileContents := "# comment\nQ7200\n\n  Q1183296  \n# another comment\n"
		query, err := buildQuery("", "", "", "authors.txt", 0, false, fakeRead(fileContents, nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "wd:Q7200") || !strings.Contains(query, "wd:Q1183296") {
			t.Errorf("expected query to contain both QIDs, got: %s", query)
		}
	})

	t.Run("-authors-file with only comments/blanks errors", func(t *testing.T) {
		_, err := buildQuery("", "", "", "authors.txt", 0, false, fakeRead("# only comments\n\n", nil))
		if err == nil {
			t.Fatal("expected error for authors file with no QIDs")
		}
	})

	t.Run("-limit applies to -authors mode", func(t *testing.T) {
		query, err := buildQuery("", "", "Q7200", "", 250, true, fakeRead("", nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(query, "LIMIT 250") {
			t.Errorf("expected LIMIT 250 in query, got: %s", query)
		}
	})

	t.Run("propagates invalid QID errors from bookdata.BuildAuthorListQuery", func(t *testing.T) {
		_, err := buildQuery("", "", "not-a-qid", "", 0, false, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error for malformed QID")
		}
	})

	t.Run("errors when -authors and -query both set", func(t *testing.T) {
		_, err := buildQuery("SELECT * WHERE {}", "", "Q7200", "", 0, false, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected mutual-exclusivity error")
		}
	})

	t.Run("errors when -authors and -authors-file both set", func(t *testing.T) {
		_, err := buildQuery("", "", "Q7200", "authors.txt", 0, false, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected mutual-exclusivity error")
		}
	})

	t.Run("errors when -limit combined with -query", func(t *testing.T) {
		_, err := buildQuery("SELECT * WHERE {}", "", "", "", 100, true, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error: -limit only meaningful with -authors/-authors-file")
		}
	})

	t.Run("errors when -limit combined with -query-file", func(t *testing.T) {
		_, err := buildQuery("", "example.rq", "", "", 100, true, fakeRead("SELECT * WHERE {}", nil))
		if err == nil {
			t.Fatal("expected error: -limit only meaningful with -authors/-authors-file")
		}
	})

	t.Run("-limit 0 explicitly set is not an error with -query (indistinguishable from default)", func(t *testing.T) {
		// limitSet=true but limit==0 combined with -query is still flagged,
		// since the flag is meaningless there regardless of its value.
		_, err := buildQuery("SELECT * WHERE {}", "", "", "", 0, true, fakeRead("", nil))
		if err == nil {
			t.Fatal("expected error: -limit only meaningful with -authors/-authors-file")
		}
	})
}

func TestSplitAuthorQIDs(t *testing.T) {
	got := splitAuthorQIDs(" Q7200 , Q1183296,, Q42 ")
	want := []string{"Q7200", "Q1183296", "Q42"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseAuthorQIDsFile(t *testing.T) {
	contents := "Q7200\n# comment\n\n  Q1183296  \n#another\nQ42\n"
	got := parseAuthorQIDsFile(contents)
	want := []string{"Q7200", "Q1183296", "Q42"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
