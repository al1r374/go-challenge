package logging

import (
	"context"
	"log/slog"
)

// Stable log event names for filtering (Loki/Datadog/etc.).
const (
	EventConfigLoaded   = "config.loaded"
	EventConfigFailed   = "config.failed"
	EventRedisConnected = "redis.connected"
	EventRedisFailed    = "redis.failed"
	EventRedisClosed    = "redis.closed"

	EventServerListening = "server.listening"
	EventServerFailed    = "server.failed"
	EventShutdownSignal  = "shutdown.signal"
	EventShutdownHTTP    = "shutdown.http_error"
	EventShutdownComplete = "shutdown.complete"

	EventHTTPRequest = "http.request"

	EventESAdd             = "es.add"
	EventESAddRejected     = "es.add_rejected"
	EventESAddFailed       = "es.add_failed"
	EventESCount           = "es.count"
	EventESCountRejected   = "es.count_rejected"
	EventESCountFailed     = "es.count_failed"

	EventRedisZAdd              = "redis.zadd"
	EventRedisZAddFailed        = "redis.zadd_failed"
	EventRedisZCardFailed       = "redis.zcard_failed"
	EventRedisZUnionStoreFailed = "redis.zunionstore_failed"
	EventRedisExistsFailed      = "redis.exists_failed"
	EventRedisExactCountBuckets = "redis.exact_count_buckets"

	EventRedisPFAdd         = "redis.pfadd"
	EventRedisPFAddFailed   = "redis.pfadd_failed"
	EventRedisPFCountFailed = "redis.pfcount_failed"
	EventRedisPFMergeFailed = "redis.pfmerge_failed"
	EventRedisApproxBuckets = "redis.approximate_count_buckets"
)

// Error codes attached to failed domain events (until a full apperr package exists).
const (
	CodeInvalidInput = "invalid_input"
	CodeStorageError = "storage_error"
	CodeInternal     = "internal"
)

func withEvent(event string, args []any) []any {
	out := make([]any, 0, len(args)+2)
	out = append(out, "event", event)
	out = append(out, args...)
	return out
}

// Debug logs at debug with a stable event name (also used as slog message).
func Debug(ctx context.Context, event string, args ...any) {
	slog.DebugContext(ctx, event, withEvent(event, args)...)
}

// Info logs at info with a stable event name.
func Info(ctx context.Context, event string, args ...any) {
	slog.InfoContext(ctx, event, withEvent(event, args)...)
}

// Warn logs at warn with a stable event name.
func Warn(ctx context.Context, event string, args ...any) {
	slog.WarnContext(ctx, event, withEvent(event, args)...)
}

// Error logs at error with a stable event name.
func Error(ctx context.Context, event string, args ...any) {
	slog.ErrorContext(ctx, event, withEvent(event, args)...)
}

// InfoNoCtx is for startup/shutdown outside a request context.
func InfoNoCtx(event string, args ...any) {
	slog.Info(event, withEvent(event, args)...)
}

// ErrorNoCtx is for startup/shutdown failures outside a request context.
func ErrorNoCtx(event string, args ...any) {
	slog.Error(event, withEvent(event, args)...)
}

// WarnNoCtx logs a warning without request context.
func WarnNoCtx(event string, args ...any) {
	slog.Warn(event, withEvent(event, args)...)
}
