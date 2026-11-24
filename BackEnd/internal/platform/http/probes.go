package platformhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// CheckFunc verifies readiness of a backing dependency.
type CheckFunc func(ctx context.Context) error

// DependencyCheck binds a logical dependency name to its readiness probe.
type DependencyCheck struct {
	Name  string
	Check CheckFunc
}

type healthResponse struct {
	Status string `json:"status"`
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
	Failed []string          `json:"failed,omitempty"`
}

// HealthzHandler responds 200 OK whenever the process is alive.
func HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	}
}

// ReadyzHandler runs all dependency checks concurrently within timeout and returns
// 200 OK if all succeed or 503 Service Unavailable with failed dependency names.
func ReadyzHandler(timeout time.Duration, deps ...DependencyCheck) http.HandlerFunc {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		checks := make(map[string]string, len(deps))
		var failed []string
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, dep := range deps {
			dep := dep
			wg.Add(1)
			go func() {
				defer wg.Done()
				var err error
				if dep.Check != nil {
					err = dep.Check(ctx)
				}
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					checks[dep.Name] = "unavailable"
					failed = append(failed, dep.Name)
				} else {
					checks[dep.Name] = "ok"
				}
			}()
		}
		wg.Wait()

		sort.Strings(failed)
		resp := readyResponse{
			Status: "ok",
			Checks: checks,
		}
		status := http.StatusOK
		if len(failed) > 0 {
			resp.Status = "unavailable"
			resp.Failed = failed
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
