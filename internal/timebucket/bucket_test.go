package timebucket_test

import (
	"testing"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
)

func TestActiveWindow(t *testing.T) {
	now := time.Date(2026, 9, 11, 15, 30, 0, 0, time.UTC)
	clock := timebucket.FixedClock{T: now}
	const days = 14
	buckets := timebucket.Active(clock, days)

	if len(buckets) != days {
		t.Fatalf("len=%d want %d", len(buckets), days)
	}
	if buckets[0].Format() != "2026-09-11" {
		t.Fatalf("newest=%s", buckets[0].Format())
	}
	oldest := buckets[len(buckets)-1].Format()
	if oldest != "2026-08-29" {
		t.Fatalf("oldest=%s want 2026-08-29", oldest)
	}
}

func TestActiveCustomRetention(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	buckets := timebucket.Active(timebucket.FixedClock{T: now}, 3)
	if len(buckets) != 3 {
		t.Fatalf("len=%d", len(buckets))
	}
	if buckets[2].Format() != "2026-09-09" {
		t.Fatalf("oldest=%s", buckets[2].Format())
	}
}

func TestNormalizeRetention(t *testing.T) {
	if got := timebucket.NormalizeRetention(0); got != timebucket.DefaultRetentionDays {
		t.Fatalf("got %d", got)
	}
	if got := timebucket.NormalizeRetention(7); got != 7 {
		t.Fatalf("got %d", got)
	}
}

func TestKeyFormat(t *testing.T) {
	id := timebucket.ID{Day: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
	got := id.Key("es:exact", "sports")
	want := "es:exact:sports:2026-09-11"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
