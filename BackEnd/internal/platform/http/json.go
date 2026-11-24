package platformhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// PageInfo describes cursor pagination metadata.
type PageInfo struct {
	Limit      int     `json:"limit"`
	Cursor     *string `json:"cursor,omitempty"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

// PageResponse wraps cursor-paginated list endpoints.
type PageResponse[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// WriteJSON encodes payload as JSON with the provided HTTP status code.
func WriteJSON(c *gin.Context, status int, payload any) {
	if payload == nil {
		c.Status(status)
		return
	}
	c.JSON(status, payload)
}

// DecodeJSON decodes a JSON request body strictly (DisallowUnknownFields, single JSON value).
// Returns apperror.PayloadTooLarge on MaxBytesError or apperror.Validation on malformed/unknown fields.
func DecodeJSON(c *gin.Context, dst any) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return apperror.Validation("request body is required")
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return apperror.PayloadTooLarge("request body exceeds maximum allowed size")
		}
		if errors.Is(err, io.EOF) {
			return apperror.Validation("request body must not be empty")
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return apperror.Validation("malformed JSON body")
		}
		var unmarshalTypeErr *json.UnmarshalTypeError
		if errors.As(err, &unmarshalTypeErr) {
			return apperror.Validation("invalid field type", apperror.FieldError{
				Field:   unmarshalTypeErr.Field,
				Message: fmt.Sprintf("expected %s", unmarshalTypeErr.Type.String()),
			})
		}
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return apperror.Validation("unknown field in request body", apperror.FieldError{
				Field:   field,
				Message: "field is not allowed",
			})
		}
		return apperror.Validation("invalid JSON payload")
	}

	// Ensure there is no trailing data after the first JSON object.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return apperror.PayloadTooLarge("request body exceeds maximum allowed size")
		}
		return apperror.Validation("request body must contain a single JSON object")
	}
	return nil
}

// ParseCursorParams parses ?limit= and ?cursor= query parameters safely.
func ParseCursorParams(c *gin.Context, defaultLimit, maxLimit int) (int, string, error) {
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	if maxLimit < defaultLimit {
		maxLimit = 100
	}

	limit := defaultLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > maxLimit {
			return 0, "", apperror.Validation("invalid pagination parameter", apperror.FieldError{
				Field:   "limit",
				Message: fmt.Sprintf("must be an integer between 1 and %d", maxLimit),
			})
		}
		limit = parsed
	}
	cursor := strings.TrimSpace(c.Query("cursor"))
	return limit, cursor, nil
}

// IdempotencyKey extracts the trimmed Idempotency-Key header value.
func IdempotencyKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetHeader("Idempotency-Key"))
}

// IfMatch extracts the trimmed If-Match header value (unquoting optional weak/strong ETag quotes).
func IfMatch(c *gin.Context) string {
	if c == nil {
		return ""
	}
	raw := strings.TrimSpace(c.GetHeader("If-Match"))
	raw = strings.TrimPrefix(raw, "W/")
	return strings.Trim(raw, `"`)
}
