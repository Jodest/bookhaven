// Command healthcheck is the container HEALTHCHECK probe of the production
// backend image (docker-compose.prod.yml). The image is distroless: it has
// no shell, curl or wget, so the probe is a static binary next to api and
// worker, like migrate.
//
//	healthcheck api     GET http://127.0.0.1:$APP_PORT/health must answer 200.
//	                    Liveness only: PostgreSQL and Redis are not checked
//	                    (that is /health/ready), so a dependency blip does not
//	                    turn the api container unhealthy.
//	healthcheck worker  At least one asynq server of this container (same
//	                    hostname) must be heartbeating to Redis in the active
//	                    state. asynq writes the heartbeat every 5 s with a
//	                    10 s TTL, so a dead or shut-down worker drops out
//	                    within seconds (a hung task handler does not stop
//	                    the heartbeat and is not detected).
//	                    The worker has no HTTP port and cannot work without
//	                    Redis, so this check does depend on Redis.
//
// It reads only APP_PORT and REDIS_URL (config.LoadProbe), never the full
// configuration, prints one line and exits 0 when healthy and 1 otherwise
// (including bad usage: Docker reserves exit code 2).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bookhaven/backend/internal/config"
	"github.com/hibiken/asynq"
)

// probeTimeout bounds one probe; docker-compose.prod.yml gives the
// healthcheck a longer timeout, so the probe reports its own error first.
const probeTimeout = 3 * time.Second

const usage = `Usage: healthcheck <api|worker>`

// serverStatusActive is asynq's ServerInfo.Status of a running server.
const serverStatusActive = "active"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "api" && args[0] != "worker") {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	cfg := config.LoadProbe()
	var err error
	switch args[0] {
	case "api":
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		err = checkAPI(ctx, http.DefaultClient, "http://127.0.0.1:"+cfg.Port+"/health")
	case "worker":
		err = checkWorkerRedis(cfg.RedisURL)
	}
	if err != nil {
		fmt.Fprintf(stderr, "unhealthy: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}

// checkAPI requires a 200 from the liveness endpoint.
func checkAPI(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return nil
}

func checkWorkerRedis(redisURL string) error {
	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("hostname: %w", err)
	}
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:         redisURL,
		DialTimeout:  probeTimeout,
		ReadTimeout:  probeTimeout,
		WriteTimeout: probeTimeout,
	})
	defer inspector.Close()

	servers, err := inspector.Servers()
	if err != nil {
		return fmt.Errorf("list asynq servers: %w", err)
	}
	return checkWorker(servers, host)
}

// checkWorker passes when at least one active asynq server runs on host.
// Servers of other hosts (another container, a local dev worker on the same
// Redis) never count.
func checkWorker(servers []*asynq.ServerInfo, host string) error {
	if host == "" {
		return errors.New("empty hostname")
	}
	var local int
	for _, s := range servers {
		if s == nil || s.Host != host {
			continue
		}
		local++
		if s.Status == serverStatusActive {
			return nil
		}
	}
	if local == 0 {
		return fmt.Errorf("no asynq server heartbeat from host %q", host)
	}
	return fmt.Errorf("%d asynq server(s) on host %q, none active", local, host)
}
