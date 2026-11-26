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
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// Handler serves VAT rates management endpoints (/vat-rates).
type Handler struct {
	svc        *pricingapp.Service
	authorizer rbacapp.Authorizer
}

// NewHandler constructs a pricing HTTP handler.
func NewHandler(svc *pricingapp.Service, authorizer rbacapp.Authorizer) *Handler {
	return &Handler{
		svc:        svc,
		authorizer: authorizer,
	}
}

// RegisterRoutes mounts authenticated VAT rate routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}

	secured.GET("/vat-rates", requireAnyPermission(h.authorizer, "vat.manage", "property.read"), h.handleListVATRates)
	secured.POST("/vat-rates", rbacapp.RequirePermission(h.authorizer, "vat.manage"), h.handleCreateVATRate)
	secured.PATCH("/vat-rates/:id", rbacapp.RequirePermission(h.authorizer, "vat.manage"), h.handleUpdateVATRate)
	secured.POST("/vat-rates/:id/deactivate", rbacapp.RequirePermission(h.authorizer, "vat.manage"), h.handleDeactivateVATRate)
}

type createVATRateRequest struct {
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Rate     decimal.Decimal `json:"rate"`
	IsActive bool            `json:"isActive,omitempty"`
}

type updateVATRateRequest struct {
	Name     *string          `json:"name,omitempty"`
	Rate     *decimal.Decimal `json:"rate,omitempty"`
	IsActive *bool            `json:"isActive,omitempty"`
	Version  *int             `json:"version,omitempty"`
}

type deactivateVATRateRequest struct {
	Version *int `json:"version,omitempty"`
}

func (h *Handler) handleListVATRates(c *gin.Context) {
	rates, err := h.svc.ListVATRates(c.Request.Context())
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": rates})
}

func (h *Handler) handleCreateVATRate(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}

	var req createVATRateRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	created, err := h.svc.CreateVATRate(c.Request.Context(), actor, pricingapp.CreateVATRateParams{
		Code:     req.Code,
		Name:     req.Name,
		Rate:     req.Rate,
		IsActive: req.IsActive,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	c.Header("ETag", fmt.Sprintf(`"%d"`, created.Version))
	platformhttp.WriteJSON(c, http.StatusCreated, gin.H{"data": created})
}

func (h *Handler) handleUpdateVATRate(c *gin.Context) {
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

	var req updateVATRateRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	expectedVersion, err := resolveExpectedVersion(c, req.Version)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	updated, err := h.svc.UpdateVATRate(c.Request.Context(), actor, pricingapp.UpdateVATRateParams{
		ID:              id,
		Name:            req.Name,
		Rate:            req.Rate,
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

func (h *Handler) handleDeactivateVATRate(c *gin.Context) {
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
			var req deactivateVATRateRequest
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

	deactivated, err := h.svc.DeactivateVATRate(c.Request.Context(), actor, id, expectedVer)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}

	c.Header("ETag", fmt.Sprintf(`"%d"`, deactivated.Version))
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": deactivated})
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

func resolveExpectedVersion(c *gin.Context, bodyVersion *int) (int, error) {
	ver, err := resolveOptionalVersion(c, bodyVersion)
	if err != nil {
		return 0, err
	}
	if ver == nil {
		return 0, apperror.Validation("version is required for optimistic concurrency control", apperror.FieldError{
			Field:   "version",
			Message: "provide version in request body or If-Match header",
		})
	}
	return *ver, nil
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

func requireAnyPermission(authorizer rbacapp.Authorizer, perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := auth.PrincipalFromContext(c.Request.Context())
		if !ok {
			platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
			return
		}
		if authorizer == nil {
			platformhttp.WriteError(c, apperror.Internal(nil))
			return
		}
		var lastErr error
		for _, p := range perms {
			if err := authorizer.Authorize(c.Request.Context(), principal, p); err == nil {
				c.Next()
				return
			} else {
				lastErr = err
			}
		}
		if lastErr == nil {
			lastErr = apperror.Forbidden("permission denied")
		}
		platformhttp.WriteError(c, lastErr)
	}
}
