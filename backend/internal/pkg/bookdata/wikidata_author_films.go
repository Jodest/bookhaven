package bookdata

import (
	"context"
	"fmt"
	"strings"
)

// Films about authors from Wikidata (openspec change add-author-screen-works,
// catalog-import/wikidata-author-film-sync). A film about an author is an
// allowed video work whose main subject (P921) is the author.

// Relations of an author film.
const (
	AuthorFilmRelationBiopic      = "biopic"
	AuthorFilmRelationDocumentary = "documentary"
)

// MaxAuthorFilmAuthors is the number of author QIDs one FetchAuthorFilms call
// accepts (one SPARQL query).
const MaxAuthorFilmAuthors = MaxEntitiesPerRequest

// Extra classes for author films, verified live against Wikidata
// (query.wikidata.org / wbsearchentities, 2026-10-15; label - description):
//
//	Q93204      documentary film - nonfictional motion picture (used as P31
//	            and as P136; e.g. Q121819143 "My Pushkin" is P31 Q11424 with
//	            P136 Q93204)
//	Q645928     biographical film - film genre; dramatizes the life of an
//	            actual person or people (Q42689885 "Tolkien" and Q1343175
//	            "Magic Beyond Words" carry it as P136)
//	Q133731263  documentary television film - television film genre
//	Q133731271  documentary television series - television series genre
//	Q7603925    documentary television program - genre of television program
//	            (documentary set only; not an allowed class)
//
// There is no separate "television documentary" item in Wikidata, so
// Q133731263 stands for it.
var (
	// authorFilmExtraKinds are lower-priority fallbacks, tried after
	// screenKindPriority.
	authorFilmExtraKinds = []struct {
		kind    string
		classes []string
	}{
		{ScreenKindSeries, []string{"Q133731271"}},
		{ScreenKindTVFilm, []string{"Q133731263"}},
		{ScreenKindFilm, []string{"Q93204", "Q645928"}},
	}
	documentaryClasses = map[string]struct{}{
		"Q93204": {}, "Q133731263": {}, "Q133731271": {}, "Q7603925": {},
	}
)

// ExternalAuthorFilm is one film about an author found in Wikidata.
type ExternalAuthorFilm struct {
	ExternalScreenWork
	Relation string // AuthorFilmRelation*
}

// allowedAuthorFilmClasses lists every class that may make an item a film
// about an author (the SPARQL VALUES list is generated from it).
func allowedAuthorFilmClasses() []string {
	var out []string
	for _, k := range screenKindPriority {
		out = append(out, k.classes...)
	}
	for _, k := range authorFilmExtraKinds {
		out = append(out, k.classes...)
	}
	return out
}

// classifyAuthorFilm maps the P31 classes and P136 genres of an item to a
// screen kind and an author relation. ok is false when the item has an
// excluded class or no allowed class.
func classifyAuthorFilm(classes, genres []string) (kind, relation string, ok bool) {
	set := make(map[string]struct{}, len(classes))
	relation = AuthorFilmRelationBiopic
	for _, c := range classes {
		c = strings.TrimSpace(c)
		if _, excluded := screenExcludedClasses[c]; excluded {
			return "", "", false
		}
		set[c] = struct{}{}
		if _, doc := documentaryClasses[c]; doc {
			relation = AuthorFilmRelationDocumentary
		}
	}
	for _, g := range genres {
		if _, doc := documentaryClasses[strings.TrimSpace(g)]; doc {
			relation = AuthorFilmRelationDocumentary
		}
	}
	for _, k := range screenKindPriority {
		for _, c := range k.classes {
			if _, has := set[c]; has {
				return k.kind, relation, true
			}
		}
	}
	for _, k := range authorFilmExtraKinds {
		for _, c := range k.classes {
			if _, has := set[c]; has {
				return k.kind, relation, true
			}
		}
	}
	return "", "", false
}

func buildAuthorFilmsQuery(authorQIDs []string) string {
	var authors, allowed strings.Builder
	for _, q := range authorQIDs {
		authors.WriteString(" wd:" + q)
	}
	for _, q := range allowedAuthorFilmClasses() {
		allowed.WriteString(" wd:" + q)
	}
	return `SELECT ?author ?work
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?cls), "entity/"); separator="|") AS ?classes)
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?genre), "entity/"); separator="|") AS ?genres)
       (SAMPLE(?ru) AS ?ruLabel) (SAMPLE(?en) AS ?enLabel) (SAMPLE(?orig) AS ?origTitle)
       (MIN(YEAR(?date)) AS ?year)
       (GROUP_CONCAT(DISTINCT ?imdb; separator="|") AS ?imdbIds)
       (GROUP_CONCAT(DISTINCT ?kp; separator="|") AS ?kpIds)
       (GROUP_CONCAT(DISTINCT COALESCE(?dirRu, ?dirEn); separator="|") AS ?directors)
WHERE {
  VALUES ?author {` + authors.String() + ` }
  ?work wdt:P921 ?author .
  FILTER EXISTS { ?work wdt:P31 ?allowed . VALUES ?allowed {` + allowed.String() + ` } }
  ?work wdt:P31 ?cls .
  OPTIONAL { ?work wdt:P136 ?genre }
  OPTIONAL { ?work rdfs:label ?ru FILTER(LANG(?ru) = "ru") }
  OPTIONAL { ?work rdfs:label ?en FILTER(LANG(?en) = "en") }
  OPTIONAL { ?work wdt:P1476 ?orig }
  OPTIONAL { ?work wdt:P577 ?date }
  OPTIONAL { ?work wdt:P345 ?imdb }
  OPTIONAL { ?work wdt:P2603 ?kp }
  OPTIONAL { ?work wdt:P57 ?dir .
             OPTIONAL { ?dir rdfs:label ?dirRu FILTER(LANG(?dirRu) = "ru") }
             OPTIONAL { ?dir rdfs:label ?dirEn FILTER(LANG(?dirEn) = "en") } }
}
GROUP BY ?author ?work`
}

// FetchAuthorFilms returns, for 1..MaxAuthorFilmAuthors author QIDs, the films
// about them (P921 "main subject") that pass the class whitelist, keyed by
// author QID. Authors without films are absent from the map. A malformed QID
// or a wrong QID count returns ErrInvalidQuery without a call. One SPARQL call
// through the shared rate limiter.
func (c *WikidataClient) FetchAuthorFilms(ctx context.Context, authorQIDs []string) (map[string][]ExternalAuthorFilm, error) {
	if len(authorQIDs) == 0 || len(authorQIDs) > MaxAuthorFilmAuthors {
		return nil, fmt.Errorf("%w: 1..%d author QIDs required, got %d", ErrInvalidQuery, MaxAuthorFilmAuthors, len(authorQIDs))
	}
	for _, q := range authorQIDs {
		if !IsWikidataQID(q) {
			return nil, fmt.Errorf("%w: invalid author QID %q", ErrInvalidQuery, q)
		}
	}
	bindings, err := c.sparqlBindings(ctx, buildAuthorFilmsQuery(authorQIDs))
	if err != nil {
		return nil, err
	}
	return parseAuthorFilmBindings(bindings), nil
}

func parseAuthorFilmBindings(bindings []map[string]sparqlValue) map[string][]ExternalAuthorFilm {
	out := map[string][]ExternalAuthorFilm{}
	seen := map[string]struct{}{}
	for _, row := range bindings {
		authorQID := extractWikidataID(row["author"].Value)
		workQID := extractWikidataID(row["work"].Value)
		if !IsWikidataQID(authorQID) || !IsWikidataQID(workQID) {
			continue
		}
		key := authorQID + "|" + workQID
		if _, dup := seen[key]; dup {
			continue
		}
		kind, relation, ok := classifyAuthorFilm(strings.Split(row["classes"].Value, "|"), strings.Split(row["genres"].Value, "|"))
		if !ok {
			continue
		}
		title := firstNonEmpty(row["ruLabel"].Value, row["enLabel"].Value, row["origTitle"].Value)
		if title == "" {
			continue
		}
		seen[key] = struct{}{}
		w := ExternalAuthorFilm{
			ExternalScreenWork: ExternalScreenWork{
				QID:         workQID,
				Title:       title,
				Kind:        kind,
				Year:        parseScreenYear(row["year"].Value),
				Director:    joinDirectors(row["directors"].Value),
				IMDbID:      firstMatching(row["imdbIds"].Value, imdbIDPattern),
				KinopoiskID: firstMatching(row["kpIds"].Value, kinopoiskIDPattern),
			},
			Relation: relation,
		}
		if orig := strings.TrimSpace(row["origTitle"].Value); orig != "" && orig != title {
			w.OriginalTitle = orig
		}
		out[authorQID] = append(out[authorQID], w)
	}
	return out
}
