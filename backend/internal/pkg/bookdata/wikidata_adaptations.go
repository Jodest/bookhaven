package bookdata

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Screen adaptations of books from Wikidata (openspec change
// add-book-screen-adaptations, catalog-import/wikidata-adaptation-sync).

// Screen work kinds returned in ExternalScreenWork.Kind.
const (
	ScreenKindFilm           = "film"
	ScreenKindTVFilm         = "tv_film"
	ScreenKindSeries         = "series"
	ScreenKindAnimatedFilm   = "animated_film"
	ScreenKindAnimatedSeries = "animated_series"
	ScreenKindTeleplay       = "teleplay"
)

// MaxAdaptationBooks is the number of book QIDs one FetchAdaptations call
// accepts (one SPARQL query).
const MaxAdaptationBooks = MaxEntitiesPerRequest

const (
	screenYearMin = 1880
	screenYearMax = 2100
	// screenMaxDirectors caps the directors joined into one string.
	screenMaxDirectors = 3
	// screenIDMaxLen matches the VARCHAR(20) columns of screen_works.
	screenIDMaxLen = 20
)

// Class whitelist, verified live against Wikidata (query.wikidata.org,
// 2026-10-15; label - description):
//
//	Q356055    teleplay (телеспектакль) - screenplay or script used in the
//	           production of a scripted television program (the 1985 Soviet
//	           "Хоббит", Q1187415, is an instance of it and of Q506240)
//	Q581714    animated series - episodic form of series of animated works
//	Q117467246 animated television series
//	Q5398426   television series
//	Q1259759   miniseries
//	Q202866    animated film
//	Q29168811  animated feature film
//	Q506240    television film
//	Q11424     film
//	Q24869     feature film
//	Q24862     short film
//
// Exclusions (any match drops the item even with an allowed class):
//
//	Q24856     film series
//	Q7889      video game
//	Q131436    board game
//	Q13406463  Wikimedia list article
//	Q21191270  television series episode (NOT a teleplay)
//	Q7725310   series of creative works
var (
	// screenKindPriority lists kinds in the order the first match wins.
	screenKindPriority = []struct {
		kind    string
		classes []string
	}{
		{ScreenKindTeleplay, []string{"Q356055"}},
		{ScreenKindAnimatedSeries, []string{"Q581714", "Q117467246"}},
		{ScreenKindSeries, []string{"Q5398426", "Q1259759"}},
		{ScreenKindAnimatedFilm, []string{"Q202866", "Q29168811"}},
		{ScreenKindTVFilm, []string{"Q506240"}},
		{ScreenKindFilm, []string{"Q11424", "Q24869", "Q24862"}},
	}
	screenExcludedClasses = map[string]struct{}{
		"Q24856": {}, "Q7889": {}, "Q131436": {}, "Q13406463": {}, "Q21191270": {}, "Q7725310": {},
	}
)

var (
	imdbIDPattern      = regexp.MustCompile(`^tt[0-9]+$`)
	kinopoiskIDPattern = regexp.MustCompile(`^[0-9]+$`)
	tmdbIDPattern      = regexp.MustCompile(`^[0-9]{1,10}$`)
)

const commonsFilePathMarker = "Special:FilePath/"

// commonsFileName extracts the file name from a Wikidata commonsMedia value
// (http://commons.wikimedia.org/wiki/Special:FilePath/<url-encoded name>).
// It returns "" for an empty name or one containing "|".
func commonsFileName(raw string) string {
	raw = strings.TrimSpace(raw)
	i := strings.Index(raw, commonsFilePathMarker)
	if i < 0 {
		return ""
	}
	name, err := url.PathUnescape(raw[i+len(commonsFilePathMarker):])
	if err != nil {
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "|") || strings.ContainsAny(name, "\r\n") {
		return ""
	}
	return name
}

// ExternalScreenWork is one adaptation of a book found in Wikidata.
type ExternalScreenWork struct {
	QID           string
	Title         string
	OriginalTitle string // empty when absent or equal to Title
	Kind          string // one of the ScreenKind* constants
	Year          *int
	Director      string // up to three names joined with ", "
	IMDbID        string // empty when absent or malformed
	KinopoiskID   string // empty when absent or malformed
	// PosterFile and ImageFile are Commons file names of P3383 (film poster)
	// and P18 (image); empty when absent or unusable.
	PosterFile string
	ImageFile  string
	// TMDBMovieID (P4947) and TMDBTVID (P4983) are numeric TMDB ids; empty
	// when absent or malformed.
	TMDBMovieID string
	TMDBTVID    string
}

// classifyAdaptation maps the P31 classes of an item to a screen kind. ok is
// false when the item has an excluded class or no allowed class.
func classifyAdaptation(classes []string) (kind string, ok bool) {
	set := make(map[string]struct{}, len(classes))
	for _, c := range classes {
		c = strings.TrimSpace(c)
		if _, excluded := screenExcludedClasses[c]; excluded {
			return "", false
		}
		set[c] = struct{}{}
	}
	for _, k := range screenKindPriority {
		for _, c := range k.classes {
			if _, has := set[c]; has {
				return k.kind, true
			}
		}
	}
	return "", false
}

func buildAdaptationsQuery(bookQIDs []string) string {
	var b strings.Builder
	for _, q := range bookQIDs {
		b.WriteString(" wd:" + q)
	}
	return `SELECT ?book ?work
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?cls), "entity/"); separator="|") AS ?classes)
       (SAMPLE(?ru) AS ?ruLabel) (SAMPLE(?en) AS ?enLabel) (SAMPLE(?orig) AS ?origTitle)
       (MIN(YEAR(?date)) AS ?year)
       (GROUP_CONCAT(DISTINCT ?imdb; separator="|") AS ?imdbIds)
       (GROUP_CONCAT(DISTINCT ?kp; separator="|") AS ?kpIds)
       (GROUP_CONCAT(DISTINCT COALESCE(?dirRu, ?dirEn); separator="|") AS ?directors)
       (SAMPLE(?posterF) AS ?posterFile) (SAMPLE(?imageF) AS ?imageFile)
       (SAMPLE(?tmdbM) AS ?tmdbMovie) (SAMPLE(?tmdbT) AS ?tmdbTv)
WHERE {
  VALUES ?book {` + b.String() + ` }
  ?work wdt:P144 ?book ; wdt:P31 ?cls .
  OPTIONAL { ?work rdfs:label ?ru FILTER(LANG(?ru) = "ru") }
  OPTIONAL { ?work rdfs:label ?en FILTER(LANG(?en) = "en") }
  OPTIONAL { ?work wdt:P1476 ?orig }
  OPTIONAL { ?work wdt:P577 ?date }
  OPTIONAL { ?work wdt:P345 ?imdb }
  OPTIONAL { ?work wdt:P2603 ?kp }
  OPTIONAL { ?work wdt:P3383 ?posterF }
  OPTIONAL { ?work wdt:P18 ?imageF }
  OPTIONAL { ?work wdt:P4947 ?tmdbM }
  OPTIONAL { ?work wdt:P4983 ?tmdbT }
  OPTIONAL { ?work wdt:P57 ?dir .
             OPTIONAL { ?dir rdfs:label ?dirRu FILTER(LANG(?dirRu) = "ru") }
             OPTIONAL { ?dir rdfs:label ?dirEn FILTER(LANG(?dirEn) = "en") } }
}
GROUP BY ?book ?work`
}

// FetchAdaptations returns, for 1..MaxAdaptationBooks book QIDs, the screen
// adaptations (P144 "based on") that pass the class whitelist, keyed by book
// QID. Books without adaptations are absent from the map. A malformed QID or
// a wrong QID count returns ErrInvalidQuery without a call. One SPARQL call
// through the shared rate limiter.
func (c *WikidataClient) FetchAdaptations(ctx context.Context, bookQIDs []string) (map[string][]ExternalScreenWork, error) {
	if len(bookQIDs) == 0 || len(bookQIDs) > MaxAdaptationBooks {
		return nil, fmt.Errorf("%w: 1..%d book QIDs required, got %d", ErrInvalidQuery, MaxAdaptationBooks, len(bookQIDs))
	}
	for _, q := range bookQIDs {
		if !IsWikidataQID(q) {
			return nil, fmt.Errorf("%w: invalid book QID %q", ErrInvalidQuery, q)
		}
	}
	bindings, err := c.sparqlBindings(ctx, buildAdaptationsQuery(bookQIDs))
	if err != nil {
		return nil, err
	}
	return parseAdaptationBindings(bindings), nil
}

func parseAdaptationBindings(bindings []map[string]sparqlValue) map[string][]ExternalScreenWork {
	out := map[string][]ExternalScreenWork{}
	seen := map[string]struct{}{}
	for _, row := range bindings {
		bookQID := extractWikidataID(row["book"].Value)
		workQID := extractWikidataID(row["work"].Value)
		if !IsWikidataQID(bookQID) || !IsWikidataQID(workQID) {
			continue
		}
		key := bookQID + "|" + workQID
		if _, dup := seen[key]; dup {
			continue
		}
		kind, ok := classifyAdaptation(strings.Split(row["classes"].Value, "|"))
		if !ok {
			continue
		}
		title := firstNonEmpty(row["ruLabel"].Value, row["enLabel"].Value, row["origTitle"].Value)
		if title == "" {
			continue
		}
		seen[key] = struct{}{}
		w := ExternalScreenWork{
			QID:         workQID,
			Title:       title,
			Kind:        kind,
			Year:        parseScreenYear(row["year"].Value),
			Director:    joinDirectors(row["directors"].Value),
			IMDbID:      firstMatching(row["imdbIds"].Value, imdbIDPattern),
			KinopoiskID: firstMatching(row["kpIds"].Value, kinopoiskIDPattern),

			PosterFile:  commonsFileName(row["posterFile"].Value),
			ImageFile:   commonsFileName(row["imageFile"].Value),
			TMDBMovieID: firstMatching(row["tmdbMovie"].Value, tmdbIDPattern),
			TMDBTVID:    firstMatching(row["tmdbTv"].Value, tmdbIDPattern),
		}
		if orig := strings.TrimSpace(row["origTitle"].Value); orig != "" && orig != title {
			w.OriginalTitle = orig
		}
		out[bookQID] = append(out[bookQID], w)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func parseScreenYear(v string) *int {
	y, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || y < screenYearMin || y > screenYearMax {
		return nil
	}
	return &y
}

// firstMatching returns the first pipe-separated value matching re (and fitting
// the id columns), or "".
func firstMatching(v string, re *regexp.Regexp) string {
	for _, part := range strings.Split(v, "|") {
		part = strings.TrimSpace(part)
		if len(part) <= screenIDMaxLen && re.MatchString(part) {
			return part
		}
	}
	return ""
}

// joinDirectors sorts the distinct names and joins at most screenMaxDirectors.
func joinDirectors(v string) string {
	seen := map[string]struct{}{}
	var names []string
	for _, n := range strings.Split(v, "|") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > screenMaxDirectors {
		names = names[:screenMaxDirectors]
	}
	return strings.Join(names, ", ")
}
