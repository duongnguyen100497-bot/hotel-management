package config_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/config"
)

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://hotel:secret@localhost:5432/hotel?sslmode=disable",
		"REDIS_URL":            "redis://localhost:6379/0",
		"JWT_ACCESS_SECRET":    "super-secret-access-key",
		"JWT_REFRESH_SECRET":   "super-secret-refresh-key",
		"SEPAY_WEBHOOK_SECRET": "sepay-webhook-secret-key",
		"MINIO_ENDPOINT":       "localhost:9000",
		"MINIO_ACCESS_KEY":     "minioadmin",
		"MINIO_SECRET_KEY":     "minio-secret-value",
		"MINIO_BUCKET":         "hotel-media",
	}
}

func mapLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadFrom_ValidDefaults(t *testing.T) {
	cfg, err := config.LoadFrom(mapLookup(validEnv()))
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	if cfg.AppEnv != config.EnvDevelopment {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, config.EnvDevelopment)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if cfg.HotelTimezone != "Asia/Ho_Chi_Minh" || cfg.HotelLocation == nil {
		t.Errorf("HotelTimezone = %q, HotelLocation = %v", cfg.HotelTimezone, cfg.HotelLocation)
	}
	if cfg.UploadMaxBytes != 5*1024*1024 {
		t.Errorf("UploadMaxBytes = %d, want 5242880", cfg.UploadMaxBytes)
	}
	if cfg.SMTPPort != 1025 || cfg.SMTPHost != "localhost" {
		t.Errorf("SMTP = %s:%d, want localhost:1025", cfg.SMTPHost, cfg.SMTPPort)
	}
}

func TestLoadFrom_MissingRequiredVars(t *testing.T) {
	required := []string{
		"DATABASE_URL",
		"REDIS_URL",
		"JWT_ACCESS_SECRET",
		"JWT_REFRESH_SECRET",
		"SEPAY_WEBHOOK_SECRET",
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_BUCKET",
	}

	for _, missing := range required {
		t.Run("missing_"+missing, func(t *testing.T) {
			env := validEnv()
			delete(env, missing)
			_, err := config.LoadFrom(mapLookup(env))
			if err == nil {
				t.Fatalf("expected error when %s is missing", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Errorf("error %q does not mention %s", err.Error(), missing)
			}
		})
	}
}

func TestLoadFrom_MinIODisabledDoesNotRequireMinIOVars(t *testing.T) {
	env := validEnv()
	env["MINIO_ENABLED"] = "false"
	delete(env, "MINIO_ENDPOINT")
	delete(env, "MINIO_ACCESS_KEY")
	delete(env, "MINIO_SECRET_KEY")
	delete(env, "MINIO_BUCKET")

	cfg, err := config.LoadFrom(mapLookup(env))
	if err != nil {
		t.Fatalf("expected valid config when MINIO_ENABLED=false, got: %v", err)
	}
	if cfg.MinIOEnabled {
		t.Errorf("expected MinIOEnabled=false")
	}
}

func TestLoadFrom_InvalidValues(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "staging"
	env["HOTEL_TIMEZONE"] = "Invalid/Zone"
	env["SHUTDOWN_TIMEOUT"] = "-5s"
	env["SMTP_PORT"] = "70000"

	_, err := config.LoadFrom(mapLookup(env))
	if err == nil {
		t.Fatal("expected error for invalid values")
	}
	for _, want := range []string{"APP_ENV", "HOTEL_TIMEZONE", "SHUTDOWN_TIMEOUT", "SMTP_PORT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %s", err.Error(), want)
		}
	}
}

func TestConfig_LogValueRedactsSecrets(t *testing.T) {
	cfg, err := config.LoadFrom(mapLookup(validEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("loaded config", "cfg", cfg)

	out := buf.String()
	for _, secret := range []string{
		"super-secret-access-key",
		"super-secret-refresh-key",
		"sepay-webhook-secret-key",
		"minio-secret-value",
		"hotel:secret@",
	} {
		if strings.Contains(out, secret) {
			t.Errorf("config log leaked secret %q in output: %s", secret, out)
		}
	}
}
