package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

type createRoomTypeRequest struct {
	Code            string           `json:"code"`
	Name            string           `json:"name"`
	Description     string           `json:"description,omitempty"`
	BaseNightlyRate decimal.Decimal  `json:"baseNightlyRate"`
	Currency        string           `json:"currency,omitempty"`
	Capacity        int              `json:"capacity"`
	AreaSqm         *decimal.Decimal `json:"areaSqm,omitempty"`
	BedType         string           `json:"bedType,omitempty"`
	Amenities       []string         `json:"amenities,omitempty"`
	IsBookable      *bool            `json:"isBookable,omitempty"`
}

type updateRoomTypeRequest struct {
	Name            *string          `json:"name,omitempty"`
	Description     *string          `json:"description,omitempty"`
	BaseNightlyRate *decimal.Decimal `json:"baseNightlyRate,omitempty"`
	Currency        *string          `json:"currency,omitempty"`
	Capacity        *int             `json:"capacity,omitempty"`
	AreaSqm         *decimal.Decimal `json:"areaSqm,omitempty"`
	ClearAreaSqm    bool             `json:"clearAreaSqm,omitempty"`
	BedType         *string          `json:"bedType,omitempty"`
	Amenities       *[]string        `json:"amenities,omitempty"`
	IsBookable      *bool            `json:"isBookable,omitempty"`
	Archived        *bool            `json:"archived,omitempty"`
	Version         *int             `json:"version,omitempty"`
}

type createRoomRequest struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	RoomTypeID  uuid.UUID `json:"roomTypeId"`
	Floor       int       `json:"floor"`
	Capacity    *int      `json:"capacity,omitempty"`
	Description string    `json:"description,omitempty"`
}

// updateRoomRequest intentionally omits operationalStatus:
// PATCH /api/v1/rooms/:id must NEVER mutate operational_status directly.
type updateRoomRequest struct {
	Code        *string    `json:"code,omitempty"`
	Name        *string    `json:"name,omitempty"`
	RoomTypeID  *uuid.UUID `json:"roomTypeId,omitempty"`
	Floor       *int       `json:"floor,omitempty"`
	Capacity    *int       `json:"capacity,omitempty"`
	Description *string    `json:"description,omitempty"`
	Version     *int       `json:"version,omitempty"`
}

type archiveRoomRequest struct {
	Version *int `json:"version,omitempty"`
}

func (h *Handler) handleSearchPublicCatalog(c *gin.Context) {
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var capacity *int
	if rawCap := strings.TrimSpace(c.Query("capacity")); rawCap != "" {
		n, err := strconv.Atoi(rawCap)
		if err != nil || n <= 0 {
			platformhttp.WriteError(c, apperror.Validation("invalid capacity query parameter", apperror.FieldError{
				Field:   "capacity",
				Message: "must be a positive integer",
			}))
			return
		}
		capacity = &n
	}

	var minPrice *decimal.Decimal
	if rawMin := strings.TrimSpace(c.Query("minPrice")); rawMin != "" {
		d, err := decimal.NewFromString(rawMin)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid minPrice query parameter", apperror.FieldError{
				Field:   "minPrice",
				Message: "must be a valid decimal amount",
			}))
			return
		}
		minPrice = &d
	}

	var maxPrice *decimal.Decimal
	if rawMax := strings.TrimSpace(c.Query("maxPrice")); rawMax != "" {
		d, err := decimal.NewFromString(rawMax)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid maxPrice query parameter", apperror.FieldError{
				Field:   "maxPrice",
				Message: "must be a valid decimal amount",
			}))
			return
		}
		maxPrice = &d
	}

	page, err := h.svc.SearchPublicCatalog(c.Request.Context(), propertyapp.PublicCatalogFilter{
		Capacity: capacity,
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		CheckIn:  strings.TrimSpace(c.Query("checkIn")),
		CheckOut: strings.TrimSpace(c.Query("checkOut")),
		Sort:     strings.TrimSpace(c.Query("sort")),
		Cursor:   cursor,
		Limit:    limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, page)
}

func (h *Handler) handleGetPublicRoomTypeByCode(c *gin.Context) {
	rt, err := h.svc.GetPublicRoomTypeByCode(c.Request.Context(), c.Param("code"))
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": rt})
}

func (h *Handler) handleListRoomTypes(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	includeArchived := false
	if raw := strings.TrimSpace(c.Query("includeArchived")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid includeArchived query parameter", apperror.FieldError{
				Field:   "includeArchived",
				Message: "must be true or false",
			}))
			return
		}
		includeArchived = b
	}

	var isBookable *bool
	if raw := strings.TrimSpace(c.Query("isBookable")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid isBookable query parameter", apperror.FieldError{
				Field:   "isBookable",
				Message: "must be true or false",
			}))
			return
		}
		isBookable = &b
	}

	page, err := h.svc.ListRoomTypes(c.Request.Context(), actor, propertyapp.StaffRoomTypeFilter{
		IncludeArchived: includeArchived,
		IsBookable:      isBookable,
		Cursor:          cursor,
		Limit:           limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, page)
}

func (h *Handler) handleCreateRoomType(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req createRoomTypeRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	created, err := h.svc.CreateRoomType(c.Request.Context(), actor, propertyapp.CreateRoomTypeParams{
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		BaseNightlyRate: req.BaseNightlyRate,
		Currency:        req.Currency,
		Capacity:        req.Capacity,
		AreaSqm:         req.AreaSqm,
		BedType:         req.BedType,
		Amenities:       req.Amenities,
		IsBookable:      req.IsBookable,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, created.Version))
	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": created})
}

func (h *Handler) handleUpdateRoomType(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req updateRoomTypeRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	expectedVersion, err := resolveExpectedVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.UpdateRoomType(c.Request.Context(), actor, propertyapp.UpdateRoomTypeParams{
		ID:              id,
		Name:            req.Name,
		Description:     req.Description,
		BaseNightlyRate: req.BaseNightlyRate,
		Currency:        req.Currency,
		Capacity:        req.Capacity,
		AreaSqm:         req.AreaSqm,
		ClearAreaSqm:    req.ClearAreaSqm,
		BedType:         req.BedType,
		Amenities:       req.Amenities,
		IsBookable:      req.IsBookable,
		Archived:        req.Archived,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) handleListRooms(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, cursor, err := platformhttp.ParseCursorParams(c, 20, 100)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var roomTypeID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("roomTypeId")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil || parsed == uuid.Nil {
			platformhttp.WriteError(c, apperror.Validation("invalid roomTypeId query parameter", apperror.FieldError{
				Field:   "roomTypeId",
				Message: "must be a valid UUID",
			}))
			return
		}
		roomTypeID = &parsed
	}

	var floor *int
	if raw := strings.TrimSpace(c.Query("floor")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid floor query parameter", apperror.FieldError{
				Field:   "floor",
				Message: "must be an integer",
			}))
			return
		}
		floor = &n
	}

	includeArchived := false
	if raw := strings.TrimSpace(c.Query("includeArchived")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid includeArchived query parameter", apperror.FieldError{
				Field:   "includeArchived",
				Message: "must be true or false",
			}))
			return
		}
		includeArchived = b
	}

	page, err := h.svc.ListRooms(c.Request.Context(), actor, propertyapp.StaffRoomFilter{
		RoomTypeID:        roomTypeID,
		OperationalStatus: strings.TrimSpace(c.Query("operationalStatus")),
		Floor:             floor,
		IncludeArchived:   includeArchived,
		Cursor:            cursor,
		Limit:             limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, page)
}

func (h *Handler) handleCreateRoom(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req createRoomRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	created, err := h.svc.CreateRoom(c.Request.Context(), actor, propertyapp.CreateRoomParams{
		Code:        req.Code,
		Name:        req.Name,
		RoomTypeID:  req.RoomTypeID,
		Floor:       req.Floor,
		Capacity:    req.Capacity,
		Description: req.Description,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, created.Version))
	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": created})
}

func (h *Handler) handleUpdateRoom(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var req updateRoomRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	expectedVersion, err := resolveExpectedVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.UpdateRoom(c.Request.Context(), actor, propertyapp.UpdateRoomParams{
		ID:              id,
		Code:            req.Code,
		Name:            req.Name,
		RoomTypeID:      req.RoomTypeID,
		Floor:           req.Floor,
		Capacity:        req.Capacity,
		Description:     req.Description,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) handleArchiveRoom(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	var bodyVer *int
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid request body"))
			return
		}
		if strings.TrimSpace(string(raw)) != "" {
			c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))
			var req archiveRoomRequest
			if err := platformhttp.DecodeJSON(c, &req); err != nil {
				platformhttp.WriteError(c, err)
				return
			}
			bodyVer = req.Version
		}
	}

	expectedVer, err := resolveOptionalVersion(c, bodyVer)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	archived, err := h.svc.ArchiveRoom(c.Request.Context(), actor, id, expectedVer)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, archived.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": archived})
}

func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, apperror.Validation("invalid path parameter", apperror.FieldError{
			Field:   name,
			Message: "must be a valid UUID",
		})
	}
	return parsed, nil
}

func resolveOptionalVersion(c *gin.Context, bodyVersion *int) (*int, error) {
	ifMatch := platformhttp.IfMatch(c)
	var headerVer *int
	if ifMatch != "" {
		parsed, err := strconv.Atoi(ifMatch)
		if err != nil || parsed < 1 {
			return nil, apperror.Validation("invalid If-Match version header", apperror.FieldError{
				Field:   "If-Match",
				Message: "must be a positive integer version",
			})
		}
		headerVer = &parsed
	}
	if bodyVersion != nil && headerVer != nil && *bodyVersion != *headerVer {
		return nil, apperror.Validation("conflicting version in If-Match header and request body", apperror.FieldError{
			Field:   "version",
			Message: "must match If-Match header when both are supplied",
		})
	}
	if bodyVersion != nil {
		if *bodyVersion < 1 {
			return nil, apperror.Validation("invalid version", apperror.FieldError{
				Field:   "version",
				Message: "must be a positive integer",
			})
		}
		return bodyVersion, nil
	}
	return headerVer, nil
}
