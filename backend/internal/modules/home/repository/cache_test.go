package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisCache_RoundTripAndMiss(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	ctx := context.Background()

	if _, ok, err := c.Get(ctx, "home:v1:2026-10-04"); ok || err != nil {
		t.Fatalf("expected clean miss, got ok=%v err=%v", ok, err)
	}
	if err := c.Set(ctx, "home:v1:2026-10-04", []byte(`{"a":1}`), 10*time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := c.Get(ctx, "home:v1:2026-10-04")
	if err != nil || !ok || string(got) != `{"a":1}` {
		t.Fatalf("get: %q ok=%v err=%v", got, ok, err)
	}
	if ttl := mr.TTL("home:v1:2026-10-04"); ttl != 10*time.Minute {
		t.Fatalf("expected 10m TTL, got %v", ttl)
	}
}

func TestRedisCache_DownRedisReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	mr.Close()

	if _, ok, err := c.Get(context.Background(), "k"); ok || err == nil {
		t.Fatalf("expected error on down redis, got ok=%v err=%v", ok, err)
	}
	if err := c.Set(context.Background(), "k", []byte("v"), time.Minute); err == nil {
		t.Fatal("expected error on down redis")
	}
}
