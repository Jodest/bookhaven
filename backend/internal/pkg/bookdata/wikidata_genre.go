package bookdata

import (
	"context"
	"fmt"
	"strings"
)

// Genre import from Wikidata (docs/features/genre-wikidata-import.md).

// genreSearchLimit is the wbsearchentities limit of SearchGenres: twice the
// 10 candidates the admin sees, leaving room for the literary-genre filter.
const genreSearchLimit = 20

// MaxGenresPerClassification caps the QIDs of one ClassifyGenres query.
const MaxGenresPerClassification = 20

// LiteraryGenreRoots are the classes whose (transitive) instances or
// subclasses are literary genres: literary genre (Q223393) and literary
// form (Q4263830), both verified against live Wikidata. Genre (Q483394) is
// deliberately absent: walking up to it accepts music and film genres.
// The admin service also checks an entity's direct P31/P279 against these
// roots before spending a SPARQL query.
var LiteraryGenreRoots = []string{"Q223393", "Q4263830"}

// fictionGenreClass is fiction genre (Q108465955, "genre of fiction"):
// fantasy, detective fiction, thriller and science fiction reach it, not a
// literary root. It is accepted unless the item also reaches one of
// nonLiteraryGenreRoots.
const fictionGenreClass = "Q108465955"

// nonLiteraryGenreRoots are music genre (Q188451) and film genre
// (Q201658). An item reaching them is accepted only through
// LiteraryGenreRoots, never through fictionGenreClass.
var nonLiteraryGenreRoots = []string{"Q188451", "Q201658"}

// GenreCandidate is one Wikidata item returned by SearchGenres.
type GenreCandidate struct {
	QID         string
	Label       string
	Description string // may be empty
}

// SearchGenres finds candidate Wikidata items by name with
// wbsearchentities (language=ru, limit 20); see searchEntities. Like
// SearchAuthors it does not filter by type: ClassifyGenres does. Zero
// matches is an empty slice and a nil error.
func (c *WikidataClient) SearchGenres(ctx context.Context, query string) ([]GenreCandidate, error) {
	hits, err := c.searchEntities(ctx, query, genreSearchLimit)
	if err != nil {
		return nil, err
	}
	candidates := make([]GenreCandidate, 0, len(hits))
	for _, h := range hits {
		candidates = append(candidates, GenreCandidate(h))
	}
	return candidates, nil
}

// GenreClassification is the ClassifyGenres verdict for one QID.
type GenreClassification struct {
	// RuLabel is the item's Russian label, "" when it has none.
	RuLabel string
	// IsLiteraryGenre: the item reaches a literary root, or it reaches
	// fiction genre and no music/film genre root.
	IsLiteraryGenre bool
}

// ClassifyGenres classifies up to MaxGenresPerClassification QIDs in one
// SPARQL query on cfg.baseURL (via doRequest), returning each item's ru
// label and literary-genre verdict keyed by QID. Every class path is
// walked upward from a bound item (never down a root's subtree), so the
// query stays small. An empty, oversized or invalid list returns
// ErrInvalidQuery without an HTTP call; a QID absent from the bindings is
// not a literary genre. Status mapping matches Query.
func (c *WikidataClient) ClassifyGenres(ctx context.Context, qids []string) (map[string]GenreClassification, error) {
	if len(qids) == 0 || len(qids) > MaxGenresPerClassification {
		return nil, ErrInvalidQuery
	}
	unique := make([]string, 0, len(qids))
	seen := make(map[string]struct{}, len(qids))
	for _, q := range qids {
		q = strings.TrimSpace(q)
		if !wikidataQIDPattern.MatchString(q) {
			return nil, ErrInvalidQuery
		}
		if _, dup := seen[q]; dup {
			continue
		}
		seen[q] = struct{}{}
		unique = append(unique, q)
	}

	bindings, err := c.sparqlBindings(ctx, buildGenreClassificationQuery(unique))
	if err != nil {
		return nil, err
	}

	out := make(map[string]GenreClassification, len(unique))
	for _, q := range unique {
		out[q] = GenreClassification{}
	}
	for _, b := range bindings {
		item, ok := b["item"]
		if !ok {
			continue
		}
		qid := extractWikidataID(item.Value)
		if _, wanted := out[qid]; !wanted {
			continue
		}
		byClass := sparqlBool(b["byClass"])
		byFiction := sparqlBool(b["byFiction"])
		nonLiterary := sparqlBool(b["nonLiterary"])
		out[qid] = GenreClassification{
			RuLabel:         b["ruLabel"].Value,
			IsLiteraryGenre: byClass || (byFiction && !nonLiterary),
		}
	}
	return out, nil
}

// buildGenreClassificationQuery builds the classification query for
// already validated QIDs (no user text reaches the query).
func buildGenreClassificationQuery(qids []string) string {
	return fmt.Sprintf(`SELECT ?item ?ruLabel ?byClass ?byFiction ?nonLiterary WHERE {
  VALUES ?item { %s }
  OPTIONAL { ?item rdfs:label ?ruLabel . FILTER(LANG(?ruLabel) = "ru") }
  BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?root . FILTER(?root IN (%s)) } AS ?byClass)
  BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?fiction . FILTER(?fiction = %s) } AS ?byFiction)
  BIND(EXISTS { ?item (wdt:P31|wdt:P279)/wdt:P279* ?other . FILTER(?other IN (%s)) } AS ?nonLiterary)
}`, strings.Join(prefixWD(qids), " "),
		strings.Join(prefixWD(LiteraryGenreRoots), ", "),
		"wd:"+fictionGenreClass,
		strings.Join(prefixWD(nonLiteraryGenreRoots), ", "))
}

func prefixWD(qids []string) []string {
	out := make([]string, 0, len(qids))
	for _, q := range qids {
		out = append(out, "wd:"+q)
	}
	return out
}

// sparqlBool parses an xsd:boolean literal ("true"/"1").
func sparqlBool(v sparqlValue) bool {
	return v.Value == "true" || v.Value == "1"
}

// ExternalGenreRecord is a single Wikidata entity fetched by FetchGenre.
type ExternalGenreRecord struct {
	// QID is the returned entity's id - canonical after a redirect.
	QID string
	// Name is the Russian label, "" when none (no language fallback).
	Name string
	// InstanceOf / SubclassOf are the valid QIDs of the P31 / P279 claims.
	InstanceOf []string
	SubclassOf []string
}

// FetchGenre fetches one entity with wbgetentities (props=labels|claims,
// languages=ru) on cfg.searchBaseURL. Error mapping matches FetchAuthor:
// an invalid QID → ErrInvalidQuery without a call, a missing entity →
// ErrNotFound; a redirect yields the canonical QID.
func (c *WikidataClient) FetchGenre(ctx context.Context, qid string) (*ExternalGenreRecord, error) {
	qid = strings.TrimSpace(qid)
	if !wikidataQIDPattern.MatchString(qid) {
		return nil, ErrInvalidQuery
	}
	entity, err := c.getEntity(ctx, qid)
	if err != nil {
		return nil, err
	}
	if entity.Missing != nil || !wikidataQIDPattern.MatchString(entity.ID) {
		return nil, ErrNotFound
	}
	record := &ExternalGenreRecord{
		QID:        entity.ID,
		InstanceOf: entityIDClaims(entity.Claims["P31"]),
		SubclassOf: entityIDClaims(entity.Claims["P279"]),
	}
	if label, ok := entity.Labels["ru"]; ok {
		record.Name = label.Value
	}
	return record, nil
}
