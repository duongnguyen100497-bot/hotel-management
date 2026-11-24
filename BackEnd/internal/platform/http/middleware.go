package platformhttp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

const (
	HeaderRequestID     = "X-Request-Id"
	HeaderCorrelationID = "X-Correlation-Id"

	DefaultBodyLimitBytes int64 = 1 << 20 // 1 MiB
)

type ctxKey string

const (
	ctxKeyRequestID     ctxKey = "request_id"
	ctxKeyCorrelationID ctxKey = "correlation_id"
)

// RequestIDFromContext returns the request ID stored in ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// CorrelationIDFromContext returns the correlation ID stored in ctx, or "".
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKeyCorrelationID).(string)
	return v
}

// RequestIDUUIDFromContext parses the request ID in ctx as *uuid.UUID if valid.
func RequestIDUUIDFromContext(ctx context.Context) *uuid.UUID {
	parsed, err := uuid.Parse(RequestIDFromContext(ctx))
	if err != nil {
		return nil
	}
	return &parsed
}

// CorrelationIDUUIDFromContext parses the correlation ID in ctx as *uuid.UUID if valid.
func CorrelationIDUUIDFromContext(ctx context.Context) *uuid.UUID {
	parsed, err := uuid.Parse(CorrelationIDFromContext(ctx))
	if err != nil {
		return nil
	}
	return &parsed
}

// WithRequestID returns a copy of ctx with the request ID attached.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithCorrelationID returns a copy of ctx with the correlation ID attached.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}

// RequestIDMiddleware attaches X-Request-Id and X-Correlation-Id to the context and response headers.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := sanitizeHeaderID(r.Header.Get(HeaderRequestID))
			if reqID == "" {
				reqID = uuid.NewString()
			}
			corrID := sanitizeHeaderID(r.Header.Get(HeaderCorrelationID))
			if corrID == "" {
				corrID = reqID
			}

			ctx := WithRequestID(r.Context(), reqID)
			ctx = WithCorrelationID(ctx, corrID)

			w.Header().Set(HeaderRequestID, reqID)
			w.Header().Set(HeaderCorrelationID, corrID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sanitizeHeaderID(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > 128 {
		return ""
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return ""
		}
	}
	return s
}

// PanicRecovery catches panics, logs stack traces server-side, and writes a clean 500 error envelope.
func PanicRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.Error("panic recovered in http handler",
							slog.Any("panic", rec),
							slog.String("method", r.Method),
							slog.String("path", r.URL.Path),
							slog.String("request_id", RequestIDFromContext(r.Context())),
							slog.String("stack", string(debug.Stack())),
						)
					}
					WriteError(w, r, apperror.Internal(fmt.Errorf("panic: %v", rec)))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(b)
	sr.bytes += n
	return n, err
}

// StructuredRequestLogger emits one structured JSON log per completed HTTP request.
func StructuredRequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if logger != nil {
				logger.Info("http request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rec.status),
					slog.Int("bytes", rec.bytes),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
					slog.String("request_id", RequestIDFromContext(r.Context())),
					slog.String("correlation_id", CorrelationIDFromContext(r.Context())),
				)
			}
		})
	}
}

// CORS enforces an exact origin allowlist without wildcard credential reflection.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Request-Id, X-Correlation-Id")
					w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id, X-Correlation-Id, ETag")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit caps incoming request body size (defaults to 1 MiB).
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultBodyLimitBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				WriteError(w, r, apperror.PayloadTooLarge("request body exceeds maximum allowed size"))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter defines the contract for in-memory or Redis-backed rate limiters.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

type fixedWindowEntry struct {
	count   int
	resetAt time.Time
}

// InMemoryRateLimiter implements a thread-safe fixed-window RateLimiter.
type InMemoryRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	nowFunc func() time.Time
	entries map[string]fixedWindowEntry
}

// NewInMemoryRateLimiter constructs an in-memory rate limiter allowing limit requests per window.
func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &InMemoryRateLimiter{
		limit:   limit,
		window:  window,
		nowFunc: time.Now,
		entries: make(map[string]fixedWindowEntry),
	}
}

// Allow checks whether key may perform another request within the current window.
func (l *InMemoryRateLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	now := l.nowFunc()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || !now.Before(entry.resetAt) {
		l.entries[key] = fixedWindowEntry{
			count:   1,
			resetAt: now.Add(l.window),
		}
		return true, 0, nil
	}
	if entry.count >= l.limit {
		return false, time.Until(entry.resetAt), nil
	}
	entry.count++
	l.entries[key] = entry
	return true, 0, nil
}

// RateLimitMiddleware applies a RateLimiter to requests keyed by keyFunc (defaults to client IP).
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = ClientIP
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			allowed, retryAfter, err := limiter.Allow(r.Context(), keyFunc(r))
			if err != nil {
				WriteError(w, r, apperror.ServiceUnavailable("rate limiter unavailable"))
				return
			}
			if !allowed {
				secs := int(retryAfter.Seconds())
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				WriteError(w, r, apperror.RateLimited("too many requests"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the caller's IP address without port.
func ClientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
