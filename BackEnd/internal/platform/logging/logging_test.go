package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
)

func TestLogger_RedactsSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("development", &buf)

	logger.Info("auth attempt",
		slog.String("email", "user@example.com"),
		slog.String("password", "PlainPassword123!"),
		slog.String("refresh_token", "tok_live_abc"),
		slog.String("jwt_secret", "sec_123"),
		slog.String("Authorization", "Bearer eyJhbGciOi..."),
		slog.String("government_id", "012345678901"),
		slog.String("bank_description", "BOOKING-XYZ-999"),
		slog.Group("nested",
			slog.String("resetToken", "reset-999"),
			slog.String("safe_field", "keep-me"),
		),
		slog.Any("payload", map[string]any{
			"password_hash": "hash_val",
			"room_number":   "101",
		}),
	)

	raw := buf.String()
	for _, secret := range []string{
		"PlainPassword123!",
		"tok_live_abc",
		"sec_123",
		"eyJhbGciOi",
		"012345678901",
		"BOOKING-XYZ-999",
		"reset-999",
		"hash_val",
	} {
		if strings.Contains(raw, secret) {
			t.Errorf("log output leaked sensitive value %q: %s", secret, raw)
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("log output is not valid JSON: %v\nraw: %s", err, raw)
	}
	if parsed["email"] != "user@example.com" {
		t.Errorf("email = %v, want user@example.com", parsed["email"])
	}
	if parsed["password"] != logging.RedactedValue {
		t.Errorf("password = %v, want %s", parsed["password"], logging.RedactedValue)
	}
	if parsed["government_id"] != logging.RedactedValue {
		t.Errorf("government_id = %v, want %s", parsed["government_id"], logging.RedactedValue)
	}
	if parsed["bank_description"] != logging.RedactedValue {
		t.Errorf("bank_description = %v, want %s", parsed["bank_description"], logging.RedactedValue)
	}
}

func TestRedactMap_DoesNotMutateInput(t *testing.T) {
	original := map[string]any{
		"name":          "Alice",
		"government_id": "999999999",
		"meta": map[string]any{
			"api_key": "k-123",
			"status":  "active",
		},
	}

	redacted := logging.RedactMap(original)
	if redacted["government_id"] != logging.RedactedValue {
		t.Fatalf("expected government_id to be redacted")
	}
	if original["government_id"] != "999999999" {
		t.Fatalf("RedactMap mutated original input")
	}
	meta := redacted["meta"].(map[string]any)
	if meta["api_key"] != logging.RedactedValue || meta["status"] != "active" {
		t.Fatalf("unexpected nested redaction: %+v", meta)
	}
}
