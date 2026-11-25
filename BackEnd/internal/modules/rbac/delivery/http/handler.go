package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// Handler serves dynamic RBAC management endpoints (/permissions, /roles, /users).
type Handler struct {
	svc *rbacapp.Service
}

// NewHandler constructs an RBAC HTTP handler.
func NewHandler(svc *rbacapp.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts authenticated RBAC routes on api (/api/v1).
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	secured := api.Group("")
	if authMiddleware != nil {
		secured.Use(authMiddleware)
	}

	secured.GET("/permissions", rbacapp.RequirePermission(h.svc, "permission.read"), h.handleListPermissions)
	secured.POST("/permissions", rbacapp.RequirePermission(h.svc, "permission.manage"), h.handleCreatePermission)
	secured.PATCH("/permissions/:id", rbacapp.RequirePermission(h.svc, "permission.manage"), h.handleUpdatePermission)

	secured.GET("/roles", rbacapp.RequirePermission(h.svc, "role.read"), h.handleListRoles)
	secured.POST("/roles", rbacapp.RequirePermission(h.svc, "role.create"), h.handleCreateRole)
	secured.PATCH("/roles/:id", rbacapp.RequirePermission(h.svc, "role.update"), h.handleUpdateRole)
	secured.DELETE("/roles/:id", rbacapp.RequirePermission(h.svc, "role.delete"), h.handleDeleteRole)
	secured.PUT("/roles/:id/permissions", rbacapp.RequirePermission(h.svc, "role_permission.assign"), h.handleSetRolePermissions)

	secured.GET("/users", rbacapp.RequirePermission(h.svc, "user.read"), h.handleListUsers)
	secured.PUT("/users/:id/roles", rbacapp.RequirePermission(h.svc, "user_role.assign"), h.handleSetUserRoles)
}

type createPermissionRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type updatePermissionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type createRoleRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type updateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type setRolePermissionsRequest struct {
	PermissionCodes []string    `json:"permissionCodes,omitempty"`
	PermissionIDs   []uuid.UUID `json:"permissionIds,omitempty"`
	Reason          string      `json:"reason,omitempty"`
}

type setUserRolesRequest struct {
	RoleCodes []string    `json:"roleCodes,omitempty"`
	RoleIDs   []uuid.UUID `json:"roleIds,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

func (h *Handler) handleListPermissions(c *gin.Context) {
	perms, err := h.svc.ListPermissions(c.Request.Context())
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": perms})
}

func (h *Handler) handleCreatePermission(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createPermissionRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	perm, err := h.svc.CreatePermission(c.Request.Context(), actor, rbacapp.CreatePermissionParams{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusCreated, perm)
}

func (h *Handler) handleUpdatePermission(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	permID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req updatePermissionRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	perm, err := h.svc.UpdatePermission(c.Request.Context(), actor, rbacapp.UpdatePermissionParams{
		ID:          permID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, perm)
}

func (h *Handler) handleListRoles(c *gin.Context) {
	roles, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": roles})
}

func (h *Handler) handleCreateRole(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	var req createRoleRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	role, err := h.svc.CreateRole(c.Request.Context(), actor, rbacapp.CreateRoleParams{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusCreated, role)
}

func (h *Handler) handleUpdateRole(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roleID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req updateRoleRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	role, err := h.svc.UpdateRole(c.Request.Context(), actor, rbacapp.UpdateRoleParams{
		ID:          roleID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, role)
}

func (h *Handler) handleDeleteRole(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roleID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	if err := h.svc.DeleteOrArchiveRole(c.Request.Context(), actor, roleID); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) handleSetRolePermissions(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	roleID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req setRolePermissionsRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	role, err := h.svc.SetRolePermissions(c.Request.Context(), actor, rbacapp.AssignRolePermissionsParams{
		RoleID:          roleID,
		PermissionCodes: req.PermissionCodes,
		PermissionIDs:   req.PermissionIDs,
		Reason:          req.Reason,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, role)
}

func (h *Handler) handleListUsers(c *gin.Context) {
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
	users, err := h.svc.ListUsers(c.Request.Context(), actor, limit)
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{"data": users})
}

func (h *Handler) handleSetUserRoles(c *gin.Context) {
	actor, ok := auth.PrincipalFromContext(c.Request.Context())
	if !ok {
		platformhttp.WriteError(c, apperror.Unauthenticated("authentication required"))
		return
	}
	targetUserID, err := parseUUIDParam(c, "id")
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	var req setUserRolesRequest
	if err := platformhttp.DecodeJSON(c, &req); err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	roles, err := h.svc.SetUserRoles(c.Request.Context(), actor, rbacapp.AssignUserRolesParams{
		TargetUserID: targetUserID,
		RoleCodes:    req.RoleCodes,
		RoleIDs:      req.RoleIDs,
		Reason:       req.Reason,
	})
	if err != nil {
		platformhttp.WriteError(c, err)
		return
	}
	platformhttp.WriteJSON(c, http.StatusOK, gin.H{
		"userId": targetUserID,
		"roles":  roles,
	})
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
