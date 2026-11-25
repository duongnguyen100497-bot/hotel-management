package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
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

// Service implements RBAC seed, bootstrap, dynamic authorization, and role/permission management.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	identity IdentityPort
	audit    auditapp.Recorder
	clk      clock.Clock
	ids      id.Generator
}

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

		// Lock any existing active root admin rows (and lock the root role row to serialize concurrent bootstrap runs).
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

// ListRoles returns all active roles in the system.
func (s *Service) ListRoles(ctx context.Context) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx)
}

// CountActiveRootAdmins returns the count of active users currently holding the root role.
func (s *Service) CountActiveRootAdmins(ctx context.Context) (int, error) {
	return s.repo.CountActiveRootAdmins(ctx)
}
