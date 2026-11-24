package platformhttp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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

// RequestIDMiddleware attaches X-Request-Id and X-Correlation-Id to the request context and response headers.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := sanitizeHeaderID(c.GetHeader(HeaderRequestID))
		if reqID == "" {
			reqID = uuid.NewString()
		}
		corrID := sanitizeHeaderID(c.GetHeader(HeaderCorrelationID))
		if corrID == "" {
			corrID = reqID
		}

		ctx := WithRequestID(c.Request.Context(), reqID)
		ctx = WithCorrelationID(ctx, corrID)
		c.Request = c.Request.WithContext(ctx)

		c.Header(HeaderRequestID, reqID)
		c.Header(HeaderCorrelationID, corrID)

		c.Next()
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
func PanicRecovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				if logger != nil {
					logger.Error("panic recovered in http handler",
						slog.Any("panic", rec),
						slog.String("method", c.Request.Method),
						slog.String("path", c.Request.URL.Path),
						slog.String("request_id", RequestIDFromContext(c.Request.Context())),
						slog.String("stack", string(debug.Stack())),
					)
				}
				WriteError(c, apperror.Internal(fmt.Errorf("panic: %v", rec)))
			}
		}()
		c.Next()
	}
}

// StructuredRequestLogger emits one structured JSON log per completed HTTP request.
func StructuredRequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if logger != nil {
			logger.Info("http request",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.Int("status", c.Writer.Status()),
				slog.Int("bytes", max(c.Writer.Size(), 0)),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", RequestIDFromContext(c.Request.Context())),
				slog.String("correlation_id", CorrelationIDFromContext(c.Request.Context())),
			)
		}
	}
}

// CORS enforces an exact origin allowlist without wildcard credential reflection.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Add("Vary", "Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, X-Request-Id, X-Correlation-Id")
				c.Header("Access-Control-Expose-Headers", "X-Request-Id, X-Correlation-Id, ETag")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodyLimit caps incoming request body size (defaults to 1 MiB).
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultBodyLimitBytes
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			WriteError(c, apperror.PayloadTooLarge("request body exceeds maximum allowed size"))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
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
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	if keyFunc == nil {
		keyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		allowed, retryAfter, err := limiter.Allow(c.Request.Context(), keyFunc(c))
		if err != nil {
			WriteError(c, apperror.ServiceUnavailable("rate limiter unavailable"))
			return
		}
		if !allowed {
			secs := int(retryAfter.Seconds())
			if secs < 1 {
				secs = 1
			}
			c.Header("Retry-After", strconv.Itoa(secs))
			WriteError(c, apperror.RateLimited("too many requests"))
			return
		}
		c.Next()
	}
}
