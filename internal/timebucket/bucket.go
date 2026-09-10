// Package timebucket divides calendar time into daily buckets used by both
// Exact (Sorted Set) and Approximate (HyperLogLog) counting backends.
//
// How it works
// ------------
// Time is split into UTC calendar days. Each day gets its own Redis key:
//
//	es:{mode}:{segment}:{YYYY-MM-DD}
//
// Only the last N buckets (retention days, from ES_RETENTION_DAYS) are "active".
// Older keys are ignored by Count and given a TTL so Redis eventually drops them.
//
// Why daily buckets?
//   - Sliding retention window without scanning every user record.
//   - Cheap expiry: set TTL ≈ retentionDays+1 on each key at write time.
//   - Count = union (exact) or merge (HLL) of the last N keys.
package timebucket

import (
	"fmt"
	"time"
)

// DefaultRetentionDays is used when ES_RETENTION_DAYS is unset.
const DefaultRetentionDays = 14

// Clock abstracts time so tests can freeze "today".
type Clock interface {
	Now() time.Time
}

// SystemClock uses time.Now in UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// FixedClock returns a constant instant (tests).
type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T.UTC() }

// ID is a daily bucket identifier (UTC date).
type ID struct {
	Day time.Time // truncated to midnight UTC
}

// Format returns YYYY-MM-DD.
func (id ID) Format() string {
	return id.Day.Format("2006-01-02")
}

// Key builds a Redis key: prefix:segment:YYYY-MM-DD.
func (id ID) Key(prefix, segment string) string {
	return fmt.Sprintf("%s:%s:%s", prefix, segment, id.Format())
}

// Current returns today's UTC bucket for clock.
func Current(clock Clock) ID {
	now := clock.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return ID{Day: day}
}

// NormalizeRetention returns days if valid (>=1), otherwise DefaultRetentionDays.
func NormalizeRetention(days int) int {
	if days < 1 {
		return DefaultRetentionDays
	}
	return days
}

// Active returns the last retentionDays bucket IDs ending at today (inclusive),
// newest first. Buckets older than the window are excluded.
func Active(clock Clock, retentionDays int) []ID {
	days := NormalizeRetention(retentionDays)
	today := Current(clock)
	out := make([]ID, 0, days)
	for i := 0; i < days; i++ {
		out = append(out, ID{Day: today.Day.AddDate(0, 0, -i)})
	}
	return out
}

// TTL is how long Redis should keep a bucket key.
// One extra day of slack covers clock skew / late writes near midnight.
func TTL(retentionDays int) time.Duration {
	days := NormalizeRetention(retentionDays)
	return time.Duration(days+1) * 24 * time.Hour
}
