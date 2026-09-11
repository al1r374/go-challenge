package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/store"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestExactAddAndCountDedupesWithinAndAcrossDays(t *testing.T) {
	_, rdb := setupRedis(t)
	day1 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	s1 := store.NewRedisExact(rdb, timebucket.FixedClock{T: day1}, 14)
	ctx := context.Background()
	if err := s1.Add(ctx, "u1", "sports"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Add(ctx, "u1", "sports"); err != nil { // same day duplicate
		t.Fatal(err)
	}
	if err := s1.Add(ctx, "u2", "sports"); err != nil {
		t.Fatal(err)
	}

	s2 := store.NewRedisExact(rdb, timebucket.FixedClock{T: day2}, 14)
	if err := s2.Add(ctx, "u1", "sports"); err != nil { // same user, next day
		t.Fatal(err)
	}
	if err := s2.Add(ctx, "u3", "sports"); err != nil {
		t.Fatal(err)
	}

	n, err := s2.Count(ctx, "sports")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("count=%d want 3 (u1,u2,u3 unique)", n)
	}
}

func TestExactIgnoresUsersOlderThanRetention(t *testing.T) {
	_, rdb := setupRedis(t)
	old := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	ctx := context.Background()
	oldStore := store.NewRedisExact(rdb, timebucket.FixedClock{T: old}, 14)
	if err := oldStore.Add(ctx, "ancient", "sports"); err != nil {
		t.Fatal(err)
	}

	cur := store.NewRedisExact(rdb, timebucket.FixedClock{T: now}, 14)
	if err := cur.Add(ctx, "fresh", "sports"); err != nil {
		t.Fatal(err)
	}
	n, err := cur.Count(ctx, "sports")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1 (ancient outside window)", n)
	}
}

func TestApproximateAddAndCount(t *testing.T) {
	_, rdb := setupRedis(t)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	s := store.NewRedisApproximate(rdb, timebucket.FixedClock{T: now}, 14)
	ctx := context.Background()

	for _, u := range []string{"a", "b", "c", "a", "b"} {
		if err := s.Add(ctx, u, "casual_readers"); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.Count(ctx, "casual_readers")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("count=%d want 3", n)
	}
}

func TestApproximateMergeAcrossDays(t *testing.T) {
	_, rdb := setupRedis(t)
	day1 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	s1 := store.NewRedisApproximate(rdb, timebucket.FixedClock{T: day1}, 14)
	_ = s1.Add(ctx, "u1", "sports")
	_ = s1.Add(ctx, "u2", "sports")

	s2 := store.NewRedisApproximate(rdb, timebucket.FixedClock{T: day2}, 14)
	_ = s2.Add(ctx, "u2", "sports")
	_ = s2.Add(ctx, "u3", "sports")

	n, err := s2.Count(ctx, "sports")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("count=%d want 3", n)
	}
}
