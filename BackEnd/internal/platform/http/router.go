package platformhttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// RouterConfig configures base middlewares, probes, and API mounting for the root chi router.
type RouterConfig struct {
	Logger             *slog.Logger
	CORSAllowedOrigins []string
	BodyLimitBytes     int64
	ReadyzTimeout      time.Duration
	ReadyzChecks       []DependencyCheck
}

// NewRouter constructs the canonical chi router with base middlewares, /healthz, /readyz,
// and the /api/v1 route group.
func NewRouter(cfg RouterConfig, mountAPI func(r chi.Router)) *chi.Mux {
	r := chi.NewRouter()

	r.Use(RequestIDMiddleware())
	r.Use(PanicRecovery(cfg.Logger))
	r.Use(StructuredRequestLogger(cfg.Logger))
	r.Use(CORS(cfg.CORSAllowedOrigins))
	r.Use(BodyLimit(cfg.BodyLimitBytes))

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, apperror.NotFound("endpoint not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, apperror.Validation("method not allowed"))
	})

	r.Get("/healthz", HealthzHandler())
	r.Get("/readyz", ReadyzHandler(cfg.ReadyzTimeout, cfg.ReadyzChecks...))

	r.Route("/api/v1", func(api chi.Router) {
		if mountAPI != nil {
			mountAPI(api)
		}
	})

	return r
}
