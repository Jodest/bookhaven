// Command harvest is a thin, enqueue-only CLI for the catalog harvest job:
// it reads a SPARQL query (from -query or -query-file), builds an
// asynq.Client from REDIS_URL, enqueues one harvest:wikidata_query task, and
// prints the resulting task ID and queue name. It performs no DB or
// bookdata-client calls itself - the actual harvest runs in cmd/worker.
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
// with is_hidden = true in the catalog.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bookhaven/backend/internal/config"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/hibiken/asynq"
)

func main() {
	queryFlag := flag.String("query", "", "SPARQL query to run against Wikidata")
	queryFileFlag := flag.String("query-file", "", "path to a .rq/.txt file containing the SPARQL query")
	flag.Parse()

	query, err := buildQuery(*queryFlag, *queryFileFlag, os.ReadFile)
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

// buildQuery resolves the SPARQL query text from exactly one of the -query
// or -query-file flags. Factored out of main() so argument-handling logic
// is unit-testable without a real Redis connection.
func buildQuery(queryFlag, queryFileFlag string, readFile func(string) ([]byte, error)) (string, error) {
	queryFlag = strings.TrimSpace(queryFlag)
	queryFileFlag = strings.TrimSpace(queryFileFlag)

	if queryFlag != "" && queryFileFlag != "" {
		return "", errors.New("specify only one of -query or -query-file")
	}
	if queryFlag != "" {
		return queryFlag, nil
	}
	if queryFileFlag != "" {
		data, err := readFile(queryFileFlag)
		if err != nil {
			return "", fmt.Errorf("read query file: %w", err)
		}
		query := strings.TrimSpace(string(data))
		if query == "" {
			return "", fmt.Errorf("query file %q is empty", queryFileFlag)
		}
		return query, nil
	}
	return "", errors.New("one of -query or -query-file is required")
}
