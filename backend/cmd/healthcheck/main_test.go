package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
)

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"db"}, {"api", "worker"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 1 {
			t.Errorf("run(%q) = %d, want 1", args, code)
		}
		if stderr.Len() == 0 {
			t.Errorf("run(%q) printed no usage", args)
		}
	}
}

func TestCheckAPI(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "ok", status: http.StatusOK},
		{name: "unavailable", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			err := checkAPI(context.Background(), srv.Client(), srv.URL+"/health")
			if (err != nil) != tt.wantErr {
				t.Errorf("checkAPI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckAPIUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/health"
	srv.Close()

	if err := checkAPI(context.Background(), http.DefaultClient, url); err == nil {
		t.Fatal("checkAPI() on a closed server returned nil")
	}
}

func TestCheckAPITimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := checkAPI(ctx, srv.Client(), srv.URL+"/health"); err == nil {
		t.Fatal("checkAPI() on a hung server returned nil")
	}
}

func TestCheckWorker(t *testing.T) {
	const host = "c0ffee123456"
	tests := []struct {
		name    string
		servers []*asynq.ServerInfo
		wantErr bool
	}{
		{name: "no servers", wantErr: true},
		{name: "only other hosts", servers: []*asynq.ServerInfo{{Host: "other", Status: "active"}}, wantErr: true},
		{name: "local active", servers: []*asynq.ServerInfo{{Host: "other", Status: "active"}, {Host: host, Status: "active"}}},
		{name: "local stopped only", servers: []*asynq.ServerInfo{{Host: host, Status: "stopped"}}, wantErr: true},
		{name: "one of several local active", servers: []*asynq.ServerInfo{{Host: host, Status: "stopped"}, {Host: host, Status: "active"}}},
		{name: "nil entry ignored", servers: []*asynq.ServerInfo{nil, {Host: host, Status: "active"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkWorker(tt.servers, host)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkWorker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	if err := checkWorker([]*asynq.ServerInfo{{Host: "", Status: "active"}}, ""); err == nil {
		t.Error("checkWorker() with an empty hostname returned nil")
	}
}

// TestCheckWorkerRedis runs a real asynq server against miniredis: the probe
// must see its heartbeat while it runs and fail once it has shut down.
func TestCheckWorkerRedis(t *testing.T) {
	mr := miniredis.RunT(t)

	if err := checkWorkerRedis(mr.Addr()); err == nil {
		t.Fatal("checkWorkerRedis() with no server returned nil")
	}

	srv := asynq.NewServer(asynq.RedisClientOpt{Addr: mr.Addr()}, asynq.Config{
		Concurrency: 1,
		LogLevel:    asynq.FatalLevel,
	})
	if err := srv.Start(asynq.NewServeMux()); err != nil {
		t.Fatalf("start asynq server: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		err := checkWorkerRedis(mr.Addr())
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			srv.Shutdown()
			t.Fatalf("checkWorkerRedis() never saw the running server: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	srv.Shutdown()
	if err := checkWorkerRedis(mr.Addr()); err == nil {
		t.Error("checkWorkerRedis() after shutdown returned nil")
	}
}

func TestCheckWorkerRedisUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()

	if err := checkWorkerRedis(addr); err == nil {
		t.Fatal("checkWorkerRedis() with Redis down returned nil")
	}
}

func TestRunAPIUsesAppPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("listener address: %v", err)
	}
	t.Setenv("APP_PORT", port)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"api"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(api) = %d, stderr %q", code, stderr.String())
	}
	if stdout.String() != "ok\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "ok\n")
	}
}
