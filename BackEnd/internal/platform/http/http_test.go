package platformhttp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

func TestWriteError_MapsAllCanonicalStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsgNot string
	}{
		{"validation_422", apperror.Validation("invalid input", apperror.FieldError{Field: "email", Message: "required"}), http.StatusUnprocessableEntity, apperror.CodeValidationFailed, ""},
		{"unauthenticated_401", apperror.Unauthenticated("bad token"), http.StatusUnauthorized, apperror.CodeUnauthenticated, ""},
		{"forbidden_403", apperror.Forbidden("missing perm"), http.StatusForbidden, apperror.CodeForbidden, ""},
		{"not_found_404", apperror.NotFound("room not found"), http.StatusNotFound, apperror.CodeNotFound, ""},
		{"conflict_409", apperror.Conflict("already booked"), http.StatusConflict, apperror.CodeConflict, ""},
		{"version_conflict_409", apperror.VersionConflict("stale version"), http.StatusConflict, apperror.CodeVersionConflict, ""},
		{"rate_limited_429", apperror.RateLimited("slow down"), http.StatusTooManyRequests, apperror.CodeRateLimited, ""},
		{"unavailable_503", apperror.ServiceUnavailable("redis down"), http.StatusServiceUnavailable, apperror.CodeServiceUnavailable, ""},
		{"internal_500_hides_sql", errors.New("pq: duplicate key violates constraint users_email_key"), http.StatusInternalServerError, apperror.CodeInternalError, "users_email_key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			c.Request = req.WithContext(platformhttp.WithRequestID(req.Context(), "req-test-123"))

			platformhttp.WriteError(c, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var env platformhttp.ErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if env.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", env.Error.Code, tc.wantCode)
			}
			if env.Error.RequestID != "req-test-123" {
				t.Errorf("requestId = %q, want req-test-123", env.Error.RequestID)
			}
			if tc.wantMsgNot != "" && strings.Contains(rec.Body.String(), tc.wantMsgNot) {
				t.Errorf("500 response leaked internal detail %q: %s", tc.wantMsgNot, rec.Body.String())
			}
		})
	}
}

func TestRouter_PanicRecoveryReturnsClean500(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	r := platformhttp.NewRouter(platformhttp.RouterConfig{
		Logger: logger,
	}, func(api *gin.RouterGroup) {
		api.GET("/boom", func(*gin.Context) {
			panic("secret db crash trace")
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret db crash trace") {
		t.Fatalf("panic message leaked to client: %s", rec.Body.String())
	}
	var env platformhttp.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid error envelope: %v", err)
	}
	if env.Error.Code != apperror.CodeInternalError || env.Error.RequestID == "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if !strings.Contains(logBuf.String(), "panic recovered") {
		t.Errorf("expected panic log, got: %s", logBuf.String())
	}
}

func TestRouter_BodyLimitRejection(t *testing.T) {
	r := platformhttp.NewRouter(platformhttp.RouterConfig{
		BodyLimitBytes: 32,
	}, func(api *gin.RouterGroup) {
		api.POST("/items", func(c *gin.Context) {
			var dst map[string]any
			if err := platformhttp.DecodeJSON(c, &dst); err != nil {
				platformhttp.WriteError(c, err)
				return
			}
			platformhttp.WriteJSON(c, http.StatusOK, dst)
		})
	})

	// Case 1: Known Content-Length > limit
	oversized := `{"name":"this payload is definitely longer than 32 bytes"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(oversized))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("known length status = %d, want 413, body = %s", rec.Code, rec.Body.String())
	}

	// Case 2: Chunked/unknown Content-Length > limit caught by MaxBytesReader + DecodeJSON
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/items", io.NopCloser(strings.NewReader(oversized)))
	req2.ContentLength = -1
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unknown length status = %d, want 413, body = %s", rec2.Code, rec2.Body.String())
	}
}

func TestDecodeJSON_RejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type createDTO struct {
		Name string `json:"name"`
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(`{"name":"Deluxe","extra":"forbidden"}`))

	var dto createDTO
	err := platformhttp.DecodeJSON(c, &dto)
	if err == nil {
		t.Fatal("expected error for unknown JSON field")
	}
	appErr, ok := apperror.As(err)
	if !ok || appErr.Code != apperror.CodeValidationFailed {
		t.Fatalf("got %v, want validation_failed", err)
	}
	if len(appErr.Fields) != 1 || appErr.Fields[0].Field != "extra" {
		t.Fatalf("fields = %+v, want field=extra", appErr.Fields)
	}
}

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	limiter := platformhttp.NewInMemoryRateLimiter(2, time.Minute)
	r := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		api.POST("/auth/login", platformhttp.RateLimitMiddleware(limiter, nil), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	})

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if i <= 2 && rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request 3: status = %d, want 429", rec.Code)
		}
	}
}

func TestCORS_ExactAllowlistOnly(t *testing.T) {
	r := platformhttp.NewRouter(platformhttp.RouterConfig{
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}, nil)

	reqAllowed := httptest.NewRequest(http.MethodOptions, "/api/v1/rooms", nil)
	reqAllowed.Header.Set("Origin", "http://localhost:5173")
	recAllowed := httptest.NewRecorder()
	r.ServeHTTP(recAllowed, reqAllowed)
	if got := recAllowed.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allowed origin header = %q, want http://localhost:5173", got)
	}

	reqDenied := httptest.NewRequest(http.MethodOptions, "/api/v1/rooms", nil)
	reqDenied.Header.Set("Origin", "https://evil.example")
	recDenied := httptest.NewRecorder()
	r.ServeHTTP(recDenied, reqDenied)
	if got := recDenied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin should not be reflected, got %q", got)
	}
}

func TestHeadersAndPaginationHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rooms?limit=25&cursor=abc123", nil)
	req.Header.Set("Idempotency-Key", "  idem-001 ")
	req.Header.Set("If-Match", `W/"7"`)
	c.Request = req

	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 25 || cursor != "abc123" {
		t.Fatalf("got limit=%d cursor=%q", limit, cursor)
	}
	if got := platformhttp.IdempotencyKey(c); got != "idem-001" {
		t.Errorf("IdempotencyKey = %q, want idem-001", got)
	}
	if got := platformhttp.IfMatch(c); got != "7" {
		t.Errorf("IfMatch = %q, want 7", got)
	}
}
