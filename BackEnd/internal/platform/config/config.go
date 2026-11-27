package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config holds strongly-typed runtime configuration loaded from environment variables.
type Config struct {
	AppEnv             string
	HTTPAddr           string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string
	PublicBaseURL      string
	FrontendBaseURL    string
	HotelTimezone      string
	HotelLocation      *time.Location
	UploadMaxBytes     int64

	DatabaseURL string
	DBMaxConns  int32
	DBMinConns  int32

	RedisURL string

	MinIOEnabled       bool
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	MinIOUseSSL        bool
	MinIOPublicBaseURL string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	SePayWebhookSecret    string
	SePayReceivingAccount string

	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// LogValue redacts all secrets if Config is ever passed to slog.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("app_env", c.AppEnv),
		slog.String("http_addr", c.HTTPAddr),
		slog.Duration("shutdown_timeout", c.ShutdownTimeout),
		slog.String("public_base_url", c.PublicBaseURL),
		slog.String("frontend_base_url", c.FrontendBaseURL),
		slog.String("hotel_timezone", c.HotelTimezone),
		slog.Int64("upload_max_bytes", c.UploadMaxBytes),
		slog.Int("db_max_conns", int(c.DBMaxConns)),
		slog.Int("db_min_conns", int(c.DBMinConns)),
		slog.Bool("minio_enabled", c.MinIOEnabled),
		slog.String("minio_endpoint", c.MinIOEndpoint),
		slog.String("minio_bucket", c.MinIOBucket),
		slog.Bool("minio_use_ssl", c.MinIOUseSSL),
		slog.String("smtp_host", c.SMTPHost),
		slog.Int("smtp_port", c.SMTPPort),
		slog.String("smtp_from", c.SMTPFrom),
		slog.String("database_url", "[REDACTED]"),
		slog.String("redis_url", "[REDACTED]"),
		slog.String("jwt_access_secret", "[REDACTED]"),
		slog.String("jwt_refresh_secret", "[REDACTED]"),
		slog.String("sepay_webhook_secret", "[REDACTED]"),
		slog.String("minio_secret_key", "[REDACTED]"),
		slog.String("smtp_pass", "[REDACTED]"),
	)
}

// Load parses and validates configuration from process environment variables.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom parses and validates configuration using the supplied lookup function.
func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	var errs []error

	getString := func(key, fallback string) string {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return fallback
		}
		return strings.TrimSpace(v)
	}

	requireString := func(key string) string {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			errs = append(errs, fmt.Errorf("missing required env %s", key))
			return ""
		}
		return strings.TrimSpace(v)
	}

	getDuration := func(key string, fallback time.Duration) time.Duration {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return fallback
		}
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("invalid duration for %s: %q", key, v))
			return fallback
		}
		return d
	}

	getInt := func(key string, fallback int) int {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return fallback
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid integer for %s: %q", key, v))
			return fallback
		}
		return n
	}

	getInt64 := func(key string, fallback int64) int64 {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return fallback
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid int64 for %s: %q", key, v))
			return fallback
		}
		return n
	}

	getBool := func(key string, fallback bool) bool {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return fallback
		}
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid boolean for %s: %q", key, v))
			return fallback
		}
		return b
	}

	cfg := Config{
		AppEnv:                getString("APP_ENV", EnvDevelopment),
		HTTPAddr:              getString("HTTP_ADDR", ":8080"),
		ShutdownTimeout:       getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		CORSAllowedOrigins:    splitCSV(getString("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		PublicBaseURL:         getString("PUBLIC_BASE_URL", "http://localhost:8080"),
		FrontendBaseURL:       getString("FRONTEND_BASE_URL", "http://localhost:5173"),
		HotelTimezone:         getString("HOTEL_TIMEZONE", "Asia/Ho_Chi_Minh"),
		UploadMaxBytes:        getInt64("UPLOAD_MAX_BYTES", 5*1024*1024),
		DatabaseURL:           requireString("DATABASE_URL"),
		DBMaxConns:            int32(getInt("DB_MAX_CONNS", 20)),
		DBMinConns:            int32(getInt("DB_MIN_CONNS", 2)),
		RedisURL:              requireString("REDIS_URL"),
		MinIOEnabled:          getBool("MINIO_ENABLED", true),
		MinIOUseSSL:           getBool("MINIO_USE_SSL", false),
		MinIOPublicBaseURL:    getString("MINIO_PUBLIC_BASE_URL", ""),
		JWTAccessSecret:       requireString("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:      requireString("JWT_REFRESH_SECRET"),
		JWTAccessTTL:          getDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:         getDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		SePayWebhookSecret:    requireString("SEPAY_WEBHOOK_SECRET"),
		SePayReceivingAccount: getString("SEPAY_RECEIVING_ACCOUNT", ""),
		SMTPHost:              getString("SMTP_HOST", "localhost"),
		SMTPPort:              getInt("SMTP_PORT", 1025),
		SMTPUser:              getString("SMTP_USER", ""),
		SMTPPass:              getString("SMTP_PASS", ""),
		SMTPFrom:              getString("SMTP_FROM", "no-reply@hotel.local"),
	}

	if cfg.MinIOEnabled {
		cfg.MinIOEndpoint = requireString("MINIO_ENDPOINT")
		cfg.MinIOAccessKey = requireString("MINIO_ACCESS_KEY")
		cfg.MinIOSecretKey = requireString("MINIO_SECRET_KEY")
		cfg.MinIOBucket = requireString("MINIO_BUCKET")
	} else {
		cfg.MinIOEndpoint = getString("MINIO_ENDPOINT", "")
		cfg.MinIOAccessKey = getString("MINIO_ACCESS_KEY", "")
		cfg.MinIOSecretKey = getString("MINIO_SECRET_KEY", "")
		cfg.MinIOBucket = getString("MINIO_BUCKET", "")
	}

	switch cfg.AppEnv {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("invalid APP_ENV %q: must be development, test, or production", cfg.AppEnv))
	}

	loc, err := time.LoadLocation(cfg.HotelTimezone)
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid HOTEL_TIMEZONE %q: %w", cfg.HotelTimezone, err))
	} else {
		cfg.HotelLocation = loc
	}

	if cfg.UploadMaxBytes <= 0 {
		errs = append(errs, fmt.Errorf("UPLOAD_MAX_BYTES must be positive, got %d", cfg.UploadMaxBytes))
	}
	if cfg.DBMaxConns <= 0 {
		errs = append(errs, fmt.Errorf("DB_MAX_CONNS must be positive, got %d", cfg.DBMaxConns))
	}
	if cfg.DBMinConns < 0 || cfg.DBMinConns > cfg.DBMaxConns {
		errs = append(errs, fmt.Errorf("DB_MIN_CONNS (%d) must be between 0 and DB_MAX_CONNS (%d)", cfg.DBMinConns, cfg.DBMaxConns))
	}
	if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
		errs = append(errs, fmt.Errorf("SMTP_PORT must be in 1..65535, got %d", cfg.SMTPPort))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
