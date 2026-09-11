// Package store defines segment membership storage backends.
//
// Exact vs Approximate (trade-offs)
// ---------------------------------
// Exact (one Sorted Set per segment, score = last-seen):
//   + Precise unique user counts across the retention window.
//   + Supports membership introspection if needed later.
//   + Count is a single ZCOUNT; memory ≈ O(unique users in window).
//   − Memory still grows with cardinality (not fixed-size like HLL).
//
// Approximate (HyperLogLog + daily buckets):
//   + ~12 KB per bucket regardless of cardinality; excellent for millions of users.
//   + Count is a cheap PFMERGE + PFCOUNT of ≤N keys (ES_RETENTION_DAYS).
//   − ~0.81% standard error; not suitable when billing/legal needs exact figures.
//
// Both use the same calendar retention window (see internal/timebucket).
// Only Approximate stores one Redis key per day.
package store

import "context"

// SegmentStore persists user→segment activity and answers unique-user counts
// over the active retention window.
type SegmentStore interface {
	// Add records that userID was seen in segment at the current time.
	Add(ctx context.Context, userID, segment string) error
	// Count returns unique users active in segment within the retention window.
	Count(ctx context.Context, segment string) (int64, error)
}

// ExactStore is the Sorted-Set backend.
type ExactStore interface {
	SegmentStore
}

// ApproximateStore is the HyperLogLog backend.
type ApproximateStore interface {
	SegmentStore
}
