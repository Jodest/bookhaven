// Command olcoveraudit reports harvested books whose Open Library cover came
// from a loose title search and looks wrong (issue #687), and can clear the
// cover link and ISBN of books the operator lists. It has no HTTP endpoint on
// purpose. By default it only reports.
//
//	prod:   docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/olcoveraudit worker
//	apply:  ... /app/olcoveraudit -apply -ids=<uuid,uuid>
//	dev:    go run ./cmd/olcoveraudit
//
// See docs/DEPLOYMENT.md (Open Library cover audit).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bookhaven/backend/internal/config"
	adminrepository "github.com/bookhaven/backend/internal/modules/admin/repository"
	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/database"
)

func main() {
	apply := flag.Bool("apply", false, "clear the cover link and ISBN of the books listed in -ids")
	idsFlag := flag.String("ids", "", "comma-separated book IDs to clear (required with -apply)")
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
	if *apply && len(ids) == 0 {
		fmt.Println("usage: olcoveraudit [-apply -ids=<uuid,uuid>]  (-apply requires -ids)")
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

	// Open Library pacing is about one request per second.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	books := adminservice.NewService(adminrepository.NewGormRepository(db))
	report, err := harvestservice.RunCoverAudit(ctx, books, bookdata.NewOpenLibraryClient(),
		harvestservice.CoverAuditOptions{Apply: *apply, IDs: ids})
	if report != nil {
		report.Write(os.Stdout)
	}
	if err != nil {
		fmt.Printf("cover audit failed: %v\n", err)
		os.Exit(1)
	}
}
