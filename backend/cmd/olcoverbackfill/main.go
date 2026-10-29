// Command olcoverbackfill finds Wikidata books without a cover, reads their
// Open Library IDs (P648) from Wikidata and sets the cover from the Open
// Library work or edition record (issue #688). Books still without a cover are
// then tried on FantLab: the Wikidata FantLab work ID (P7439) leads to a
// Russian paper edition named exactly like the book, and its cover is linked,
// not copied (issue #686). It has no HTTP endpoint on
// purpose. By default it only reports; -apply sets the cover links through the
// normal book save, which queues a cover copy.
//
//	prod:   docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/olcoverbackfill worker
//	apply:  ... /app/olcoverbackfill -apply [-ids=<uuid,uuid>]
//	dev:    go run ./cmd/olcoverbackfill
//
// Do not run it while a harvest job is active: both pace FantLab requests on
// their own. See docs/DEPLOYMENT.md (Open Library cover backfill).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/bookhaven/backend/internal/config"
	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	coversservice "github.com/bookhaven/backend/internal/modules/covers/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/database"
)

func main() {
	apply := flag.Bool("apply", false, "set the cover link of the found books")
	idsFlag := flag.String("ids", "", "comma-separated book IDs to update (only with -apply)")
	flag.Parse()

	var ids []uuid.UUID
	for _, raw := range strings.Split(*idsFlag, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			fmt.Printf("invalid id %q: %v\n", raw, err)
			os.Exit(2)
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 && !*apply {
		fmt.Println("usage: olcoverbackfill [-apply [-ids=<uuid,uuid>]]  (-ids requires -apply)")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}
	db, err := database.Open(cfg.DatabaseURL, database.PoolConfigFrom(cfg))
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Wikidata and Open Library pace about one request per second, FantLab one
	// per 1.2 s with up to 4 requests per book (about 365 works).
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()

	var adminOpts []adminservice.Option
	if *apply && cfg.CoversMirrorEnabled {
		client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})
		defer client.Close()
		adminOpts = append(adminOpts, adminservice.WithCoverEnqueuer(coversservice.NewEnqueuer(client)))
	}
	books := adminservice.NewService(adminrepository.NewGormRepository(db), adminOpts...)

	wikidata := bookdata.NewWikidataClient()
	report, err := harvestservice.RunCoverBackfill(ctx, books, wikidata, bookdata.NewOpenLibraryClient(),
		harvestservice.CoverBackfillOptions{Apply: *apply, IDs: ids, FantLab: bookdata.NewFantLabClient(), FantLabIDs: wikidata})
	if report != nil {
		report.Write(os.Stdout)
	}
	if err != nil {
		fmt.Printf("cover backfill failed: %v\n", err)
		os.Exit(1)
	}
}
