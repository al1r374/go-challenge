package service_test

import (
	"context"
	"testing"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/service"
)

type memStore struct {
	adds   []string
	count  int64
	called string
}

func (m *memStore) Add(_ context.Context, userID, segment string) error {
	m.adds = append(m.adds, userID+":"+segment)
	m.count++
	return nil
}

func (m *memStore) Count(_ context.Context, segment string) (int64, error) {
	m.called = segment
	return m.count, nil
}

func TestServiceRoutesByMode(t *testing.T) {
	cfg, err := config.NewMemory(map[string]config.Mode{
		"premium_users": config.ModeExact,
		"sports":        config.ModeApproximate,
	})
	if err != nil {
		t.Fatal(err)
	}
	exact := &memStore{}
	approx := &memStore{}
	es := service.New(cfg, exact, approx)
	ctx := context.Background()

	if err := es.Add(ctx, "u1", "premium_users"); err != nil {
		t.Fatal(err)
	}
	if err := es.Add(ctx, "u2", "sports"); err != nil {
		t.Fatal(err)
	}
	if err := es.Add(ctx, "u3", "unknown_seg"); err != nil { // default approximate
		t.Fatal(err)
	}

	if len(exact.adds) != 1 || exact.adds[0] != "u1:premium_users" {
		t.Fatalf("exact adds=%v", exact.adds)
	}
	if len(approx.adds) != 2 {
		t.Fatalf("approx adds=%v", approx.adds)
	}

	exact.count = 42
	n, err := es.Count(ctx, "premium_users")
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 || exact.called != "premium_users" {
		t.Fatalf("exact count path failed: n=%d called=%s", n, exact.called)
	}
}
