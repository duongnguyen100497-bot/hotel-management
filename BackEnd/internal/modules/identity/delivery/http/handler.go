package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// HandlerConfig configures HTTP transport behavior for identity auth routes.
type HandlerConfig struct {
	Service      *identityapp.Service
	Tokens       *auth.TokenManager
	SecureCookie bool
	LoginLimiter platformhttp.RateLimiter
}

// Handler serves /api/v1/auth/* and /api/v1/me/{password,sessions} endpoints.
type Handler struct {
	svc          *identityapp.Service
	tokens       *auth.TokenManager
	secureCookie bool
	loginLimiter platformhttp.RateLimiter
}

// NewHandler constructs an identity HTTP handler.
func NewHandler(cfg HandlerConfig) *Handler {
	limiter := cfg.LoginLimiter
	if limiter == nil {
		limiter = platformhttp.NewInMemoryRateLimiter(20, time.Minute)
	}
	return &Handler{
		svc:          cfg.Service,
		tokens:       cfg.Tokens,
		secureCookie: cfg.SecureCookie,
		loginLimiter: limiter,
	}
}

// RegisterRoutes mounts public and authenticated auth endpoints on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	authGroup := api.Group("/auth")
	authGroup.POST("/register", h.handleRegister)
	authGroup.POST("/login", platformhttp.RateLimitMiddleware(h.loginLimiter, loginRateLimitKey), h.handleLogin)
	authGroup.POST("/refresh", h.handleRefresh)
	authGroup.POST("/activate", h.handleActivate)
	authGroup.POST("/password-reset-requests", platformhttp.RateLimitMiddleware(h.loginLimiter, loginRateLimitKey), h.handlePasswordResetRequest)
	authGroup.POST("/password-reset-confirm", h.handlePasswordResetConfirm)

	meAuth := authMiddleware
	if meAuth == nil {
		meAuth = auth.AuthenticatorMiddleware(h.tokens, h.svc)
	}
	authGroup.POST("/logout", meAuth, h.handleLogout)

	meGroup := api.Group("/me", meAuth)
	meGroup.POST("/password", h.handleChangeOwnPassword)
	meGroup.GET("/sessions", h.handleListOwnSessions)
	meGroup.DELETE("/sessions/:id", h.handleRevokeOwnSession)
}

func loginRateLimitKey(c *gin.Context) string {
	ip := c.ClientIP()
	if c.Request == nil || c.Request.Body == nil {
		return "login:" + ip
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 4096))
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return "login:" + ip
	}
	var peek struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &peek)
	email := strings.ToLower(strings.TrimSpace(peek.Email))
	if email == "" {
		return "login:" + ip
	}
	return "login:" + ip + ":" + email
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Phone       string `json:"phone,omitempty"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type logoutRequest struct {
	All bool `json:"all,omitempty"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type activateAccountRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type sessionView struct {
	ID        string `json:"id"`
	FamilyID  string `json:"familyId"`
	IssuedAt  string `json:"issuedAt"`
	ExpiresAt string `json:"expiresAt"`
	Current   bool   `json:"current"`
}

type authResponse struct {
	AccessToken string           `json:"accessToken"`
	ExpiresIn   int              `json:"expiresIn"`
	User        identityapp.User `json:"user"`
}

func (h *Handler) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	res, err := h.svc.RegisterCustomer(c.Request.Context(), identityapp.RegisterCustomerParams{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Phone:       req.Phone,
		ClientIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	auth.SetRefreshCookie(c, res.RefreshToken, h.tokens.RefreshTTL(), h.secureCookie)
	platformhttp.WriteJSON(c, http.StatusCreated, authResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		User:        res.User,
	})
}

func (h *Handler) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	res, err := h.svc.Login(c.Request.Context(), identityapp.LoginParams{
		Email:     req.Email,
		Password:  req.Password,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	auth.SetRefreshCookie(c, res.RefreshToken, h.tokens.RefreshTTL(), h.secureCookie)
	platformhttp.WriteJSON(c, http.StatusOK, authResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		User:        res.User,
	})
}

func (h *Handler) handleRefresh(c *gin.Context) {
	rawToken := auth.ReadRefreshCookie(c)
	res, err := h.svc.RefreshSession(c.Request.Context(), identityapp.RefreshParams{
		RawRefreshToken: rawToken,
		ClientIP:        c.ClientIP(),
		UserAgent:       c.Request.UserAgent(),
	})
	if err != nil {
		auth.ClearRefreshCookie(c, h.secureCookie)
		platformhttp.WriteError(c, err)
		return
	}

	auth.SetRefreshCookie(c, res.RefreshToken, h.tokens.RefreshTTL(), h.secureCookie)
	platformhttp.WriteJSON(c, http.StatusOK, authResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		User:        res.User,
	})
}

func (h *Handler) handleLogout(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	all := strings.EqualFold(strings.TrimSpace(c.Query("all")), "true")
	if c.Request.ContentLength > 0 {
		var req logoutRequest
		if err := platformhttp.DecodeJSON(c, &req); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
		if req.All {
			all = true
		}
	}

	if err := h.svc.Logout(c.Request.Context(), identityapp.LogoutParams{
		UserID:    principal.UserID,
		SessionID: principal.SessionID,
		All:       all,
	}); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	auth.ClearRefreshCookie(c, h.secureCookie)
	c.Status(http.StatusNoContent)
}

func (h *Handler) handlePasswordResetRequest(c *gin.Context) {
	var req passwordResetRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	if err := h.svc.RequestPasswordReset(c.Request.Context(), identityapp.PasswordResetRequestParams{
		Email:    req.Email,
		ClientIP: c.ClientIP(),
	}); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	platformhttp.WriteJSON(c, http.StatusAccepted, gin.H{
		"status":  "accepted",
		"message": "If an active account matches that email, a password reset link will be sent shortly.",
	})
}

func (h *Handler) handlePasswordResetConfirm(c *gin.Context) {
	var req passwordResetConfirmRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	if err := h.svc.ConfirmPasswordReset(c.Request.Context(), identityapp.PasswordResetConfirmParams{
		RawToken:    req.Token,
		NewPassword: req.Password,
		ClientIP:    c.ClientIP(),
	}); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	auth.ClearRefreshCookie(c, h.secureCookie)
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{
		"status": "password_reset_confirmed",
	})
}

func (h *Handler) handleActivate(c *gin.Context) {
	var req activateAccountRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	res, err := h.svc.ActivateAccount(c.Request.Context(), identityapp.ActivateAccountParams{
		RawToken:  req.Token,
		Password:  req.Password,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	auth.SetRefreshCookie(c, res.RefreshToken, h.tokens.RefreshTTL(), h.secureCookie)
	platformhttp.WriteJSON(c, http.StatusOK, authResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		User:        res.User,
	})
}

func (h *Handler) handleChangeOwnPassword(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req changePasswordRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if err := h.svc.ChangeOwnPassword(c.Request.Context(), identityapp.ChangePasswordParams{
		UserID:          principal.UserID,
		CurrentSession:  principal.SessionID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
		ClientIP:        c.ClientIP(),
	}); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"status": "password_changed"})
}

func (h *Handler) handleListOwnSessions(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	sessions, err := h.svc.ListOwnSessions(c.Request.Context(), principal.UserID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	views := make([]sessionView, 0, len(sessions))
	for _, s := range sessions {
		views = append(views, sessionView{
			ID:        s.ID.String(),
			FamilyID:  s.FamilyID.String(),
			IssuedAt:  s.IssuedAt.UTC().Format(time.RFC3339),
			ExpiresAt: s.ExpiresAt.UTC().Format(time.RFC3339),
			Current:   s.ID == principal.SessionID,
		})
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": views})
}

func (h *Handler) handleRevokeOwnSession(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil || sessionID == uuid.Nil {
		platformhttp.WriteError(c, apperror.Validation("invalid session ID", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		}))
		return
	}
	if err := h.svc.RevokeOwnSession(c.Request.Context(), principal.UserID, sessionID); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
