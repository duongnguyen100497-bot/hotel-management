package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	peopleapp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// Handler serves /api/v1/admin/employees* (and /api/v1/customers* in T1.6).
type Handler struct {
	svc  *peopleapp.Service
	rbac rbacapp.Authorizer
}

// NewHandler constructs a People HTTP handler.
func NewHandler(svc *peopleapp.Service, rbac rbacapp.Authorizer) *Handler {
	return &Handler{
		svc:  svc,
		rbac: rbac,
	}
}

// RegisterRoutes mounts authenticated employee and customer routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}

	empGroup := secured.Group("/admin/employees")
	empGroup.GET("", rbacapp.RequirePermission(h.rbac, "employee.read"), h.handleListEmployees)
	empGroup.POST("", rbacapp.RequirePermission(h.rbac, "employee.create"), h.handleCreateEmployee)
	empGroup.GET("/:id", rbacapp.RequirePermission(h.rbac, "employee.read"), h.handleGetEmployee)
	empGroup.PATCH("/:id", rbacapp.RequirePermission(h.rbac, "employee.update"), h.handleUpdateEmployee)
	empGroup.POST("/:id/resend-activation", rbacapp.RequirePermission(h.rbac, "employee.update"), h.handleResendActivation)
	empGroup.POST("/:id/deactivate", rbacapp.RequirePermission(h.rbac, "employee.deactivate"), h.handleDeactivateEmployee)
}

type createEmployeeRequest struct {
	EmployeeCode  string      `json:"employeeCode"`
	FullName      string      `json:"fullName"`
	Phone         string      `json:"phone"`
	Email         string      `json:"email"`
	PositionTitle string      `json:"positionTitle"`
	Username      string      `json:"username,omitempty"`
	RoleCodes     []string    `json:"roleCodes,omitempty"`
	RoleIDs       []uuid.UUID `json:"roleIds,omitempty"`
}

type updateEmployeeRequest struct {
	FullName      string `json:"fullName,omitempty"`
	Phone         string `json:"phone,omitempty"`
	PositionTitle string `json:"positionTitle,omitempty"`
	Version       int    `json:"version,omitempty"`
}

type deactivateEmployeeRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *Handler) handleCreateEmployee(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createEmployeeRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	emp, err := h.svc.CreateEmployee(c.Request.Context(), actor, peopleapp.CreateEmployeeParams{
		EmployeeCode:  req.EmployeeCode,
		FullName:      req.FullName,
		Phone:         req.Phone,
		Email:         req.Email,
		PositionTitle: req.PositionTitle,
		Username:      req.Username,
		RoleCodes:     req.RoleCodes,
		RoleIDs:       req.RoleIDs,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusCreated, emp)
}

func (h *Handler) handleListEmployees(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	limit, _, err := platformhttp.ParseCursorParams(c, 50, 200)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	emps, err := h.svc.ListEmployees(c.Request.Context(), actor, limit)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": emps})
}

func (h *Handler) handleGetEmployee(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	empID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	emp, err := h.svc.GetEmployee(c.Request.Context(), actor, empID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, emp)
}

func (h *Handler) handleUpdateEmployee(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	empID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req updateEmployeeRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	version := req.Version
	if version == 0 {
		if ifMatch := platformhttp.IfMatch(c); ifMatch != "" {
			if parsed, err := strconv.Atoi(ifMatch); err == nil {
				version = parsed
			}
		}
	}
	emp, err := h.svc.UpdateEmployee(c.Request.Context(), actor, peopleapp.UpdateEmployeeParams{
		ID:              empID,
		FullName:        req.FullName,
		Phone:           req.Phone,
		PositionTitle:   req.PositionTitle,
		ExpectedVersion: version,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, emp)
}

func (h *Handler) handleResendActivation(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	empID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if err := h.svc.ResendEmployeeActivation(c.Request.Context(), actor, empID); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusAccepted, gin.H{"status": "activation_email_queued"})
}

func (h *Handler) handleDeactivateEmployee(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	empID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req deactivateEmployeeRequest
	if c.Request.ContentLength > 0 {
		if err := platformhttp.DecodeJSON(c, &req); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
	}
	emp, err := h.svc.DeactivateEmployee(c.Request.Context(), actor, empID, strings.TrimSpace(req.Reason))
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, emp)
}

func parseUUIDParam(c *gin.Context, param string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(c.Param(param))
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, apperror.Validation("invalid UUID path parameter", apperror.FieldError{
			Field:   param,
			Message: "must be a valid UUID",
		})
	}
	return parsed, nil
}
