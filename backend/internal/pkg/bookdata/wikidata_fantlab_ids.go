package bookdata

import (
	"context"
	"fmt"
	"strings"
)

// MaxFantLabIDBooks is the number of book QIDs one FetchFantLabWorkIDs
// call accepts (one SPARQL query).
const MaxFantLabIDBooks = MaxEntitiesPerRequest

func buildFantLabWorkIDsQuery(qids []string) string {
	var b strings.Builder
	for _, q := range qids {
		b.WriteString(" wd:" + q)
	}
	return `SELECT ?item (GROUP_CONCAT(DISTINCT ?fantlab; separator="|") AS ?fantlabIds)
WHERE {
  VALUES ?item {` + b.String() + ` }
  ?item wdt:P7439 ?fantlab .
}
GROUP BY ?item`
}

// FetchFantLabWorkIDs returns, for 1..MaxFantLabIDBooks book QIDs, the
// FantLab IDs (P7439) of each, keyed by QID. Books without P7439 are
// absent. A malformed QID or a wrong QID count returns ErrInvalidQuery without
// a call. One SPARQL call through the shared rate limiter.
func (c *WikidataClient) FetchFantLabWorkIDs(ctx context.Context, qids []string) (map[string][]string, error) {
	if len(qids) == 0 || len(qids) > MaxFantLabIDBooks {
		return nil, fmt.Errorf("%w: 1..%d book QIDs required, got %d", ErrInvalidQuery, MaxFantLabIDBooks, len(qids))
	}
	for _, q := range qids {
		if !IsWikidataQID(q) {
			return nil, fmt.Errorf("%w: invalid book QID %q", ErrInvalidQuery, q)
		}
	}
	bindings, err := c.sparqlBindings(ctx, buildFantLabWorkIDsQuery(qids))
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, row := range bindings {
		qid := extractWikidataID(row["item"].Value)
		if !IsWikidataQID(qid) {
			continue
		}
		for _, id := range strings.Split(row["fantlabIds"].Value, "|") {
			if id = strings.TrimSpace(id); id != "" {
				out[qid] = append(out[qid], id)
			}
		}
	}
	return out, nil
}
