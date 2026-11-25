package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/db/seed"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository persists roles, permissions, role_permissions, and user_roles in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL RBAC repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

// RunSeed executes the embedded idempotent RBAC seed SQL inside the active transaction or pool.
func (r *Repository) RunSeed(ctx context.Context) error {
	_, err := db.Querier(ctx, r.pool).Exec(ctx, seed.RBACSeedSQL)
	return db.MapPgError(err)
}

func (r *Repository) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.queries(ctx).ListPermissions(ctx)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPermissionRow(row))
	}
	return out, nil
}

func (r *Repository) GetPermissionByID(ctx context.Context, id uuid.UUID) (domain.Permission, error) {
	row, err := r.queries(ctx).GetPermissionByID(ctx, id)
	if err != nil {
		return domain.Permission{}, db.MapPgError(err)
	}
	return mapPermissionRow(row), nil
}

func (r *Repository) GetPermissionByCode(ctx context.Context, code string) (domain.Permission, error) {
	row, err := r.queries(ctx).GetPermissionByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.Permission{}, db.MapPgError(err)
	}
	return mapPermissionRow(row), nil
}

func (r *Repository) ListPermissionsByCodes(ctx context.Context, codes []string) ([]domain.Permission, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListPermissionsByCodes(ctx, codes)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPermissionRow(row))
	}
	return out, nil
}

func (r *Repository) ListPermissionsByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Permission, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListPermissionsByIDs(ctx, ids)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPermissionRow(row))
	}
	return out, nil
}

func (r *Repository) CreatePermission(ctx context.Context, perm domain.Permission) (domain.Permission, error) {
	var desc pgtype.Text
	if trimmed := strings.TrimSpace(perm.Description); trimmed != "" {
		desc = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := r.queries(ctx).CreatePermission(ctx, sqlcgen.CreatePermissionParams{
		ID:          perm.ID,
		Code:        strings.TrimSpace(perm.Code),
		Name:        strings.TrimSpace(perm.Name),
		Description: desc,
		IsSystem:    perm.IsSystem,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.Permission{}, apperror.Conflict("permission code already exists").WithCause(err)
		}
		return domain.Permission{}, db.MapPgError(err)
	}
	return mapPermissionRow(row), nil
}

func (r *Repository) UpdatePermission(ctx context.Context, id uuid.UUID, name, description string) (domain.Permission, error) {
	var desc pgtype.Text
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		desc = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := r.queries(ctx).UpdatePermission(ctx, sqlcgen.UpdatePermissionParams{
		ID:          id,
		Name:        strings.TrimSpace(name),
		Description: desc,
	})
	if err != nil {
		return domain.Permission{}, db.MapPgError(err)
	}
	return mapPermissionRow(row), nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.queries(ctx).ListRoles(ctx)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRoleRow(row))
	}
	return out, nil
}

func (r *Repository) GetRoleByID(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	row, err := r.queries(ctx).GetRoleByID(ctx, id)
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) GetRoleByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	row, err := r.queries(ctx).GetRoleByIDForUpdate(ctx, id)
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) GetRoleByCode(ctx context.Context, code string) (domain.Role, error) {
	row, err := r.queries(ctx).GetRoleByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) GetRootRole(ctx context.Context) (domain.Role, error) {
	row, err := r.queries(ctx).GetRootRole(ctx)
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) ListRolesByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListRolesByIDs(ctx, ids)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRoleRow(row))
	}
	return out, nil
}

func (r *Repository) ListRolesByCodes(ctx context.Context, codes []string) ([]domain.Role, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	rows, err := r.queries(ctx).ListRolesByCodes(ctx, codes)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRoleRow(row))
	}
	return out, nil
}

func (r *Repository) CreateRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	var desc pgtype.Text
	if trimmed := strings.TrimSpace(role.Description); trimmed != "" {
		desc = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := r.queries(ctx).CreateRole(ctx, sqlcgen.CreateRoleParams{
		ID:          role.ID,
		Code:        strings.TrimSpace(role.Code),
		Name:        strings.TrimSpace(role.Name),
		Description: desc,
		IsSystem:    role.IsSystem,
		IsRoot:      role.IsRoot,
		CreatedAt:   role.CreatedAt.UTC(),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.Role{}, apperror.Conflict("role code or root role constraint already exists").WithCause(err)
		}
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) UpdateRole(ctx context.Context, id uuid.UUID, name, description string, now time.Time) (domain.Role, error) {
	var desc pgtype.Text
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		desc = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := r.queries(ctx).UpdateRole(ctx, sqlcgen.UpdateRoleParams{
		ID:          id,
		Name:        strings.TrimSpace(name),
		Description: desc,
		UpdatedAt:   now.UTC(),
	})
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) ArchiveRole(ctx context.Context, id uuid.UUID, now time.Time) (domain.Role, error) {
	ts := now.UTC()
	row, err := r.queries(ctx).ArchiveRole(ctx, sqlcgen.ArchiveRoleParams{
		ID:         id,
		ArchivedAt: &ts,
	})
	if err != nil {
		return domain.Role{}, db.MapPgError(err)
	}
	return mapRoleRow(row), nil
}

func (r *Repository) DeleteRole(ctx context.Context, id uuid.UUID) (int64, error) {
	n, err := r.queries(ctx).DeleteRole(ctx, id)
	return n, db.MapPgError(err)
}

func (r *Repository) ListRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	rows, err := r.queries(ctx).ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPermissionRow(row))
	}
	return out, nil
}

func (r *Repository) ReplaceRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error {
	q := r.queries(ctx)
	if err := q.DeleteRolePermissionsByRole(ctx, roleID); err != nil {
		return db.MapPgError(err)
	}
	ts := grantedAt.UTC()
	for _, pid := range permissionIDs {
		if err := q.InsertRolePermission(ctx, sqlcgen.InsertRolePermissionParams{
			RoleID:       roleID,
			PermissionID: pid,
			GrantedBy:    grantedBy,
			GrantedAt:    ts,
		}); err != nil {
			return db.MapPgError(err)
		}
	}
	return nil
}

func (r *Repository) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	rows, err := r.queries(ctx).ListUserRoles(ctx, userID)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRoleRow(row))
	}
	return out, nil
}

func (r *Repository) ReplaceUserRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error {
	q := r.queries(ctx)
	if err := q.DeleteUserRolesByUser(ctx, userID); err != nil {
		return db.MapPgError(err)
	}
	ts := grantedAt.UTC()
	for _, rid := range roleIDs {
		if err := q.InsertUserRole(ctx, sqlcgen.InsertUserRoleParams{
			UserID:    userID,
			RoleID:    rid,
			GrantedBy: grantedBy,
			GrantedAt: ts,
		}); err != nil {
			return db.MapPgError(err)
		}
	}
	return nil
}

func (r *Repository) InsertUserRole(ctx context.Context, userID, roleID uuid.UUID, grantedBy *uuid.UUID, grantedAt time.Time) error {
	return db.MapPgError(r.queries(ctx).InsertUserRole(ctx, sqlcgen.InsertUserRoleParams{
		UserID:    userID,
		RoleID:    roleID,
		GrantedBy: grantedBy,
		GrantedAt: grantedAt.UTC(),
	}))
}

func (r *Repository) CountUsersAssignedToRole(ctx context.Context, roleID uuid.UUID) (int, error) {
	c, err := r.queries(ctx).CountUsersAssignedToRole(ctx, roleID)
	return int(c), db.MapPgError(err)
}

func (r *Repository) ListUserIDsByRole(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := r.queries(ctx).ListUserIDsByRole(ctx, roleID)
	return ids, db.MapPgError(err)
}

func (r *Repository) CountActiveRootAdmins(ctx context.Context) (int, error) {
	c, err := r.queries(ctx).CountActiveRootAdmins(ctx)
	return int(c), db.MapPgError(err)
}

func (r *Repository) LockActiveRootAdminUsers(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.queries(ctx).LockActiveRootAdminUsers(ctx)
	return ids, db.MapPgError(err)
}

func (r *Repository) UserHasActiveRootRole(ctx context.Context, userID uuid.UUID) (bool, error) {
	has, err := r.queries(ctx).UserHasActiveRootRole(ctx, userID)
	return has, db.MapPgError(err)
}

func (r *Repository) UserHasEffectivePermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	has, err := r.queries(ctx).UserHasEffectivePermission(ctx, sqlcgen.UserHasEffectivePermissionParams{
		ID:   userID,
		Code: strings.TrimSpace(permCode),
	})
	return has, db.MapPgError(err)
}

func (r *Repository) ListUserEffectivePermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	codes, err := r.queries(ctx).ListUserEffectivePermissionCodes(ctx, userID)
	return codes, db.MapPgError(err)
}

func (r *Repository) ListUsersWithRoles(ctx context.Context, limit int) ([]sqlcgen.ListUsersWithRolesRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.queries(ctx).ListUsersWithRoles(ctx, int32(limit))
	return rows, db.MapPgError(err)
}

func mapPermissionRow(row sqlcgen.Permission) domain.Permission {
	desc := ""
	if row.Description.Valid {
		desc = row.Description.String
	}
	return domain.Permission{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		Description: desc,
		IsSystem:    row.IsSystem,
		ArchivedAt:  row.ArchivedAt,
	}
}

func mapRoleRow(row sqlcgen.Role) domain.Role {
	desc := ""
	if row.Description.Valid {
		desc = row.Description.String
	}
	return domain.Role{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		Description: desc,
		IsSystem:    row.IsSystem,
		IsRoot:      row.IsRoot,
		ArchivedAt:  row.ArchivedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Version:     int(row.Version),
	}
}
