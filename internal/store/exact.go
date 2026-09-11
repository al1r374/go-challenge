package store

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/timebucket"
	"github.com/redis/go-redis/v9"
)

const exactKeyPrefix = "es:exact"

// RedisExact implements ExactStore with one Redis Sorted Set per segment.
//
// Key: es:exact:{segment}
// Member: user_id, score: unix seconds of last Add (last-seen).
// Re-adding the same user updates the score (still one member).
//
// Count uses ZCOUNT over scores in the calendar retention window.
// Add prunes members older than the window via ZREMRANGEBYSCORE and refreshes TTL.
type RedisExact struct {
	rdb           redis.Cmdable
	clock         timebucket.Clock
	retentionDays int
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

func exactKey(segment string) string {
	return fmt.Sprintf("%s:%s", exactKeyPrefix, segment)
}

func (s *RedisExact) Add(ctx context.Context, userID, segment string) error {
	if userID == "" || segment == "" {
		return fmt.Errorf("user_id and segment are required")
	}
	key := exactKey(segment)
	now := s.clock.Now()
	score := float64(now.Unix())
	windowStart := timebucket.WindowStart(s.clock, s.retentionDays).Unix()

	logging.Debug(ctx, logging.EventRedisZAdd,
		"component", "store",
		"backend", "exact",
		"key", key,
		"user_id", userID,
		"segment", segment,
		"retention_days", s.retentionDays,
		"window_start", windowStart,
	)

	pipe := s.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: userID})
	// Exclusive upper bound: drop scores strictly older than the window start.
	pipe.ZRemRangeByScore(ctx, key, "-inf", "("+strconv.FormatInt(windowStart, 10))
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

	key := exactKey(segment)
	windowStart := timebucket.WindowStart(s.clock, s.retentionDays).Unix()
	min := strconv.FormatInt(windowStart, 10)

	logging.Debug(ctx, logging.EventRedisExactCount,
		"component", "store",
		"backend", "exact",
		"segment", segment,
		"key", key,
		"retention_days", s.retentionDays,
		"window_start", windowStart,
	)

	n, err := s.rdb.ZCount(ctx, key, min, "+inf").Result()
	if err != nil {
		logging.Error(ctx, logging.EventRedisZCountFailed,
			"component", "store",
			"backend", "exact",
			"code", logging.CodeStorageError,
			"key", key,
			"err", err,
		)
		return 0, fmt.Errorf("exact zcount: %w", err)
	}
	return n, nil
}
