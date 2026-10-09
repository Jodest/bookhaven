// Command harvest is a thin, enqueue-only CLI for the catalog harvest job:
// it reads a SPARQL query (from -query, -query-file, or the curated
// author-list mode below), builds an asynq.Client from REDIS_URL, enqueues
// one harvest:wikidata_query task, and prints the resulting task ID and
// queue name. It performs no DB or bookdata-client calls itself - the
// actual harvest runs in cmd/worker.
//
// Manual smoke-test recipe (see docs/features/async-harvest-queue.md for
// the full procedure): start `docker compose up postgres redis backend
// worker`, then from another shell run
//
//	go run ./cmd/harvest -query-file ./testdata/example.rq
//
// note the printed task ID, watch `docker compose logs -f worker` for
// processing, then inspect the result with asynq.Inspector (or the
// `asynqmon` CLI/web UI pointed at the same Redis) using the printed queue
// name + task ID. A completed job should have created at least one book
// with status = pending in the catalog.
//
// Curated-author-list mode ("Approach A", see
// docs/features/harvest-review-queue.md): instead of hand-writing SPARQL,
// pass one or more Wikidata author QIDs via -authors or -authors-file and a
// bounded query is built for you, e.g.:
//
//	go run ./cmd/harvest -authors Q7200,Q1183296 -limit 500
//
// -authors/-authors-file are mutually exclusive with each other and with
// -query/-query-file; exactly one input mode is required.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bookhaven/backend/internal/config"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/hibiken/asynq"
)

func main() {
	queryFlag := flag.String("query", "", "SPARQL query to run against Wikidata")
	queryFileFlag := flag.String("query-file", "", "path to a .rq/.txt file containing the SPARQL query")
	authorsFlag := flag.String("authors", "", "comma-separated Wikidata author QIDs, e.g. Q7200,Q1183296 (mutually exclusive with -query/-query-file/-authors-file)")
	authorsFileFlag := flag.String("authors-file", "", "path to a .txt file with one Wikidata author QID per line (blank lines and #-prefixed comments ignored; mutually exclusive with -query/-query-file/-authors)")
	limitFlag := flag.Int("limit", 0, "SPARQL LIMIT for -authors/-authors-file mode only (default 500, clamped to 5000); usage error if combined with -query/-query-file")
	flag.Parse()

	limitSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "limit" {
			limitSet = true
		}
	})

	query, err := buildQuery(*queryFlag, *queryFileFlag, *authorsFlag, *authorsFileFlag, *limitFlag, limitSet, os.ReadFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "harvest:", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "harvest: load config:", err)
		os.Exit(1)
	}

	task, err := harvestservice.NewHarvestTask(query)
	if err != nil {
		fmt.Fprintln(os.Stderr, "harvest: build task:", err)
		os.Exit(1)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})
	defer client.Close()

	info, err := client.Enqueue(task)
	if err != nil {
		fmt.Fprintln(os.Stderr, "harvest: enqueue:", err)
		os.Exit(1)
	}

	fmt.Printf("enqueued task id=%s queue=%s\n", info.ID, info.Queue)
}

// buildQuery resolves the SPARQL query text from exactly one of the
// -query, -query-file, -authors, or -authors-file flags. Factored out of
// main() so argument-handling logic is unit-testable without a real Redis
// connection. -authors/-authors-file build the query via
// bookdata.BuildAuthorListQuery instead of taking raw SPARQL; limit only
// applies to those two modes (limitSet distinguishes an explicit -limit 0
// from the flag simply not having been passed, so -limit combined with
// -query/-query-file is reliably flagged as a usage error).
func buildQuery(queryFlag, queryFileFlag, authorsFlag, authorsFileFlag string, limit int, limitSet bool, readFile func(string) ([]byte, error)) (string, error) {
	queryFlag = strings.TrimSpace(queryFlag)
	queryFileFlag = strings.TrimSpace(queryFileFlag)
	authorsFlag = strings.TrimSpace(authorsFlag)
	authorsFileFlag = strings.TrimSpace(authorsFileFlag)

	modeCount := 0
	for _, v := range []string{queryFlag, queryFileFlag, authorsFlag, authorsFileFlag} {
		if v != "" {
			modeCount++
		}
	}
	if modeCount > 1 {
		return "", errors.New("specify only one of -query, -query-file, -authors, or -authors-file")
	}

	if limitSet && (queryFlag != "" || queryFileFlag != "") {
		return "", errors.New("-limit is only meaningful with -authors or -authors-file")
	}

	switch {
	case queryFlag != "":
		return queryFlag, nil
	case queryFileFlag != "":
		data, err := readFile(queryFileFlag)
		if err != nil {
			return "", fmt.Errorf("read query file: %w", err)
		}
		query := strings.TrimSpace(string(data))
		if query == "" {
			return "", fmt.Errorf("query file %q is empty", queryFileFlag)
		}
		return query, nil
	case authorsFlag != "":
		return bookdata.BuildAuthorListQuery(splitAuthorQIDs(authorsFlag), limit)
	case authorsFileFlag != "":
		data, err := readFile(authorsFileFlag)
		if err != nil {
			return "", fmt.Errorf("read authors file: %w", err)
		}
		qids := parseAuthorQIDsFile(string(data))
		if len(qids) == 0 {
			return "", fmt.Errorf("authors file %q contains no QIDs", authorsFileFlag)
		}
		return bookdata.BuildAuthorListQuery(qids, limit)
	}
	return "", errors.New("one of -query, -query-file, -authors, or -authors-file is required")
}

// splitAuthorQIDs splits a comma-separated -authors flag value into
// trimmed, non-empty QID strings. Format/dedup validation happens in
// bookdata.BuildAuthorListQuery.
func splitAuthorQIDs(value string) []string {
	parts := strings.Split(value, ",")
	qids := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			qids = append(qids, p)
		}
	}
	return qids
}

// parseAuthorQIDsFile parses a -authors-file value: one QID per line, blank
// lines and #-prefixed comment lines ignored, mirroring the plain-text
// convention already used for -query-file.
func parseAuthorQIDsFile(contents string) []string {
	lines := strings.Split(contents, "\n")
	qids := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		qids = append(qids, line)
	}
	return qids
}
