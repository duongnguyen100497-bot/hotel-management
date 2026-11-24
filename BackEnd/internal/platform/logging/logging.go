package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// RedactedValue replaces sensitive field values in structured logs and audit payloads.
const RedactedValue = "[REDACTED]"

var sensitiveSubstrings = []string{
	"password",
	"passwd",
	"token",
	"secret",
	"authorization",
	"governmentid",
	"bankdescription",
	"apikey",
	"smtppass",
	"privatekey",
	"cookie",
}

// IsSensitiveKey returns true when a field key should be redacted.
func IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	for _, s := range sensitiveSubstrings {
		if strings.Contains(normalized, s) {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c == '_' || c == '-' || c == '.' || c == ' ' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// RedactMap returns a defensive copy of m with all sensitive keys recursively redacted.
func RedactMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if IsSensitiveKey(k) {
			out[k] = RedactedValue
			continue
		}
		out[k] = redactAny(v)
	}
	return out
}

func redactAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return RedactMap(typed)
	case []any:
		cp := make([]any, len(typed))
		for i, elem := range typed {
			cp[i] = redactAny(elem)
		}
		return cp
	default:
		return v
	}
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if IsSensitiveKey(a.Key) {
		return slog.String(a.Key, RedactedValue)
	}
	switch a.Value.Kind() {
	case slog.KindGroup:
		attrs := a.Value.Group()
		cleaned := make([]slog.Attr, len(attrs))
		for i, sub := range attrs {
			cleaned[i] = redactAttr(nil, sub)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(cleaned...)}
	case slog.KindAny:
		if m, ok := a.Value.Any().(map[string]any); ok {
			return slog.Any(a.Key, RedactMap(m))
		}
	}
	return a
}

// New constructs a JSON slog.Logger with automatic redaction of sensitive keys.
func New(env string, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}
	level := slog.LevelInfo
	if env == "development" || env == "test" {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	})
	return slog.New(handler)
}
