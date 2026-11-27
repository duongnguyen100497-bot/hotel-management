package application

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/domain"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module callers.
type (
	Role       = domain.Role
	Permission = domain.Permission
)

const (
	RoleSystemAdministrator = domain.RoleSystemAdministrator
	RoleHotelManager        = domain.RoleHotelManager
	RoleFrontDesk           = domain.RoleFrontDesk
	RoleAccounting          = domain.RoleAccounting
)

var codeSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,98}[a-z0-9]$`)

// Authorizer enforces dynamic RBAC authorization from PostgreSQL with an immutable code-level root bypass.
type Authorizer interface {
	Authorize(ctx context.Context, principal auth.Principal, permission string) error
	IsRootUser(ctx context.Context, userID uuid.UUID) (bool, error)
}

// RequirePermission returns a Gin middleware that enforces dynamic RBAC permission checks via Authorizer.
func RequirePermission(authorizer Authorizer, perm string) gin.HandlerFunc {
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
		if err := authorizer.Authorize(c.Request.Context(), principal, perm); err != nil {
			platformhttp.WriteError(c, err)
			return
		}
		c.Next()
	}
}

// Repository defines the persistence contract for RBAC roles, permissions, and bindings.
type Repository interface {
	RunSeed(ctx context.Context) error

	ListPermissions(ctx context.Context) ([]domain.Permission, error)
	GetPermissionByID(ctx context.Context, id uuid.UUID) (domain.Permission, error)
	GetPermissionByCode(ctx context.Context, code string) (domain.Permission, error)
	ListPermissionsByCodes(ctx context.Context, codes []string) ([]domain.Permission, error)
	ListPermissionsByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Permission, error)
	CreatePermission(ctx context.Context, perm domain.Permission) (domain.Permission, error)
	UpdatePermission(ctx context.Context, id uuid.UUID, name, description string) (domain.Permission, error)

	ListRoles(ctx context.Context) ([]domain.Role, error)
	GetRoleByID(ctx context.Context, id uuid.UUID) (domain.Role, error)
	GetRoleByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Role, error)
	GetRoleByCode(ctx context.Context, code string) (domain.Role, error)
	GetRootRole(ctx context.Context) (domain.Role, error)
	ListRolesByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Role, error)
	ListRolesByCodes(ctx context.Context, codes []string) ([]domain.Role, error)
	CreateRole(ctx context.Context, role domain.Role) (domain.Role, error)
	UpdateRole(ctx context.Context, id uuid.UUID, name, description string, now time.Time) (domain.Role, error)
	ArchiveRole(ctx context.Context, id uuid.UUID, now time.Time) (domain.Role, error)
	DeleteRole(ctx context.Context, id uuid.UUID) (int64, error)

	ListRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
	ReplaceRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error

	ListUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
	ReplaceUserRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error
	InsertUserRole(ctx context.Context, userID, roleID uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error
	CountUsersAssignedToRole(ctx context.Context, roleID uuid.UUID) (int, error)
	ListUserIDsByRole(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error)

	CountActiveRootAdmins(ctx context.Context) (int, error)
	LockActiveRootAdminUsers(ctx context.Context) ([]uuid.UUID, error)
	UserHasActiveRootRole(ctx context.Context, userID uuid.UUID) (bool, error)
	UserHasEffectivePermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error)
	ListUserEffectivePermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
	ListUsersWithRoles(ctx context.Context, limit int) ([]rbacpg.UserAccountRow, error)
}

// IdentityPort defines the narrow cross-module dependency from RBAC to Identity.
type IdentityPort interface {
	CreateActiveAdminUser(ctx context.Context, params identityapp.CreateActiveAdminParams) (identityapp.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (identityapp.User, error)
	GetUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (identityapp.User, error)
	BumpAuthVersion(ctx context.Context, userIDs ...uuid.UUID) error
}

// BootstrapAdminParams configures the idempotent root admin bootstrap process.
type BootstrapAdminParams struct {
	Email       string
	Password    string
	DisplayName string
}

// BootstrapResult summarizes the outcome of running BootstrapRootAdmin.
type BootstrapResult struct {
	CreatedRootAdmin bool
	RootUserID       uuid.UUID
	RootRoleID       uuid.UUID
}

// CreatePermissionParams defines inputs for creating a new permission code.
type CreatePermissionParams struct {
	Code        string
	Name        string
	Description string
}

// UpdatePermissionParams defines inputs for updating a permission's mutable display metadata (code is immutable).
type UpdatePermissionParams struct {
	ID          uuid.UUID
	Name        string
	Description string
}

// CreateRoleParams defines inputs for creating a custom non-system, non-root role.
type CreateRoleParams struct {
	Code        string
	Name        string
	Description string
}

// UpdateRoleParams defines inputs for updating a role's name and description.
type UpdateRoleParams struct {
	ID          uuid.UUID
	Name        string
	Description string
}

// AssignRolePermissionsParams defines inputs for replacing a role's permission bindings.
type AssignRolePermissionsParams struct {
	RoleID          uuid.UUID
	PermissionCodes []string
	PermissionIDs   []uuid.UUID
	Reason          string
}

// AssignUserRolesParams defines inputs for replacing a staff/admin user's role bindings.
type AssignUserRolesParams struct {
	TargetUserID uuid.UUID
	RoleCodes    []string
	RoleIDs      []uuid.UUID
	Reason       string
}

// UserWithRoles bundles a user account summary with their active assigned roles.
type UserWithRoles struct {
	ID                   uuid.UUID     `json:"id"`
	Kind                 string        `json:"kind"`
	Email                string        `json:"email"`
	Username             string        `json:"username,omitempty"`
	DisplayName          string        `json:"displayName"`
	Status               string        `json:"status"`
	AuthorizationVersion int           `json:"authorizationVersion"`
	LastLoginAt          *time.Time    `json:"lastLoginAt,omitempty"`
	CreatedAt            time.Time     `json:"createdAt"`
	UpdatedAt            time.Time     `json:"updatedAt"`
	Version              int           `json:"version"`
	Roles                []domain.Role `json:"roles"`
}

// Service implements RBAC seed, bootstrap, dynamic authorization, and role/permission management.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	identity IdentityPort
	audit    auditapp.Recorder
	clk      clock.Clock
	ids      id.Generator
}

// Compile-time check that Service implements Authorizer.
var _ Authorizer = (*Service)(nil)

// NewService constructs an RBAC application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	identity IdentityPort,
	audit auditapp.Recorder,
	clk clock.Clock,
	ids id.Generator,
) *Service {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if ids == nil {
		ids = id.NewUUIDGenerator()
	}
	return &Service{
		txRunner: txRunner,
		repo:     repo,
		identity: identity,
		audit:    audit,
		clk:      clk,
		ids:      ids,
	}
}

// Authorize checks whether principal holds permission.
// Code-level invariant: any active user holding an active role with is_root = true
// automatically bypasses all permission checks (including future permissions never added to role_permissions).
func (s *Service) Authorize(ctx context.Context, principal auth.Principal, permission string) error {
	if principal.UserID == uuid.Nil {
		return apperror.Unauthenticated("authentication required")
	}
	perm := strings.TrimSpace(permission)
	if perm == "" {
		return nil
	}

	isRoot, err := s.repo.UserHasActiveRootRole(ctx, principal.UserID)
	if err != nil {
		return err
	}
	if isRoot {
		return nil
	}

	hasPerm, err := s.repo.UserHasEffectivePermission(ctx, principal.UserID, perm)
	if err != nil {
		return err
	}
	if !hasPerm {
		return apperror.Forbidden("insufficient permissions: missing " + perm)
	}
	return nil
}

// IsRootUser reports whether userID currently holds an active root role.
func (s *Service) IsRootUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.repo.UserHasActiveRootRole(ctx, userID)
}

// SeedRBAC runs the idempotent RBAC permission and default system roles seed.
func (s *Service) SeedRBAC(ctx context.Context) error {
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		return s.repo.RunSeed(txCtx)
	})
}

// BootstrapRootAdmin seeds all canonical permissions and default roles idempotently,
// and creates the single active Root Admin user if and only if no active user holds the root role yet.
func (s *Service) BootstrapRootAdmin(ctx context.Context, params BootstrapAdminParams) (BootstrapResult, error) {
	if s.identity == nil {
		return BootstrapResult{}, apperror.Internal(nil)
	}
	var res BootstrapResult

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.RunSeed(txCtx); err != nil {
			return err
		}

		rootRole, err := s.repo.GetRootRole(txCtx)
		if err != nil {
			return err
		}
		res.RootRoleID = rootRole.ID

		if _, err := s.repo.GetRoleByIDForUpdate(txCtx, rootRole.ID); err != nil {
			return err
		}
		existingRoots, err := s.repo.LockActiveRootAdminUsers(txCtx)
		if err != nil {
			return err
		}
		if len(existingRoots) > 0 {
			res.CreatedRootAdmin = false
			res.RootUserID = existingRoots[0]
			return nil
		}

		user, err := s.identity.CreateActiveAdminUser(txCtx, identityapp.CreateActiveAdminParams{
			Email:       params.Email,
			Password:    params.Password,
			DisplayName: params.DisplayName,
		})
		if err != nil {
			return err
		}

		now := s.clk.Now()
		if err := s.repo.InsertUserRole(txCtx, user.ID, rootRole.ID, &user.ID, now); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:  &user.ID,
				Action:       "identity.bootstrap_root_admin",
				ResourceType: "user",
				ResourceID:   &user.ID,
				Reason:       "initial root administrator bootstrap",
				AfterData: map[string]any{
					"email":        user.Email,
					"display_name": user.DisplayName,
					"kind":         user.Kind,
					"status":       user.Status,
					"role_code":    rootRole.Code,
					"is_root":      true,
				},
			}); err != nil {
				return err
			}
		}

		res.CreatedRootAdmin = true
		res.RootUserID = user.ID
		return nil
	})
	return res, err
}

// ListPermissions returns all active permissions in the system.
func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	return s.repo.ListPermissions(ctx)
}

// CreatePermission adds a custom permission to the registry (code is immutable once created).
func (s *Service) CreatePermission(ctx context.Context, actor auth.Principal, params CreatePermissionParams) (domain.Permission, error) {
	if err := s.Authorize(ctx, actor, "permission.manage"); err != nil {
		return domain.Permission{}, err
	}
	code := strings.ToLower(strings.TrimSpace(params.Code))
	if !codeSlugPattern.MatchString(code) {
		return domain.Permission{}, apperror.Validation("invalid permission code", apperror.FieldError{
			Field:   "code",
			Message: "must be lowercase alphanumeric with dots, hyphens or underscores (3..100 chars)",
		})
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return domain.Permission{}, apperror.Validation("invalid permission name", apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}

	var created domain.Permission
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		p, err := s.repo.CreatePermission(txCtx, domain.Permission{
			ID:          s.ids.NewUUID(),
			Code:        code,
			Name:        name,
			Description: strings.TrimSpace(params.Description),
			IsSystem:    false,
		})
		if err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.permission_created",
				ResourceType:  "permission",
				ResourceID:    &p.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"code":        p.Code,
					"name":        p.Name,
					"description": p.Description,
				},
			}); err != nil {
				return err
			}
		}
		created = p
		return nil
	})
	return created, err
}

// UpdatePermission updates a permission's display name and description (code is immutable).
func (s *Service) UpdatePermission(ctx context.Context, actor auth.Principal, params UpdatePermissionParams) (domain.Permission, error) {
	if err := s.Authorize(ctx, actor, "permission.manage"); err != nil {
		return domain.Permission{}, err
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return domain.Permission{}, apperror.Validation("invalid permission name", apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}

	var updated domain.Permission
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetPermissionByID(txCtx, params.ID)
		if err != nil {
			return err
		}
		after, err := s.repo.UpdatePermission(txCtx, params.ID, name, params.Description)
		if err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.permission_updated",
				ResourceType:  "permission",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData: map[string]any{
					"code":        before.Code,
					"name":        before.Name,
					"description": before.Description,
				},
				AfterData: map[string]any{
					"code":        after.Code,
					"name":        after.Name,
					"description": after.Description,
				},
			}); err != nil {
				return err
			}
		}
		updated = after
		return nil
	})
	return updated, err
}

// ListRoles returns all active roles with their assigned permissions populated.
func (s *Service) ListRoles(ctx context.Context) ([]domain.Role, error) {
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		perms, err := s.repo.ListRolePermissions(ctx, roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = perms
	}
	return roles, nil
}

// CreateRole creates a custom role (always is_system=false, is_root=false).
func (s *Service) CreateRole(ctx context.Context, actor auth.Principal, params CreateRoleParams) (domain.Role, error) {
	if err := s.Authorize(ctx, actor, "role.create"); err != nil {
		return domain.Role{}, err
	}
	code := strings.ToLower(strings.TrimSpace(params.Code))
	if !codeSlugPattern.MatchString(code) {
		return domain.Role{}, apperror.Validation("invalid role code", apperror.FieldError{
			Field:   "code",
			Message: "must be lowercase alphanumeric with hyphens or underscores (3..64 chars)",
		})
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return domain.Role{}, apperror.Validation("invalid role name", apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}

	now := s.clk.Now()
	var created domain.Role
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.repo.CreateRole(txCtx, domain.Role{
			ID:          s.ids.NewUUID(),
			Code:        code,
			Name:        name,
			Description: strings.TrimSpace(params.Description),
			IsSystem:    false,
			IsRoot:      false,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.role_created",
				ResourceType:  "role",
				ResourceID:    &r.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"code":        r.Code,
					"name":        r.Name,
					"description": r.Description,
				},
			}); err != nil {
				return err
			}
		}
		created = r
		return nil
	})
	return created, err
}

// UpdateRole updates a role's name and description. Updating a system role requires role.update_system.
func (s *Service) UpdateRole(ctx context.Context, actor auth.Principal, params UpdateRoleParams) (domain.Role, error) {
	if err := s.Authorize(ctx, actor, "role.update"); err != nil {
		return domain.Role{}, err
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return domain.Role{}, apperror.Validation("invalid role name", apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}

	now := s.clk.Now()
	var updated domain.Role
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetRoleByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if before.ArchivedAt != nil {
			return apperror.Conflict("cannot update an archived role")
		}
		if before.IsSystem {
			if err := s.Authorize(txCtx, actor, "role.update_system"); err != nil {
				return err
			}
		}
		after, err := s.repo.UpdateRole(txCtx, params.ID, name, params.Description, now)
		if err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.role_updated",
				ResourceType:  "role",
				ResourceID:    &after.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData: map[string]any{
					"name":        before.Name,
					"description": before.Description,
				},
				AfterData: map[string]any{
					"name":        after.Name,
					"description": after.Description,
				},
			}); err != nil {
				return err
			}
		}
		updated = after
		return nil
	})
	return updated, err
}

// DeleteOrArchiveRole archives a custom role if it is not a system/root role and has no assigned users.
func (s *Service) DeleteOrArchiveRole(ctx context.Context, actor auth.Principal, roleID uuid.UUID) error {
	if err := s.Authorize(ctx, actor, "role.delete"); err != nil {
		return err
	}

	now := s.clk.Now()
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		role, err := s.repo.GetRoleByIDForUpdate(txCtx, roleID)
		if err != nil {
			return err
		}
		if role.IsRoot || role.IsSystem {
			return apperror.Conflict("system and root roles cannot be deleted or archived")
		}
		assignedCount, err := s.repo.CountUsersAssignedToRole(txCtx, role.ID)
		if err != nil {
			return err
		}
		if assignedCount > 0 {
			return apperror.Conflict("cannot delete or archive a role while users are still assigned to it")
		}

		if _, err := s.repo.ArchiveRole(txCtx, role.ID, now); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.role_archived",
				ResourceType:  "role",
				ResourceID:    &role.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				BeforeData: map[string]any{
					"code": role.Code,
					"name": role.Name,
				},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetRolePermissions replaces the permission set of a role.
// Invariants enforced:
// 1. Root role permissions cannot be stripped/modified.
// 2. Modifying a system role requires role.update_system.
// 3. Grantable-set check: a non-root caller can only grant permissions that the caller currently possesses.
// 4. All users assigned to the role have their authorization_version incremented in the same transaction.
func (s *Service) SetRolePermissions(ctx context.Context, actor auth.Principal, params AssignRolePermissionsParams) (domain.Role, error) {
	if err := s.Authorize(ctx, actor, "role_permission.assign"); err != nil {
		return domain.Role{}, err
	}

	now := s.clk.Now()
	var updatedRole domain.Role

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		role, err := s.repo.GetRoleByIDForUpdate(txCtx, params.RoleID)
		if err != nil {
			return err
		}
		if role.ArchivedAt != nil {
			return apperror.Conflict("cannot modify permissions of an archived role")
		}
		if role.IsRoot {
			return apperror.Conflict("root administrator role permissions are immutable")
		}
		if role.IsSystem {
			if err := s.Authorize(txCtx, actor, "role.update_system"); err != nil {
				return err
			}
		}

		targetPerms, err := s.resolvePermissions(txCtx, params.PermissionCodes, params.PermissionIDs)
		if err != nil {
			return err
		}

		// Grantable-set check for non-root callers.
		if err := s.enforceGrantablePermissions(txCtx, actor.UserID, targetPerms); err != nil {
			return err
		}

		beforePerms, err := s.repo.ListRolePermissions(txCtx, role.ID)
		if err != nil {
			return err
		}

		permIDs := make([]uuid.UUID, 0, len(targetPerms))
		afterCodes := make([]string, 0, len(targetPerms))
		for _, p := range targetPerms {
			permIDs = append(permIDs, p.ID)
			afterCodes = append(afterCodes, p.Code)
		}
		sort.Strings(afterCodes)

		beforeCodes := make([]string, 0, len(beforePerms))
		for _, p := range beforePerms {
			beforeCodes = append(beforeCodes, p.Code)
		}
		sort.Strings(beforeCodes)

		if err := s.repo.ReplaceRolePermissions(txCtx, role.ID, permIDs, &actor.UserID, now); err != nil {
			return err
		}

		affectedUsers, err := s.repo.ListUserIDsByRole(txCtx, role.ID)
		if err != nil {
			return err
		}
		if len(affectedUsers) > 0 && s.identity != nil {
			if err := s.identity.BumpAuthVersion(txCtx, affectedUsers...); err != nil {
				return err
			}
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.role_permissions_updated",
				ResourceType:  "role",
				ResourceID:    &role.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				Reason:        strings.TrimSpace(params.Reason),
				BeforeData: map[string]any{
					"role_code":   role.Code,
					"permissions": beforeCodes,
				},
				AfterData: map[string]any{
					"role_code":   role.Code,
					"permissions": afterCodes,
				},
			}); err != nil {
				return err
			}
		}

		role.Permissions = targetPerms
		updatedRole = role
		return nil
	})
	return updatedRole, err
}

// SetUserRoles replaces the roles assigned to a staff/admin user.
// Invariants enforced:
// 1. Cannot assign staff/admin roles to a customer account.
// 2. Non-root callers cannot assign the root role, nor any role whose permissions exceed the caller's own grantable set.
// 3. Last active Root Admin protection: cannot remove the root role from the last active root administrator (returns 409).
// 4. Increments the target user's authorization_version and records an audit event in the same transaction.
func (s *Service) SetUserRoles(ctx context.Context, actor auth.Principal, params AssignUserRolesParams) ([]domain.Role, error) {
	if err := s.Authorize(ctx, actor, "user_role.assign"); err != nil {
		return nil, err
	}
	if s.identity == nil {
		return nil, apperror.Internal(nil)
	}

	now := s.clk.Now()
	var assignedRoles []domain.Role

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		targetUser, err := s.identity.GetUserByIDForUpdate(txCtx, params.TargetUserID)
		if err != nil {
			return err
		}
		if targetUser.Kind == identityapp.UserKindCustomer {
			return apperror.Validation("cannot assign RBAC roles to a customer account", apperror.FieldError{
				Field:   "userId",
				Message: "must be a staff or admin account",
			})
		}

		targetRoles, err := s.resolveRoles(txCtx, params.RoleCodes, params.RoleIDs)
		if err != nil {
			return err
		}

		if err := s.enforceGrantableRoles(txCtx, actor.UserID, targetRoles); err != nil {
			return err
		}

		beforeRoles, err := s.repo.ListUserRoles(txCtx, targetUser.ID)
		if err != nil {
			return err
		}

		hadRoot := false
		beforeCodes := make([]string, 0, len(beforeRoles))
		for _, r := range beforeRoles {
			beforeCodes = append(beforeCodes, r.Code)
			if r.IsRoot {
				hadRoot = true
			}
		}
		sort.Strings(beforeCodes)

		willHaveRoot := false
		roleIDs := make([]uuid.UUID, 0, len(targetRoles))
		afterCodes := make([]string, 0, len(targetRoles))
		for _, r := range targetRoles {
			roleIDs = append(roleIDs, r.ID)
			afterCodes = append(afterCodes, r.Code)
			if r.IsRoot {
				willHaveRoot = true
			}
		}
		sort.Strings(afterCodes)

		// Protect the last active Root Admin!
		if hadRoot && !willHaveRoot && targetUser.Status == identityapp.UserStatusActive {
			activeRootIDs, err := s.repo.LockActiveRootAdminUsers(txCtx)
			if err != nil {
				return err
			}
			if len(activeRootIDs) <= 1 {
				return apperror.Conflict("cannot remove root role from the last active root administrator")
			}
		}

		if err := s.repo.ReplaceUserRoles(txCtx, targetUser.ID, roleIDs, &actor.UserID, now); err != nil {
			return err
		}
		if err := s.identity.BumpAuthVersion(txCtx, targetUser.ID); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &actor.UserID,
				Action:        "rbac.user_roles_updated",
				ResourceType:  "user",
				ResourceID:    &targetUser.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				Reason:        strings.TrimSpace(params.Reason),
				BeforeData: map[string]any{
					"roles": beforeCodes,
				},
				AfterData: map[string]any{
					"roles": afterCodes,
				},
			}); err != nil {
				return err
			}
		}

		assignedRoles = targetRoles
		return nil
	})
	return assignedRoles, err
}

// AssignInitialRoles assigns initial roles to a newly created staff user inside an active transaction,
// verifying the caller's grantable-set rules.
func (s *Service) AssignInitialRoles(ctx context.Context, actor auth.Principal, targetUserID uuid.UUID, roleCodes []string, roleIDs []uuid.UUID) ([]domain.Role, error) {
	if len(roleCodes) == 0 && len(roleIDs) == 0 {
		return nil, nil
	}
	targetRoles, err := s.resolveRoles(ctx, roleCodes, roleIDs)
	if err != nil {
		return nil, err
	}
	if err := s.enforceGrantableRoles(ctx, actor.UserID, targetRoles); err != nil {
		return nil, err
	}
	now := s.clk.Now()
	ids := make([]uuid.UUID, 0, len(targetRoles))
	for _, r := range targetRoles {
		ids = append(ids, r.ID)
	}
	if err := s.repo.ReplaceUserRoles(ctx, targetUserID, ids, &actor.UserID, now); err != nil {
		return nil, err
	}
	return targetRoles, nil
}

// EnsureNotLastActiveRootAdmin checks inside an active transaction whether deactivating targetUserID
// would leave the system with zero active root administrators.
func (s *Service) EnsureNotLastActiveRootAdmin(ctx context.Context, targetUserID uuid.UUID) error {
	isRoot, err := s.repo.UserHasActiveRootRole(ctx, targetUserID)
	if err != nil {
		return err
	}
	if !isRoot {
		return nil
	}
	activeRoots, err := s.repo.LockActiveRootAdminUsers(ctx)
	if err != nil {
		return err
	}
	if len(activeRoots) <= 1 {
		return apperror.Conflict("cannot deactivate the last active root administrator")
	}
	return nil
}

// ListUsers returns users along with their active assigned roles.
func (s *Service) ListUsers(ctx context.Context, actor auth.Principal, limit int) ([]UserWithRoles, error) {
	if err := s.Authorize(ctx, actor, "user.read"); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListUsersWithRoles(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]UserWithRoles, 0, len(rows))
	for _, row := range rows {
		roles, err := s.repo.ListUserRoles(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, UserWithRoles{
			ID:                   row.ID,
			Kind:                 row.Kind,
			Email:                row.Email,
			Username:             row.Username,
			DisplayName:          row.DisplayName,
			Status:               row.Status,
			AuthorizationVersion: row.AuthorizationVersion,
			LastLoginAt:          row.LastLoginAt,
			CreatedAt:            row.CreatedAt,
			UpdatedAt:            row.UpdatedAt,
			Version:              row.Version,
			Roles:                roles,
		})
	}
	return out, nil
}

// ListUserRoles returns the active roles assigned to userID.
func (s *Service) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	return s.repo.ListUserRoles(ctx, userID)
}

// CountActiveRootAdmins returns the count of active users currently holding the root role.
func (s *Service) CountActiveRootAdmins(ctx context.Context) (int, error) {
	return s.repo.CountActiveRootAdmins(ctx)
}

func (s *Service) resolvePermissions(ctx context.Context, codes []string, ids []uuid.UUID) ([]domain.Permission, error) {
	seen := make(map[uuid.UUID]domain.Permission)

	if len(codes) > 0 {
		cleanCodes := uniqueTrimmedStrings(codes)
		perms, err := s.repo.ListPermissionsByCodes(ctx, cleanCodes)
		if err != nil {
			return nil, err
		}
		if len(perms) != len(cleanCodes) {
			return nil, apperror.Validation("one or more permission codes do not exist", apperror.FieldError{
				Field:   "permissionCodes",
				Message: "contains unknown or archived permission code",
			})
		}
		for _, p := range perms {
			seen[p.ID] = p
		}
	}

	if len(ids) > 0 {
		cleanIDs := uniqueUUIDs(ids)
		perms, err := s.repo.ListPermissionsByIDs(ctx, cleanIDs)
		if err != nil {
			return nil, err
		}
		if len(perms) != len(cleanIDs) {
			return nil, apperror.Validation("one or more permission IDs do not exist", apperror.FieldError{
				Field:   "permissionIds",
				Message: "contains unknown or archived permission ID",
			})
		}
		for _, p := range perms {
			seen[p.ID] = p
		}
	}

	out := make([]domain.Permission, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Service) resolveRoles(ctx context.Context, codes []string, ids []uuid.UUID) ([]domain.Role, error) {
	seen := make(map[uuid.UUID]domain.Role)

	if len(codes) > 0 {
		cleanCodes := uniqueTrimmedStrings(codes)
		roles, err := s.repo.ListRolesByCodes(ctx, cleanCodes)
		if err != nil {
			return nil, err
		}
		if len(roles) != len(cleanCodes) {
			return nil, apperror.Validation("one or more role codes do not exist", apperror.FieldError{
				Field:   "roleCodes",
				Message: "contains unknown or archived role code",
			})
		}
		for _, r := range roles {
			seen[r.ID] = r
		}
	}

	if len(ids) > 0 {
		cleanIDs := uniqueUUIDs(ids)
		roles, err := s.repo.ListRolesByIDs(ctx, cleanIDs)
		if err != nil {
			return nil, err
		}
		if len(roles) != len(cleanIDs) {
			return nil, apperror.Validation("one or more role IDs do not exist", apperror.FieldError{
				Field:   "roleIds",
				Message: "contains unknown or archived role ID",
			})
		}
		for _, r := range roles {
			seen[r.ID] = r
		}
	}

	out := make([]domain.Role, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Service) enforceGrantablePermissions(ctx context.Context, actorUserID uuid.UUID, perms []domain.Permission) error {
	if len(perms) == 0 {
		return nil
	}
	isRoot, err := s.repo.UserHasActiveRootRole(ctx, actorUserID)
	if err != nil {
		return err
	}
	if isRoot {
		return nil
	}

	actorCodes, err := s.repo.ListUserEffectivePermissionCodes(ctx, actorUserID)
	if err != nil {
		return err
	}
	held := make(map[string]struct{}, len(actorCodes))
	for _, c := range actorCodes {
		held[c] = struct{}{}
	}
	for _, p := range perms {
		if _, ok := held[p.Code]; !ok {
			return apperror.Forbidden("privilege escalation denied: caller does not hold permission " + p.Code)
		}
	}
	return nil
}

func (s *Service) enforceGrantableRoles(ctx context.Context, actorUserID uuid.UUID, roles []domain.Role) error {
	if len(roles) == 0 {
		return nil
	}
	isRoot, err := s.repo.UserHasActiveRootRole(ctx, actorUserID)
	if err != nil {
		return err
	}
	if isRoot {
		return nil
	}

	actorCodes, err := s.repo.ListUserEffectivePermissionCodes(ctx, actorUserID)
	if err != nil {
		return err
	}
	held := make(map[string]struct{}, len(actorCodes))
	for _, c := range actorCodes {
		held[c] = struct{}{}
	}

	for _, r := range roles {
		if r.IsRoot {
			return apperror.Forbidden("privilege escalation denied: only a root administrator can grant the root role")
		}
		rolePerms, err := s.repo.ListRolePermissions(ctx, r.ID)
		if err != nil {
			return err
		}
		for _, p := range rolePerms {
			if _, ok := held[p.Code]; !ok {
				return apperror.Forbidden("privilege escalation denied: role " + r.Code + " contains permission " + p.Code + " not held by caller")
			}
		}
	}
	return nil
}

func uniqueTrimmedStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; !ok {
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}

func uniqueUUIDs(in []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
