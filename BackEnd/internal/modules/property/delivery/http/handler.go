package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// HandlerConfig configures the property HTTP handler.
type HandlerConfig struct {
	Service    *propertyapp.Service
	Authorizer rbacapp.Authorizer
	Tokens     *auth.TokenManager
	Verifier   auth.AccountStateVerifier
}

// Handler serves hotel settings, room types, rooms, operational states, and service catalog endpoints.
type Handler struct {
	svc        *propertyapp.Service
	authorizer rbacapp.Authorizer
	tokens     *auth.TokenManager
	verifier   auth.AccountStateVerifier
}

// NewHandler constructs a property HTTP handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		svc:        cfg.Service,
		authorizer: cfg.Authorizer,
		tokens:     cfg.Tokens,
		verifier:   cfg.Verifier,
	}
}

// RegisterRoutes mounts public and authenticated property routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Public endpoints
	api.GET("/hotel/settings", h.handleGetHotelSettings)
	api.GET("/catalog/room-types", h.handleSearchPublicCatalog)
	api.GET("/catalog/room-types/:code", h.handleGetPublicRoomTypeByCode)

	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}
	secured.PUT("/hotel/settings", rbacapp.RequirePermission(h.authorizer, "hotel_settings.update"), h.handleUpdateHotelSettings)

	// Staff Room Types
	secured.GET("/room-types", rbacapp.RequirePermission(h.authorizer, "property.read"), h.handleListRoomTypes)
	secured.POST("/room-types", rbacapp.RequirePermission(h.authorizer, "room_type.manage"), h.handleCreateRoomType)
	secured.PATCH("/room-types/:id", rbacapp.RequirePermission(h.authorizer, "room_type.manage"), h.handleUpdateRoomType)

	// Staff Physical Rooms
	secured.GET("/rooms", rbacapp.RequirePermission(h.authorizer, "property.read"), h.handleListRooms)
	secured.POST("/rooms", rbacapp.RequirePermission(h.authorizer, "room.create"), h.handleCreateRoom)
	secured.PATCH("/rooms/:id", rbacapp.RequirePermission(h.authorizer, "room.update"), h.handleUpdateRoom)
	secured.POST("/rooms/:id/archive", rbacapp.RequirePermission(h.authorizer, "room.archive"), h.handleArchiveRoom)
	secured.POST("/rooms/:id/maintenance", rbacapp.RequirePermission(h.authorizer, "room.state.update"), h.handleRoomMaintenance)
	secured.POST("/rooms/:id/ready", rbacapp.RequirePermission(h.authorizer, "room.state.update"), h.handleRoomReady)
	secured.GET("/rooms/:id/state-history", rbacapp.RequirePermission(h.authorizer, "property.read"), h.handleListRoomStateHistory)

	// Ancillary Service Catalog
	secured.GET("/services", rbacapp.RequirePermission(h.authorizer, "service.read"), h.handleListServices)
	secured.POST("/services", rbacapp.RequirePermission(h.authorizer, "service.manage"), h.handleCreateService)
	secured.PATCH("/services/:id", rbacapp.RequirePermission(h.authorizer, "service.manage"), h.handleUpdateService)
	secured.POST("/services/:id/deactivate", rbacapp.RequirePermission(h.authorizer, "service.manage"), h.handleDeactivateService)
}

type updateHotelSettingsRequest struct {
	Name                     string     `json:"name"`
	Address                  string     `json:"address"`
	Phone                    string     `json:"phone"`
	Email                    string     `json:"email"`
	Timezone                 string     `json:"timezone"`
	Currency                 string     `json:"currency"`
	CheckInTime              string     `json:"checkInTime"`
	CheckOutTime             string     `json:"checkOutTime"`
	IndividualHoldTTLMinutes int        `json:"individualHoldTtlMinutes"`
	GroupHoldTTLMinutes      int        `json:"groupHoldTtlMinutes"`
	FreeCancelHours          int        `json:"freeCancelHours"`
	LogoMediaID              *uuid.UUID `json:"logoMediaId,omitempty"`
	Version                  *int       `json:"version,omitempty"`
}

func (h *Handler) handleGetHotelSettings(c *gin.Context) {
	ctx := c.Request.Context()
	rawAuth := strings.TrimSpace(c.GetHeader("Authorization"))
	if rawAuth == "" {
		pub, err := h.svc.GetPublicSettings(ctx)
		if err != nil {
			platformhttp.WriteError(c, err)
			return
		}
		platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": pub})
		return
	}

	principal, err := h.authenticateBearer(ctx, rawAuth)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if principal.Kind == "customer" {
		pub, err := h.svc.GetPublicSettings(ctx)
		if err != nil {
			platformhttp.WriteError(c, err)
			return
		}
		platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": pub})
		return
	}

	if h.authorizer != nil {
		if err := h.authorizer.Authorize(ctx, principal, "hotel_settings.read"); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
	}
	settings, err := h.svc.GetSettings(ctx)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, settings.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": settings})
}

func (h *Handler) handleUpdateHotelSettings(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req updateHotelSettingsRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	expectedVersion, err := resolveExpectedVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.UpdateSettings(c.Request.Context(), actor, propertyapp.UpdateSettingsParams{
		Name:                     req.Name,
		Address:                  req.Address,
		Phone:                    req.Phone,
		Email:                    req.Email,
		Timezone:                 req.Timezone,
		Currency:                 req.Currency,
		CheckInTime:              req.CheckInTime,
		CheckOutTime:             req.CheckOutTime,
		IndividualHoldTTLMinutes: req.IndividualHoldTTLMinutes,
		GroupHoldTTLMinutes:      req.GroupHoldTTLMinutes,
		FreeCancelHours:          req.FreeCancelHours,
		LogoMediaID:              req.LogoMediaID,
		ExpectedVersion:          expectedVersion,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) authenticateBearer(ctx context.Context, rawHeader string) (auth.Principal, error) {
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		return p, nil
	}
	parts := strings.SplitN(rawHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return auth.Principal{}, apperror.Unauthenticated("invalid authorization header format")
	}
	if h.tokens == nil {
		return auth.Principal{}, apperror.Unauthenticated("token verifier unavailable")
	}
	principal, _, err := h.tokens.VerifyAccessToken(parts[1])
	if err != nil {
		return auth.Principal{}, apperror.Unauthenticated("invalid or expired access token")
	}
	if h.verifier != nil {
		state, err := h.verifier.VerifyAccountState(ctx, principal.UserID, principal.SessionID)
		if err != nil {
			return auth.Principal{}, apperror.Unauthenticated("invalid session or account state")
		}
		if state.Status != "active" || state.AuthorizationVersion != principal.AuthorizationVersion || !state.SessionActive {
			return auth.Principal{}, apperror.Unauthenticated("session or authorization version revoked")
		}
		principal.Kind = state.Kind
	}
	return principal, nil
}

func resolveExpectedVersion(c *gin.Context, bodyVersion *int) (int, error) {
	ifMatch := platformhttp.IfMatch(c)
	var headerVer *int
	if ifMatch != "" {
		parsed, err := strconv.Atoi(ifMatch)
		if err != nil || parsed < 1 {
			return 0, apperror.Validation("invalid If-Match version header", apperror.FieldError{
				Field:   "If-Match",
				Message: "must be a positive integer version",
			})
		}
		headerVer = &parsed
	}

	if bodyVersion != nil && headerVer != nil && *bodyVersion != *headerVer {
		return 0, apperror.Validation("conflicting version in If-Match header and request body", apperror.FieldError{
			Field:   "version",
			Message: "must match If-Match header when both are supplied",
		})
	}
	if bodyVersion != nil {
		if *bodyVersion < 1 {
			return 0, apperror.Validation("invalid version", apperror.FieldError{
				Field:   "version",
				Message: "must be a positive integer",
			})
		}
		return *bodyVersion, nil
	}
	if headerVer != nil {
		return *headerVer, nil
	}
	return 0, apperror.Validation("version is required for optimistic concurrency control", apperror.FieldError{
		Field:   "version",
		Message: "provide version in request body or If-Match header",
	})
}
