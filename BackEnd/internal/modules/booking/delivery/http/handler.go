package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// Handler serves booking availability, hold, individual/group booking, and cancellation endpoints.
type Handler struct {
	svc        *bookingapp.Service
	authorizer rbacapp.Authorizer
}

// NewHandler constructs a booking HTTP handler.
func NewHandler(svc *bookingapp.Service, authorizer rbacapp.Authorizer) *Handler {
	return &Handler{
		svc:        svc,
		authorizer: authorizer,
	}
}

// RegisterRoutes mounts booking routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Public availability & quote
	api.POST("/availability/quote", h.handleAvailabilityQuote)

	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}
	secured.POST("/holds", h.handleCreateHold)
	secured.DELETE("/holds/:id", h.handleReleaseHold)
}

type availabilityQuoteRequest struct {
	CheckIn  string                        `json:"checkIn"`
	CheckOut string                        `json:"checkOut"`
	Items    []bookingapp.QuoteItemRequest `json:"items"`
	Adults   int                           `json:"adults"`
	Children int                           `json:"children"`
}

func (h *Handler) handleAvailabilityQuote(c *gin.Context) {
	var req availabilityQuoteRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	quote, err := h.svc.QuoteAvailability(c.Request.Context(), bookingapp.QuoteAvailabilityParams{
		CheckIn:  req.CheckIn,
		CheckOut: req.CheckOut,
		Items:    req.Items,
		Adults:   req.Adults,
		Children: req.Children,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": quote})
}

type createHoldRequest struct {
	CustomerID *uuid.UUID                    `json:"customerId,omitempty"`
	CheckIn    string                        `json:"checkIn"`
	CheckOut   string                        `json:"checkOut"`
	Items      []bookingapp.QuoteItemRequest `json:"items"`
	Adults     int                           `json:"adults"`
	Children   int                           `json:"children"`
}

func (h *Handler) handleCreateHold(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createHoldRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	res, err := h.svc.CreateHold(c.Request.Context(), principal, bookingapp.CreateHoldParams{
		CustomerID: req.CustomerID,
		CheckIn:    req.CheckIn,
		CheckOut:   req.CheckOut,
		Items:      req.Items,
		Adults:     req.Adults,
		Children:   req.Children,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": res})
}

type releaseHoldRequest struct {
	HoldToken string `json:"holdToken"`
}

func (h *Handler) handleReleaseHold(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	holdID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttp.WriteError(c, apperror.Validation("invalid hold id", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		}))
		return
	}

	token := strings.TrimSpace(c.GetHeader("X-Hold-Token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("holdToken"))
	}
	if token == "" && c.Request.Body != nil && c.Request.ContentLength != 0 {
		var body releaseHoldRequest
		if err := platformhttp.DecodeJSON(c, &body); err != nil && err != io.EOF {
			platformhttp.WriteError(c, err)
			return
		}
		token = strings.TrimSpace(body.HoldToken)
	}

	if err := h.svc.ReleaseHold(c.Request.Context(), principal, bookingapp.ReleaseHoldParams{
		ID:        holdID,
		HoldToken: token,
	}); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": gin.H{"released": true}})
}
