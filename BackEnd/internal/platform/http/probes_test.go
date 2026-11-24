package platformhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	platformhttp.HealthzHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestReadyzHandler_AllHealthy(t *testing.T) {
	handler := platformhttp.ReadyzHandler(2*time.Second,
		platformhttp.DependencyCheck{Name: "postgres", Check: func(context.Context) error { return nil }},
		platformhttp.DependencyCheck{Name: "redis", Check: func(context.Context) error { return nil }},
	)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReadyzHandler_DependencyFailureReturns503(t *testing.T) {
	handler := platformhttp.ReadyzHandler(2*time.Second,
		platformhttp.DependencyCheck{Name: "postgres", Check: func(context.Context) error { return nil }},
		platformhttp.DependencyCheck{Name: "redis", Check: func(context.Context) error { return errors.New("connection refused") }},
	)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
		Failed []string          `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Status != "unavailable" {
		t.Errorf("status = %q, want unavailable", body.Status)
	}
	if len(body.Failed) != 1 || body.Failed[0] != "redis" {
		t.Errorf("failed = %v, want [redis]", body.Failed)
	}
	if body.Checks["postgres"] != "ok" || body.Checks["redis"] != "unavailable" {
		t.Errorf("checks = %+v", body.Checks)
	}
}
