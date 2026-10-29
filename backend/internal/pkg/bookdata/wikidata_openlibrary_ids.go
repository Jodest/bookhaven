package bookdata

import (
	"context"
	"fmt"
	"strings"
)

// MaxOpenLibraryIDBooks is the number of book QIDs one FetchOpenLibraryIDs
// call accepts (one SPARQL query).
const MaxOpenLibraryIDBooks = MaxEntitiesPerRequest

func buildOpenLibraryIDsQuery(qids []string) string {
	var b strings.Builder
	for _, q := range qids {
		b.WriteString(" wd:" + q)
	}
	return `SELECT ?item (GROUP_CONCAT(DISTINCT ?olid; separator="|") AS ?olids)
WHERE {
  VALUES ?item {` + b.String() + ` }
  ?item wdt:P648 ?olid .
}
GROUP BY ?item`
}

// FetchOpenLibraryIDs returns, for 1..MaxOpenLibraryIDBooks book QIDs, the
// Open Library IDs (P648) of each, keyed by QID. Books without P648 are
// absent. A malformed QID or a wrong QID count returns ErrInvalidQuery without
// a call. One SPARQL call through the shared rate limiter.
func (c *WikidataClient) FetchOpenLibraryIDs(ctx context.Context, qids []string) (map[string][]string, error) {
	if len(qids) == 0 || len(qids) > MaxOpenLibraryIDBooks {
		return nil, fmt.Errorf("%w: 1..%d book QIDs required, got %d", ErrInvalidQuery, MaxOpenLibraryIDBooks, len(qids))
	}
	for _, q := range qids {
		if !IsWikidataQID(q) {
			return nil, fmt.Errorf("%w: invalid book QID %q", ErrInvalidQuery, q)
		}
	}
	bindings, err := c.sparqlBindings(ctx, buildOpenLibraryIDsQuery(qids))
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, row := range bindings {
		qid := extractWikidataID(row["item"].Value)
		if !IsWikidataQID(qid) {
			continue
		}
		for _, id := range strings.Split(row["olids"].Value, "|") {
			if id = strings.TrimSpace(id); id != "" {
				out[qid] = append(out[qid], id)
			}
		}
	}
	return out, nil
}
