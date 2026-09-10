package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
	"github.com/redis/go-redis/v9"
)

const exactKeyPrefix = "es:exact"

// RedisExact implements ExactStore with Redis Sorted Sets + daily time buckets.
//
// Per day key: es:exact:{segment}:{YYYY-MM-DD}
// Member: user_id, score: unix seconds of the Add (last-seen within the day).
// Re-adding the same user the same day updates the score (still one member).
//
// Count unions members across the retention window via ZUNIONSTORE into a
// short-lived temp key, then ZCARD. That yields an exact unique count even when
// a user appears in multiple days.
type RedisExact struct {
	rdb            redis.Cmdable
	clock          timebucket.Clock
	retentionDays  int
}

// NewRedisExact creates an ExactStore.
// clock may be nil (system UTC). retentionDays < 1 falls back to DefaultRetentionDays.
func NewRedisExact(rdb redis.Cmdable, clock timebucket.Clock, retentionDays int) *RedisExact {
	if clock == nil {
		clock = timebucket.SystemClock{}
	}
	return &RedisExact{
		rdb:           rdb,
		clock:         clock,
		retentionDays: timebucket.NormalizeRetention(retentionDays),
	}
}

func (s *RedisExact) Add(ctx context.Context, userID, segment string) error {
	if userID == "" || segment == "" {
		return fmt.Errorf("user_id and segment are required")
	}
	bucket := timebucket.Current(s.clock)
	key := bucket.Key(exactKeyPrefix, segment)
	score := float64(s.clock.Now().Unix())

	logging.Debug(ctx, logging.EventRedisZAdd,
		"component", "store",
		"backend", "exact",
		"key", key,
		"bucket", bucket.Format(),
		"user_id", userID,
		"segment", segment,
		"retention_days", s.retentionDays,
	)

	pipe := s.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: userID})
	pipe.Expire(ctx, key, timebucket.TTL(s.retentionDays))
	if _, err := pipe.Exec(ctx); err != nil {
		logging.Error(ctx, logging.EventRedisZAddFailed,
			"component", "store",
			"backend", "exact",
			"code", logging.CodeStorageError,
			"key", key,
			"err", err,
		)
		return fmt.Errorf("exact add: %w", err)
	}
	return nil
}

func (s *RedisExact) Count(ctx context.Context, segment string) (int64, error) {
	if segment == "" {
		return 0, fmt.Errorf("segment is required")
	}

	buckets := timebucket.Active(s.clock, s.retentionDays)
	keys := make([]string, 0, len(buckets))
	for _, b := range buckets {
		keys = append(keys, b.Key(exactKeyPrefix, segment))
	}

	existing, err := existingKeys(ctx, s.rdb, keys)
	if err != nil {
		logging.Error(ctx, logging.EventRedisExistsFailed,
			"component", "store",
			"backend", "exact",
			"code", logging.CodeStorageError,
			"segment", segment,
			"err", err,
		)
		return 0, err
	}
	logging.Debug(ctx, logging.EventRedisExactCountBuckets,
		"component", "store",
		"backend", "exact",
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
		n, err := s.rdb.ZCard(ctx, existing[0]).Result()
		if err != nil {
			logging.Error(ctx, logging.EventRedisZCardFailed,
				"component", "store",
				"backend", "exact",
				"code", logging.CodeStorageError,
				"key", existing[0],
				"err", err,
			)
			return 0, fmt.Errorf("exact zcard: %w", err)
		}
		return n, nil
	}

	tmp := fmt.Sprintf("es:tmp:exact:%s:%d", segment, time.Now().UnixNano())
	defer s.rdb.Del(ctx, tmp)

	if err := s.rdb.ZUnionStore(ctx, tmp, &redis.ZStore{Keys: existing}).Err(); err != nil {
		logging.Error(ctx, logging.EventRedisZUnionStoreFailed,
			"component", "store",
			"backend", "exact",
			"code", logging.CodeStorageError,
			"segment", segment,
			"keys", len(existing),
			"err", err,
		)
		return 0, fmt.Errorf("exact zunionstore: %w", err)
	}
	_ = s.rdb.Expire(ctx, tmp, 30*time.Second).Err()

	n, err := s.rdb.ZCard(ctx, tmp).Result()
	if err != nil {
		logging.Error(ctx, logging.EventRedisZCardFailed,
			"component", "store",
			"backend", "exact",
			"code", logging.CodeStorageError,
			"key", tmp,
			"err", err,
		)
		return 0, fmt.Errorf("exact count zcard: %w", err)
	}
	return n, nil
}

func existingKeys(ctx context.Context, rdb redis.Cmdable, keys []string) ([]string, error) {
	pipe := rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.Exists(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("exists check: %w", err)
	}
	out := make([]string, 0, len(keys))
	for i, cmd := range cmds {
		n, err := cmd.Result()
		if err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, keys[i])
		}
	}
	return out, nil
}
