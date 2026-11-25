package application_test

import (
	"context"
	"testing"
	"time"

	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacdomain "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/domain"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestBootstrapRootAdmin_IdempotentAndSeedComplete(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	fixedNow := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedNow)
	ids := id.NewUUIDGenerator()

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids)
	rbacRepo := rbacpg.NewRepository(pg.Pool)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacRepo, identitySvc, auditSvc, clk, ids)

	// 1. First bootstrap run creates root admin and seeds all canonical permissions + 4 system roles.
	res1, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "ROOT.Admin@hotel.local",
		Password:    "StrongRootPass123",
		DisplayName: "Root System Admin",
	})
	if err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}
	if !res1.CreatedRootAdmin {
		t.Fatalf("expected CreatedRootAdmin=true on first run")
	}

	// Verify seeded permissions match CanonicalPermissions().
	perms, err := rbacSvc.ListPermissions(ctx)
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	canonical := rbacdomain.CanonicalPermissions()
	if len(perms) != len(canonical) {
		t.Fatalf("expected %d canonical permissions, got %d", len(canonical), len(perms))
	}

	// Verify 4 default system roles exist and exactly 1 has is_root=true.
	roles, err := rbacSvc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("expected 4 default roles, got %d", len(roles))
	}
	rootRoles := 0
	for _, r := range roles {
		if !r.IsSystem {
			t.Errorf("expected role %s to have IsSystem=true", r.Code)
		}
		if r.IsRoot {
			rootRoles++
			if r.Code != rbacdomain.RoleSystemAdministrator {
				t.Errorf("unexpected root role code: %s", r.Code)
			}
		}
	}
	if rootRoles != 1 {
		t.Fatalf("expected exactly 1 root role, got %d", rootRoles)
	}

	// Verify created root user and Argon2id password hash.
	rootUser, err := identitySvc.GetUserByID(ctx, res1.RootUserID)
	if err != nil {
		t.Fatalf("get root user: %v", err)
	}
	if rootUser.Email != "root.admin@hotel.local" {
		t.Errorf("expected normalized email root.admin@hotel.local, got %s", rootUser.Email)
	}
	if rootUser.Kind != identityapp.UserKindAdmin || rootUser.Status != identityapp.UserStatusActive {
		t.Errorf("unexpected user kind/status: %s/%s", rootUser.Kind, rootUser.Status)
	}

	cred, err := identityRepo.GetUserCredential(ctx, res1.RootUserID)
	if err != nil {
		t.Fatalf("get root user credential: %v", err)
	}
	ok, err := auth.VerifyPassword("StrongRootPass123", cred.PasswordHash)
	if err != nil || !ok {
		t.Fatalf("expected Argon2id password verification to succeed: ok=%v err=%v", ok, err)
	}

	// 2. Second bootstrap run with different email/password must be idempotent and NOT create a second root admin.
	res2, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "second-root@hotel.local",
		Password:    "AnotherStrongPass999",
		DisplayName: "Second Attempt",
	})
	if err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}
	if res2.CreatedRootAdmin {
		t.Fatalf("expected CreatedRootAdmin=false on second run")
	}
	if res2.RootUserID != res1.RootUserID {
		t.Fatalf("expected same RootUserID %s, got %s", res1.RootUserID, res2.RootUserID)
	}

	activeRoots, err := rbacSvc.CountActiveRootAdmins(ctx)
	if err != nil {
		t.Fatalf("count active root admins: %v", err)
	}
	if activeRoots != 1 {
		t.Fatalf("expected exactly 1 active root admin, got %d", activeRoots)
	}

	// 3. Partial unique index uq_roles_single_active_root must block creating another active is_root=true role.
	_, err = rbacRepo.CreateRole(ctx, rbacdomain.Role{
		ID:        ids.NewUUID(),
		Code:      "shadow-root",
		Name:      "Shadow Root",
		IsSystem:  false,
		IsRoot:    true,
		CreatedAt: fixedNow,
	})
	if err == nil {
		t.Fatalf("expected partial unique index to reject a second active is_root=true role")
	}

	// Verify audit event was recorded once for bootstrap.
	var auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events WHERE action = 'identity.bootstrap_root_admin'`).Scan(&auditCount); err != nil {
		t.Fatalf("count bootstrap audit events: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 bootstrap audit event, got %d", auditCount)
	}
}
