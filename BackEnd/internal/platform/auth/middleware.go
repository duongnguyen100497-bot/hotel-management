package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

const (
	RefreshCookieName = "hotel_refresh_token"
	RefreshCookiePath = "/api/v1/auth"
)

// AccountState represents the live authorization state of a user in PostgreSQL.
type AccountState struct {
	UserID               uuid.UUID
	Kind                 string
	Status               string
	AuthorizationVersion int
	SessionActive        bool
}

// AccountStateVerifier looks up the authoritative user status, authorization_version, and session state.
type AccountStateVerifier interface {
	VerifyAccountState(ctx context.Context, userID, sessionID uuid.UUID) (AccountState, error)
}

// AuthenticatorMiddleware verifies the Bearer JWT access token, cross-checks the live account
// status ("active"), authorization_version, and session validity, and attaches Principal to ctx.
func AuthenticatorMiddleware(tokens *TokenManager, verifier AccountStateVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if rawHeader == "" {
			platformhttp.WriteError(c, apperror.Unauthenticated("missing authorization header"))
			return
		}
		parts := strings.SplitN(rawHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			platformhttp.WriteError(c, apperror.Unauthenticated("invalid authorization header format"))
			return
		}

		principal, _, err := tokens.VerifyAccessToken(parts[1])
		if err != nil {
			platformhttp.WriteError(c, apperror.Unauthenticated("invalid or expired access token"))
			return
		}

		if verifier != nil {
			state, err := verifier.VerifyAccountState(c.Request.Context(), principal.UserID, principal.SessionID)
			if err != nil {
				platformhttp.WriteError(c, apperror.Unauthenticated("invalid session or account state"))
				return
			}
			if state.Status != "active" || state.AuthorizationVersion != principal.AuthorizationVersion || !state.SessionActive {
				platformhttp.WriteError(c, apperror.Unauthenticated("session or authorization version revoked"))
				return
			}
			principal.Kind = state.Kind
		}

		ctx := WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// SetRefreshCookie writes the rotating refresh token into an HttpOnly, SameSite=Lax cookie.
func SetRefreshCookie(c *gin.Context, rawToken string, ttl time.Duration, secure bool) {
	maxAge := int(ttl.Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    rawToken,
		Path:     RefreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearRefreshCookie expires the refresh token cookie on logout or auth failure.
func ClearRefreshCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     RefreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadRefreshCookie extracts the refresh token from the HttpOnly cookie.
func ReadRefreshCookie(c *gin.Context) string {
	if c == nil {
		return ""
	}
	val, err := c.Cookie(RefreshCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}
