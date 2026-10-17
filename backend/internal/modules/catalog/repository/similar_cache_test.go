package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestSimilarCache(t *testing.T) (*RedisSimilarCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisSimilarCache(client), mr
}

func TestRedisSimilarCache_RoundTrip(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	if err := c.Set(ctx, 0, bookID, ids, 6*time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !mr.Exists(similarCacheKey(0, bookID)) {
		t.Fatal("expected key similar:v1:<id> to exist")
	}

	got, ok, err := c.Get(ctx, 0, bookID)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if len(got) != len(ids) {
		t.Fatalf("expected %d ids, got %d", len(ids), len(got))
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Fatalf("position %d: expected %s, got %s", i, ids[i], got[i])
		}
	}
}

func TestRedisSimilarCache_EmptyListRoundTrip(t *testing.T) {
	c, _ := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()

	if err := c.Set(ctx, 0, bookID, []uuid.UUID{}, time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok, err := c.Get(ctx, 0, bookID)
	if err != nil || !ok || len(got) != 0 {
		t.Fatalf("expected cached empty list, got ids=%v ok=%v err=%v", got, ok, err)
	}
}

func TestRedisSimilarCache_MissingKey(t *testing.T) {
	c, _ := newTestSimilarCache(t)
	got, ok, err := c.Get(context.Background(), 0, uuid.New())
	if err != nil || ok || got != nil {
		t.Fatalf("expected miss, got ids=%v ok=%v err=%v", got, ok, err)
	}
}

func TestRedisSimilarCache_TTLSet(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	bookID := uuid.New()
	if err := c.Set(context.Background(), 0, bookID, []uuid.UUID{uuid.New()}, 6*time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if ttl := mr.TTL(similarCacheKey(0, bookID)); ttl != 6*time.Hour {
		t.Fatalf("expected TTL 6h, got %v", ttl)
	}
}

func TestRedisSimilarCache_CorruptValue(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	bookID := uuid.New()
	key := similarCacheKey(0, bookID)

	for _, raw := range []string{"not-json", `["not-a-uuid"]`} {
		if err := mr.Set(key, raw); err != nil {
			t.Fatalf("seed: %v", err)
		}
		_, ok, err := c.Get(context.Background(), 0, bookID)
		if err == nil || ok {
			t.Fatalf("value %q: expected error and ok=false, got ok=%v err=%v", raw, ok, err)
		}
	}
}

func TestRedisSimilarCache_ClosedServerErrorsQuickly(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	mr.Close()

	start := time.Now()
	_, ok, err := c.Get(context.Background(), 0, uuid.New())
	if err == nil || ok {
		t.Fatalf("expected error on closed server, got ok=%v err=%v", ok, err)
	}
	if err := c.Set(context.Background(), 0, uuid.New(), []uuid.UUID{uuid.New()}, time.Hour); err == nil {
		t.Fatal("expected Set error on closed server")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("expected fail-fast, took %v", elapsed)
	}
}

func TestRedisSimilarCache_InvalidateDropsOldEntries(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()
	ids := []uuid.UUID{uuid.New()}

	gen, err := c.Generation(ctx)
	if err != nil || gen != 0 {
		t.Fatalf("expected generation 0, got %d err=%v", gen, err)
	}
	if err := c.Set(ctx, gen, bookID, ids, time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Invalidate(ctx); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	gen, err = c.Generation(ctx)
	if err != nil || gen != 1 {
		t.Fatalf("expected generation 1, got %d err=%v", gen, err)
	}
	if _, ok, err := c.Get(ctx, gen, bookID); err != nil || ok {
		t.Fatalf("expected miss after invalidate, got ok=%v err=%v", ok, err)
	}
	if err := c.Set(ctx, gen, bookID, ids, time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !mr.Exists(similarCacheKey(1, bookID)) {
		t.Fatal("expected entry under generation 1")
	}
	if got, ok, err := c.Get(ctx, gen, bookID); err != nil || !ok || len(got) != 1 {
		t.Fatalf("expected hit in new generation, got ids=%v ok=%v err=%v", got, ok, err)
	}
}

// A list computed before an Invalidate is written under the generation read
// before the compute, so it is not visible afterwards.
func TestRedisSimilarCache_InvalidateBetweenGetAndSetHidesStaleWrite(t *testing.T) {
	c, _ := newTestSimilarCache(t)
	ctx := context.Background()
	bookID := uuid.New()

	gen, err := c.Generation(ctx)
	if err != nil {
		t.Fatalf("generation: %v", err)
	}
	if _, ok, _ := c.Get(ctx, gen, bookID); ok {
		t.Fatal("expected miss")
	}
	if err := c.Invalidate(ctx); err != nil { // admin write commits mid-compute
		t.Fatalf("invalidate: %v", err)
	}
	if err := c.Set(ctx, gen, bookID, []uuid.UUID{uuid.New()}, time.Hour); err != nil {
		t.Fatalf("set: %v", err)
	}
	newGen, _ := c.Generation(ctx)
	if _, ok, err := c.Get(ctx, newGen, bookID); err != nil || ok {
		t.Fatalf("stale write must not be visible, got ok=%v err=%v", ok, err)
	}
}

func TestRedisSimilarCache_GenerationClosedServerErrors(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	mr.Close()
	if _, err := c.Generation(context.Background()); err == nil {
		t.Fatal("expected error on closed server")
	}
}

func TestRedisSimilarCache_InvalidateClosedServerErrors(t *testing.T) {
	c, mr := newTestSimilarCache(t)
	mr.Close()
	if err := c.Invalidate(context.Background()); err == nil {
		t.Fatal("expected error on closed server")
	}
}
