package platformhttp

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// RouterConfig configures base middlewares, probes, and API mounting for the root Gin engine.
type RouterConfig struct {
	Logger             *slog.Logger
	CORSAllowedOrigins []string
	BodyLimitBytes     int64
	ReadyzTimeout      time.Duration
	ReadyzChecks       []DependencyCheck
}

// NewRouter constructs the canonical Gin engine with base middlewares, /healthz, /readyz,
// and the /api/v1 route group.
func NewRouter(cfg RouterConfig, mountAPI func(api *gin.RouterGroup)) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.HandleMethodNotAllowed = true

	r.Use(
		RequestIDMiddleware(),
		PanicRecovery(cfg.Logger),
		StructuredRequestLogger(cfg.Logger),
		CORS(cfg.CORSAllowedOrigins),
		BodyLimit(cfg.BodyLimitBytes),
	)

	r.NoRoute(func(c *gin.Context) {
		WriteError(c, apperror.NotFound("endpoint not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		WriteError(c, apperror.Validation("method not allowed"))
	})

	r.GET("/healthz", HealthzHandler())
	r.GET("/readyz", ReadyzHandler(cfg.ReadyzTimeout, cfg.ReadyzChecks...))

	api := r.Group("/api/v1")
	if mountAPI != nil {
		mountAPI(api)
	}

	return r
}
