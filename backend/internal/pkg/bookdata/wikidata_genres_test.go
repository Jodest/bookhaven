package bookdata

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Genre QID pairing (docs/features/genre-status.md).

func TestParseGenrePairs(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []ExternalGenre
	}{
		{
			name:  "several paired genres",
			value: "Q8261\tроман|Q1196408\tрусская литература|Q49084\tрассказ",
			want:  []ExternalGenre{{Name: "роман", QID: "Q8261"}, {Name: "русская литература", QID: "Q1196408"}, {Name: "рассказ", QID: "Q49084"}},
		},
		{
			name:  "label with spaces and punctuation stays paired",
			value: "Q1\tнаучная фантастика (жанр), т. е. НФ|Q2\tроман-эпопея",
			want:  []ExternalGenre{{Name: "научная фантастика (жанр), т. е. НФ", QID: "Q1"}, {Name: "роман-эпопея", QID: "Q2"}},
		},
		{
			name:  "missing QID keeps name",
			value: "\tбез кода|просто жанр",
			want:  []ExternalGenre{{Name: "без кода"}, {Name: "просто жанр"}},
		},
		{
			name:  "invalid QID dropped, name kept",
			value: "http://x/Q1\tроман|q5\tповесть",
			want:  []ExternalGenre{{Name: "роман"}, {Name: "повесть"}},
		},
		{
			name:  "empty aggregate",
			value: "",
			want:  []ExternalGenre{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGenrePairs(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWikidataQuery_PairedGenres_PopulatesGenresAndRefs(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, `{
  "results": {"bindings": [
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q1"},
      "itemLabel": {"type":"literal","value":"Война и мир"},
      "genres": {"type":"literal","value":"Q8261\tроман|bad\tроман-эпопея"}
    },
    {
      "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q2"},
      "itemLabel": {"type":"literal","value":"Без жанров"},
      "genres": {"type":"literal","value":""}
    }
  ]}
}`, nil), nil
	})
	client := newWikidataTestClient(t, responder)

	records, err := client.Query(context.Background(), "SELECT * WHERE { ?s ?p ?o }")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	first := records[0]
	wantRefs := []ExternalGenre{{Name: "роман", QID: "Q8261"}, {Name: "роман-эпопея"}}
	if len(first.GenreRefs) != 2 || first.GenreRefs[0] != wantRefs[0] || first.GenreRefs[1] != wantRefs[1] {
		t.Errorf("GenreRefs = %+v, want %+v", first.GenreRefs, wantRefs)
	}
	if len(first.Genres) != 2 || first.Genres[0] != "роман" || first.Genres[1] != "роман-эпопея" {
		t.Errorf("Genres = %v, want names of GenreRefs", first.Genres)
	}
	if len(records[1].Genres) != 0 || len(records[1].GenreRefs) != 0 {
		t.Errorf("empty aggregate: Genres = %v, GenreRefs = %v", records[1].Genres, records[1].GenreRefs)
	}
}

func TestQueryBuilders_UsePairedGenreAggregate(t *testing.T) {
	const paired = `GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?genre), "/entity/"), "\t", ?genreLabel); separator="|") AS ?genres`
	const ruFilter = `?genre rdfs:label ?genreLabel .
    FILTER(LANG(?genreLabel) = "ru")`
	listQuery, err := BuildAuthorListQuery([]string{"Q7243"}, 10)
	if err != nil {
		t.Fatalf("BuildAuthorListQuery: %v", err)
	}
	queries := map[string]string{
		"title+author": buildTitleAuthorQuery("Война и мир", "Толстой"),
		"title only":   buildTitleAuthorQuery("Война и мир", ""),
		"isbn":         buildISBNQuery("9780140447934"),
		"author list":  listQuery,
	}
	for name, q := range queries {
		if !strings.Contains(q, paired) {
			t.Errorf("%s query lacks paired genre aggregate:\n%s", name, q)
		}
		if !strings.Contains(q, ruFilter) {
			t.Errorf("%s query lacks the ru genre label filter:\n%s", name, q)
		}
		if strings.Contains(q, `GROUP_CONCAT(DISTINCT ?genreLabel`) {
			t.Errorf("%s query still has the label-only genre aggregate", name)
		}
	}
}

func TestOpenLibraryRecords_LeaveGenreRefsEmpty(t *testing.T) {
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusOK, openLibrarySearchFixture, nil), nil
	})
	client := newOpenLibraryTestClient(t, responder)

	record, err := client.FindByTitleAuthor(context.Background(), "War and Peace", "Tolstoy")
	if err != nil {
		t.Fatalf("FindByTitleAuthor: %v", err)
	}
	if len(record.GenreRefs) != 0 {
		t.Errorf("GenreRefs = %+v, want empty for Open Library", record.GenreRefs)
	}
}
