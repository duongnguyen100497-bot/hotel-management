package platformhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// ErrorEnvelope is the canonical REST error body:
// { "error": { "code": "...", "message": "...", "fields": [...], "requestId": "..." } }.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody holds the serialized error details.
type ErrorBody struct {
	Code      string                `json:"code"`
	Message   string                `json:"message"`
	Fields    []apperror.FieldError `json:"fields,omitempty"`
	RequestID string                `json:"requestId,omitempty"`
}

// StatusFromError maps domain/application errors to HTTP status codes and safe client payloads.
func StatusFromError(err error) (int, ErrorBody) {
	if appErr, ok := apperror.As(err); ok {
		status := statusFromCode(appErr.Code)
		msg := appErr.Message
		if status == http.StatusInternalServerError {
			msg = "internal server error"
		}
		return status, ErrorBody{
			Code:    appErr.Code,
			Message: msg,
			Fields:  appErr.Fields,
		}
	}
	return http.StatusInternalServerError, ErrorBody{
		Code:    apperror.CodeInternalError,
		Message: "internal server error",
	}
}

func statusFromCode(code string) int {
	switch code {
	case apperror.CodeValidationFailed:
		return http.StatusUnprocessableEntity
	case apperror.CodeUnauthenticated:
		return http.StatusUnauthorized
	case apperror.CodeForbidden:
		return http.StatusForbidden
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apperror.CodeConflict, apperror.CodeVersionConflict:
		return http.StatusConflict
	case apperror.CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperror.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperror.CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WriteError encodes an error using the standard ErrorEnvelope, attaches requestId from context,
// and aborts the Gin middleware chain.
func WriteError(c *gin.Context, err error) {
	status, body := StatusFromError(err)
	if c != nil && c.Request != nil {
		body.RequestID = RequestIDFromContext(c.Request.Context())
	}
	c.AbortWithStatusJSON(status, ErrorEnvelope{Error: body})
}
