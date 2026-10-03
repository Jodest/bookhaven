package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	// Validate arguments before touching the database.
	cmd, err := parseArgs(os.Args[1:], os.Getenv("APP_ENV"))
	if err != nil {
		fmt.Printf("error: %v\n\n%s\n", err, usage)
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Println("DATABASE_URL is required")
		os.Exit(1)
	}

	m, err := migrate.New(
		"file:///app/migrations",
		databaseURL,
	)
	if err != nil {
		fmt.Printf("failed to create migrate instance: %v\n", err)
		os.Exit(1)
	}

	switch cmd.action {
	case actionUp:
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("failed to run migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrations applied successfully")
	case actionDownSteps:
		if _, _, err := m.Version(); err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("no migrations applied; nothing to roll back")
				return
			}
			fmt.Printf("failed to get version: %v\n", err)
			os.Exit(1)
		}
		err := m.Steps(-cmd.steps)
		var short migrate.ErrShortLimit
		switch {
		case err == nil, errors.Is(err, migrate.ErrNoChange):
		case errors.As(err, &short):
			fmt.Printf("warning: requested %d step(s) but only %d could be rolled back\n", cmd.steps, uint(cmd.steps)-short.Short)
		default:
			fmt.Printf("failed to rollback migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("rolled back up to %d migration step(s)\n", cmd.steps)
		printVersion(m)
	case actionDownAll:
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("failed to rollback migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("all migrations rolled back")
		printVersion(m)
	case actionVersion:
		version, dirty, err := m.Version()
		if err != nil {
			fmt.Printf("failed to get version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("version: %d, dirty: %v\n", version, dirty)
	}
}

// printVersion reports the schema version after a rollback.
func printVersion(m *migrate.Migrate) {
	version, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		fmt.Println("current version: none (no migrations applied)")
	case err != nil:
		fmt.Printf("failed to get version: %v\n", err)
	default:
		fmt.Printf("current version: %d, dirty: %v\n", version, dirty)
	}
}

// Exact confirmation flag required for a full rollback (drops every table).
const confirmDropAllFlag = "--yes-drop-everything"

// Extra flag required for a full rollback when APP_ENV=production.
const forceFlag = "--force"

const usage = `Usage: migrate <command>

Commands:
  up                                   apply all pending migrations
  version                              print the current schema version
  down                                 roll back ONE migration step
  down N                               roll back N migration steps (N > 0)
  down-all --yes-drop-everything       roll back ALL migrations (drops the whole schema);
                                       refused when APP_ENV=production unless --force is also given`

type action int

const (
	actionUp action = iota + 1
	actionVersion
	actionDownSteps
	actionDownAll
)

type command struct {
	action action
	steps  int // number of steps to roll back, only for actionDownSteps
}

// parseArgs turns command-line arguments (without the program name) into a
// command. It is pure so it can be unit-tested without a database.
func parseArgs(args []string, appEnv string) (command, error) {
	if len(args) == 0 {
		return command{}, errors.New("no command given")
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "up", "version":
		if len(rest) > 0 {
			return command{}, fmt.Errorf("%q takes no arguments", cmd)
		}
		if cmd == "up" {
			return command{action: actionUp}, nil
		}
		return command{action: actionVersion}, nil

	case "down":
		switch len(rest) {
		case 0:
			return command{action: actionDownSteps, steps: 1}, nil
		case 1:
			n, err := strconv.Atoi(rest[0])
			if err != nil || n <= 0 {
				return command{}, fmt.Errorf("invalid step count %q: must be a positive integer", rest[0])
			}
			return command{action: actionDownSteps, steps: n}, nil
		default:
			return command{}, errors.New(`"down" takes at most one argument (step count)`)
		}

	case "down-all":
		confirmed, forced := false, false
		for _, a := range rest {
			switch a {
			case confirmDropAllFlag:
				confirmed = true
			case forceFlag:
				forced = true
			default:
				return command{}, fmt.Errorf("unknown argument %q for down-all", a)
			}
		}
		if !confirmed {
			return command{}, fmt.Errorf("down-all rolls back EVERY migration and drops the whole schema; "+
				"re-run with %s to confirm, or use \"down\" / \"down N\" to roll back step by step", confirmDropAllFlag)
		}
		if appEnv == "production" && !forced {
			return command{}, fmt.Errorf("refusing to run down-all with APP_ENV=production without %s", forceFlag)
		}
		return command{action: actionDownAll}, nil

	default:
		return command{}, fmt.Errorf("unknown command %q", cmd)
	}
}
