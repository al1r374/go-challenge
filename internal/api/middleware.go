package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
)

// Middleware logs each HTTP request with method, path, status, duration, and request_id.
// It also propagates/creates an X-Request-ID for cross-layer tracing.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = newRequestID()
		}
		ctx := logging.WithRequestID(r.Context(), reqID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", reqID)

		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		attrs := []any{
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		}
		if r.URL.Path == "/healthz" && rw.status < 400 {
			logging.Debug(r.Context(), logging.EventHTTPRequest, attrs...)
			return
		}
		switch {
		case rw.status >= 500:
			logging.Error(r.Context(), logging.EventHTTPRequest, attrs...)
		case rw.status >= 400:
			logging.Warn(r.Context(), logging.EventHTTPRequest, attrs...)
		default:
			logging.Info(r.Context(), logging.EventHTTPRequest, attrs...)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}
