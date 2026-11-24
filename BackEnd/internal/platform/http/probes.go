package platformhttp

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
func HealthzHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse{Status: "ok"})
	}
}

// ReadyzHandler runs all dependency checks concurrently within timeout and returns
// 200 OK if all succeed or 503 Service Unavailable with failed dependency names.
func ReadyzHandler(timeout time.Duration, deps ...DependencyCheck) gin.HandlerFunc {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
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

		c.JSON(status, resp)
	}
}
