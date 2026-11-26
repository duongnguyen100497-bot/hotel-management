package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

type createServiceRequest struct {
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
	Currency    string          `json:"currency,omitempty"`
	Description string          `json:"description,omitempty"`
	IsActive    *bool           `json:"isActive,omitempty"`
}

type updateServiceRequest struct {
	Name        *string          `json:"name,omitempty"`
	Category    *string          `json:"category,omitempty"`
	UnitPrice   *decimal.Decimal `json:"unitPrice,omitempty"`
	Currency    *string          `json:"currency,omitempty"`
	Description *string          `json:"description,omitempty"`
	IsActive    *bool            `json:"isActive,omitempty"`
	Version     *int             `json:"version,omitempty"`
}

type deactivateServiceRequest struct {
	Version *int `json:"version,omitempty"`
}

func (h *Handler) handleListServices(c *gin.Context) {
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

	var isActive *bool
	if raw := strings.TrimSpace(c.Query("isActive")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			platformhttp.WriteError(c, apperror.Validation("invalid isActive query parameter", apperror.FieldError{
				Field:   "isActive",
				Message: "must be true or false",
			}))
			return
		}
		isActive = &b
	}

	page, err := h.svc.ListServices(c.Request.Context(), actor, propertyapp.ListServicesFilter{
		Category: strings.TrimSpace(c.Query("category")),
		IsActive: isActive,
		Cursor:   cursor,
		Limit:    limit,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, page)
}

func (h *Handler) handleCreateService(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req createServiceRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	created, err := h.svc.CreateService(c.Request.Context(), actor, propertyapp.CreateServiceParams{
		Code:        req.Code,
		Name:        req.Name,
		Category:    req.Category,
		UnitPrice:   req.UnitPrice,
		Currency:    req.Currency,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, created.Version))
	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": created})
}

func (h *Handler) handleUpdateService(c *gin.Context) {
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

	var req updateServiceRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	expectedVersion, err := resolveExpectedVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.UpdateService(c.Request.Context(), actor, propertyapp.UpdateServiceParams{
		ID:              id,
		Name:            req.Name,
		Category:        req.Category,
		UnitPrice:       req.UnitPrice,
		Currency:        req.Currency,
		Description:     req.Description,
		IsActive:        req.IsActive,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, updated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) handleDeactivateService(c *gin.Context) {
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
			var req deactivateServiceRequest
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

	deactivated, err := h.svc.DeactivateService(c.Request.Context(), actor, id, expectedVer)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%d"`, deactivated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": deactivated})
}
