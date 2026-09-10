package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
	"github.com/redis/go-redis/v9"
)

const approxKeyPrefix = "es:approx"

// RedisApproximate implements ApproximateStore with Redis HyperLogLog + daily buckets.
//
// Per day key: es:approx:{segment}:{YYYY-MM-DD}
// PFADD user_id into today's HLL; Expire with retention TTL.
//
// Count PFMERGEs HLL keys in the retention window into a temporary key, then PFCOUNT.
// Redis HLL standard error is ~0.81%; memory stays ~12KB per bucket.
type RedisApproximate struct {
	rdb           redis.Cmdable
	clock         timebucket.Clock
	retentionDays int
}

// NewRedisApproximate creates an ApproximateStore.
// clock may be nil (system UTC). retentionDays < 1 falls back to DefaultRetentionDays.
func NewRedisApproximate(rdb redis.Cmdable, clock timebucket.Clock, retentionDays int) *RedisApproximate {
	if clock == nil {
		clock = timebucket.SystemClock{}
	}
	return &RedisApproximate{
		rdb:           rdb,
		clock:         clock,
		retentionDays: timebucket.NormalizeRetention(retentionDays),
	}
}

func (s *RedisApproximate) Add(ctx context.Context, userID, segment string) error {
	if userID == "" || segment == "" {
		return fmt.Errorf("user_id and segment are required")
	}
	bucket := timebucket.Current(s.clock)
	key := bucket.Key(approxKeyPrefix, segment)

	logging.Debug(ctx, logging.EventRedisPFAdd,
		"component", "store",
		"backend", "approximate",
		"key", key,
		"bucket", bucket.Format(),
		"user_id", userID,
		"segment", segment,
		"retention_days", s.retentionDays,
	)

	pipe := s.rdb.Pipeline()
	pipe.PFAdd(ctx, key, userID)
	pipe.Expire(ctx, key, timebucket.TTL(s.retentionDays))
	if _, err := pipe.Exec(ctx); err != nil {
		logging.Error(ctx, logging.EventRedisPFAddFailed,
			"component", "store",
			"backend", "approximate",
			"code", logging.CodeStorageError,
			"key", key,
			"err", err,
		)
		return fmt.Errorf("approximate add: %w", err)
	}
	return nil
}

func (s *RedisApproximate) Count(ctx context.Context, segment string) (int64, error) {
	if segment == "" {
		return 0, fmt.Errorf("segment is required")
	}

	buckets := timebucket.Active(s.clock, s.retentionDays)
	keys := make([]string, 0, len(buckets))
	for _, b := range buckets {
		keys = append(keys, b.Key(approxKeyPrefix, segment))
	}

	existing, err := existingKeys(ctx, s.rdb, keys)
	if err != nil {
		logging.Error(ctx, logging.EventRedisExistsFailed,
			"component", "store",
			"backend", "approximate",
			"code", logging.CodeStorageError,
			"segment", segment,
			"err", err,
		)
		return 0, err
	}
	logging.Debug(ctx, logging.EventRedisApproxBuckets,
		"component", "store",
		"backend", "approximate",
		"segment", segment,
		"retention_days", s.retentionDays,
		"window_keys", len(keys),
		"existing_keys", len(existing),
		"today", buckets[0].Format(),
	)

	if len(existing) == 0 {
		return 0, nil
	}
	if len(existing) == 1 {
		n, err := s.rdb.PFCount(ctx, existing[0]).Result()
		if err != nil {
			logging.Error(ctx, logging.EventRedisPFCountFailed,
				"component", "store",
				"backend", "approximate",
				"code", logging.CodeStorageError,
				"key", existing[0],
				"err", err,
			)
			return 0, fmt.Errorf("approximate pfcount: %w", err)
		}
		return n, nil
	}

	tmp := fmt.Sprintf("es:tmp:approx:%s:%d", segment, time.Now().UnixNano())
	defer s.rdb.Del(ctx, tmp)

	args := make([]string, 0, len(existing)+1)
	args = append(args, tmp)
	args = append(args, existing...)
	if err := s.rdb.PFMerge(ctx, args[0], args[1:]...).Err(); err != nil {
		logging.Error(ctx, logging.EventRedisPFMergeFailed,
			"component", "store",
			"backend", "approximate",
			"code", logging.CodeStorageError,
			"segment", segment,
			"keys", len(existing),
			"err", err,
		)
		return 0, fmt.Errorf("approximate pfmerge: %w", err)
	}
	_ = s.rdb.Expire(ctx, tmp, 30*time.Second).Err()

	n, err := s.rdb.PFCount(ctx, tmp).Result()
	if err != nil {
		logging.Error(ctx, logging.EventRedisPFCountFailed,
			"component", "store",
			"backend", "approximate",
			"code", logging.CodeStorageError,
			"key", tmp,
			"err", err,
		)
		return 0, fmt.Errorf("approximate count pfcount: %w", err)
	}
	return n, nil
}
