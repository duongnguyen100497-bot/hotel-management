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

	custGroup := secured.Group("/customers")
	custGroup.GET("", rbacapp.RequirePermission(h.rbac, "customer.read"), h.handleListCustomers)
	custGroup.POST("", rbacapp.RequirePermission(h.rbac, "customer.create"), h.handleCreateCustomer)
	custGroup.GET("/:id", rbacapp.RequirePermission(h.rbac, "customer.read"), h.handleGetCustomer)
	custGroup.PATCH("/:id", rbacapp.RequirePermission(h.rbac, "customer.update"), h.handleUpdateCustomer)
	custGroup.POST("/:id/archive", rbacapp.RequirePermission(h.rbac, "customer.archive"), h.handleArchiveCustomer)

	secured.GET("/me", h.handleGetMe)
	secured.PATCH("/me", h.handleUpdateMe)
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

type createCustomerRequest struct {
	FullName        string `json:"fullName"`
	Phone           string `json:"phone"`
	Email           string `json:"email,omitempty"`
	NationalityCode string `json:"nationalityCode,omitempty"`
	GovernmentID    string `json:"governmentId,omitempty"`
	Address         string `json:"address,omitempty"`
}

type updateCustomerRequest struct {
	FullName        *string `json:"fullName,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	Email           *string `json:"email,omitempty"`
	NationalityCode *string `json:"nationalityCode,omitempty"`
	GovernmentID    *string `json:"governmentId,omitempty"`
	Address         *string `json:"address,omitempty"`
	Version         int     `json:"version,omitempty"`
}

type archiveCustomerRequest struct {
	Reason string `json:"reason,omitempty"`
}

type updateMeRequest struct {
	DisplayName     *string `json:"displayName,omitempty"`
	FullName        *string `json:"fullName,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	NationalityCode *string `json:"nationalityCode,omitempty"`
	GovernmentID    *string `json:"governmentId,omitempty"`
	Address         *string `json:"address,omitempty"`
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

func (h *Handler) handleCreateCustomer(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createCustomerRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	cust, err := h.svc.CreateWalkInCustomer(c.Request.Context(), actor, peopleapp.CreateWalkInCustomerParams{
		FullName:        req.FullName,
		Phone:           req.Phone,
		Email:           req.Email,
		NationalityCode: req.NationalityCode,
		GovernmentID:    req.GovernmentID,
		Address:         req.Address,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusCreated, cust)
}

func (h *Handler) handleListCustomers(c *gin.Context) {
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
	search := strings.TrimSpace(c.Query("q"))
	if search == "" {
		search = strings.TrimSpace(c.Query("search"))
	}
	items, nextCursor, err := h.svc.ListCustomers(c.Request.Context(), actor, search, cursor, limit)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var curPtr *string
	if cursor != "" {
		curPtr = &cursor
	}
	platformhttp.WriteJSON(c, http.StatusOK, platformhttp.PageResponse[peopleapp.Customer]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	})
}

func (h *Handler) handleGetCustomer(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	custID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	cust, err := h.svc.GetCustomer(c.Request.Context(), actor, custID)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, cust)
}

func (h *Handler) handleUpdateCustomer(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	custID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req updateCustomerRequest
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
	cust, err := h.svc.UpdateCustomer(c.Request.Context(), actor, peopleapp.UpdateCustomerParams{
		ID:              custID,
		FullName:        req.FullName,
		Phone:           req.Phone,
		Email:           req.Email,
		NationalityCode: req.NationalityCode,
		GovernmentID:    req.GovernmentID,
		Address:         req.Address,
		ExpectedVersion: version,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, cust)
}

func (h *Handler) handleArchiveCustomer(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	custID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req archiveCustomerRequest
	if c.Request.ContentLength > 0 {
		if err := platformhttp.DecodeJSON(c, &req); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
	}
	cust, err := h.svc.ArchiveCustomer(c.Request.Context(), actor, custID, strings.TrimSpace(req.Reason))
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, cust)
}

func (h *Handler) handleGetMe(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	profile, err := h.svc.GetMe(c.Request.Context(), actor)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, profile)
}

func (h *Handler) handleUpdateMe(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req updateMeRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	displayName := req.DisplayName
	if displayName == nil && req.FullName != nil {
		displayName = req.FullName
	}
	profile, err := h.svc.UpdateMe(c.Request.Context(), actor, peopleapp.UpdateMeParams{
		DisplayName:     displayName,
		Phone:           req.Phone,
		NationalityCode: req.NationalityCode,
		GovernmentID:    req.GovernmentID,
		Address:         req.Address,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, profile)
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
