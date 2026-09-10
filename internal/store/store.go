// Package store defines segment membership storage backends.
//
// Exact vs Approximate (trade-offs)
// ---------------------------------
// Exact (Sorted Set + daily buckets):
//   + Precise unique user counts across the retention window.
//   + Supports membership introspection if needed later.
//   − Memory ≈ O(unique users × active days); millions of users → large RAM.
//   − Count needs a multi-key union (ZUNIONSTORE) → slower at high cardinality.
//
// Approximate (HyperLogLog + daily buckets):
//   + ~12 KB per bucket regardless of cardinality; excellent for millions of users.
//   + Count is a cheap PFMERGE + PFCOUNT of ≤N keys (ES_RETENTION_DAYS).
//   − ~0.81% standard error; not suitable when billing/legal needs exact figures.
//
// Both share the same time-bucket scheme (see internal/timebucket).
package store

import "context"

// SegmentStore persists user→segment activity and answers unique-user counts
// over the active retention window.
type SegmentStore interface {
	// Add records that userID was seen in segment at the current bucket.
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
