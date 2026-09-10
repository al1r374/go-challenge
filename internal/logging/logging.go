package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type requestIDKeyType struct{}

// RequestIDKey is the context key for HTTP request ids (shared with api middleware via WithRequestID).
var RequestIDKey = requestIDKeyType{}

// WithRequestID stores a request id on ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// RequestID returns the request id from ctx, or "".
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// Setup configures the global slog logger.
// level: debug|info|warn|error (default info)
// format: text|json (default text)
func Setup(level, format string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	out := io.Writer(os.Stdout)
	if strings.EqualFold(format, "json") {
		h = slog.NewJSONHandler(out, opts)
	} else {
		h = slog.NewTextHandler(out, opts)
	}
	logger := slog.New(&contextHandler{Handler: h})
	slog.SetDefault(logger)
	return logger
}

// contextHandler injects request_id from context into every log record.
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.Add("request_id", id)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
