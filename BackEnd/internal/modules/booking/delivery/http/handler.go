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

	// Individual bookings
	secured.POST("/bookings", h.handleCreateBooking)
	secured.GET("/bookings", rbacapp.RequirePermission(h.authorizer, "booking.read"), h.handleListBookings)
	secured.GET("/me/bookings", h.handleListMyBookings)
	secured.GET("/bookings/:id", h.handleGetBooking)
	secured.POST("/bookings/:id/confirm", rbacapp.RequirePermission(h.authorizer, "booking.confirm"), h.handleConfirmBooking)

	// Group bookings
	secured.POST("/booking-groups", h.handleCreateBookingGroup)
	secured.GET("/booking-groups/:id", h.handleGetBookingGroup)
	secured.GET("/me/booking-groups", h.handleListMyBookingGroups)
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

type createBookingRequest struct {
	CustomerID          *uuid.UUID               `json:"customerId,omitempty"`
	HoldID              *uuid.UUID               `json:"holdId,omitempty"`
	HoldToken           string                   `json:"holdToken,omitempty"`
	RoomTypeID          *uuid.UUID               `json:"roomTypeId,omitempty"`
	RoomID              *uuid.UUID               `json:"roomId,omitempty"`
	CheckIn             string                   `json:"checkIn,omitempty"`
	CheckOut            string                   `json:"checkOut,omitempty"`
	Adults              int                      `json:"adults"`
	Children            int                      `json:"children"`
	SpecialRequest      string                   `json:"specialRequest,omitempty"`
	RepresentativeGuest bookingapp.GuestSnapshot `json:"representativeGuest"`
}

func (h *Handler) handleCreateBooking(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createBookingRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	b, replayed, err := h.svc.CreateBooking(c.Request.Context(), principal, bookingapp.CreateBookingParams{
		IdempotencyKey:      platformhttp.IdempotencyKey(c),
		CustomerID:          req.CustomerID,
		HoldID:              req.HoldID,
		HoldToken:           req.HoldToken,
		RoomTypeID:          req.RoomTypeID,
		RoomID:              req.RoomID,
		CheckIn:             req.CheckIn,
		CheckOut:            req.CheckOut,
		Adults:              req.Adults,
		Children:            req.Children,
		SpecialRequest:      req.SpecialRequest,
		RepresentativeGuest: req.RepresentativeGuest,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	platformhttp.WriteJSON(c, status, gin.H{"data": b})
}

func (h *Handler) handleGetBooking(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttp.WriteError(c, apperror.Validation("invalid booking id", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		}))
		return
	}
	b, err := h.svc.GetBooking(c.Request.Context(), principal, bookingID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": b})
}

func (h *Handler) handleListBookings(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var custID, groupID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("customerId")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid customerId", apperror.FieldError{Field: "customerId", Message: "must be a valid UUID"}))
			return
		}
		custID = &parsed
	}
	if raw := strings.TrimSpace(c.Query("bookingGroupId")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid bookingGroupId", apperror.FieldError{Field: "bookingGroupId", Message: "must be a valid UUID"}))
			return
		}
		groupID = &parsed
	}
	res, err := h.svc.ListBookings(c.Request.Context(), principal, bookingapp.ListBookingsFilter{
		CustomerID:     custID,
		BookingGroupID: groupID,
		Status:         strings.TrimSpace(c.Query("status")),
		Cursor:         cursor,
		Limit:          limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, res)
}

func (h *Handler) handleListMyBookings(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	res, err := h.svc.ListMyBookings(c.Request.Context(), principal, bookingapp.ListBookingsFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, res)
}

type confirmBookingRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *Handler) handleConfirmBooking(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttp.WriteError(c, apperror.Validation("invalid booking id", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		}))
		return
	}
	var req confirmBookingRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := platformhttp.DecodeJSON(c, &req); err != nil && err != io.EOF {
			platformhttp.WriteError(c, err)
			return
		}
	}
	b, err := h.svc.ConfirmBooking(c.Request.Context(), principal, bookingID, req.Reason)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": b})
}

type createBookingGroupRequest struct {
	CustomerID          *uuid.UUID                  `json:"customerId,omitempty"`
	GroupHoldID         uuid.UUID                   `json:"groupHoldId"`
	HoldToken           string                      `json:"holdToken"`
	Adults              int                         `json:"adults,omitempty"`
	Children            int                         `json:"children,omitempty"`
	SpecialRequest      string                      `json:"specialRequest,omitempty"`
	RepresentativeGuest *bookingapp.GuestSnapshot   `json:"representativeGuest,omitempty"`
	RoomGuests          []bookingapp.RoomGuestInput `json:"roomGuests,omitempty"`
}

func (h *Handler) handleCreateBookingGroup(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createBookingGroupRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	grp, replayed, err := h.svc.CreateBookingGroup(c.Request.Context(), principal, bookingapp.CreateBookingGroupParams{
		IdempotencyKey:      platformhttp.IdempotencyKey(c),
		CustomerID:          req.CustomerID,
		GroupHoldID:         req.GroupHoldID,
		HoldToken:           req.HoldToken,
		Adults:              req.Adults,
		Children:            req.Children,
		SpecialRequest:      req.SpecialRequest,
		RepresentativeGuest: req.RepresentativeGuest,
		RoomGuests:          req.RoomGuests,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	platformhttp.WriteJSON(c, status, gin.H{"data": grp})
}

func (h *Handler) handleGetBookingGroup(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		platformhttp.WriteError(c, apperror.Validation("invalid booking group id", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		}))
		return
	}
	grp, err := h.svc.GetBookingGroup(c.Request.Context(), principal, groupID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": grp})
}

func (h *Handler) handleListMyBookingGroups(c *gin.Context) {
	principal, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	res, err := h.svc.ListMyBookingGroups(c.Request.Context(), principal, cursor, limit)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, res)
}
