package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identityhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/delivery/http"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/domain"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbachttp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/delivery/http"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestDynamicRBAC_NegativeSecurityMatrixAndRootBypass(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	now := time.Date(2026, 11, 25, 16, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	ids := id.NewUUIDGenerator()

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	tokens := auth.NewTokenManager("test-access-secret-32-bytes-minimum!!", 15*time.Minute, 7*24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens)
	rbacRepo := rbacpg.NewRepository(pg.Pool)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacRepo, identitySvc, auditSvc, clk, ids)

	// Bootstrap root admin and seed permissions/roles.
	boot, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "RootPass123!",
		DisplayName: "Root Admin",
	})
	if err != nil {
		t.Fatalf("bootstrap root admin: %v", err)
	}

	authMW := auth.AuthenticatorMiddleware(tokens, identitySvc)
	idHandler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokens,
		SecureCookie: false,
	})
	rbacHandler := rbachttp.NewHandler(rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		idHandler.RegisterRoutes(api, authMW)
		rbacHandler.RegisterRoutes(api, authMW)

		// Route protected by a brand-new future permission that is NOT in role_permissions for any role.
		api.GET("/future-quantum-feature",
			authMW,
			rbacapp.RequirePermission(rbacSvc, "future.unassigned.capability"),
			func(c *gin.Context) {
				platformhttp.WriteJSON(c, http.StatusOK, gin.H{"ok": true})
			},
		)
	})

	doJSON := func(method, path string, body any, bearer string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	loginAs := func(email, password string) string {
		res := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    email,
			"password": password,
		}, "")
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed (%d): %s", email, res.Code, res.Body.String())
		}
		var payload struct {
			AccessToken string `json:"accessToken"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &payload)
		return payload.AccessToken
	}

	rootToken := loginAs("root@hotel.local", "RootPass123!")

	// 1. Unauthenticated request -> 401.
	unauthRes := doJSON(http.MethodGet, "/api/v1/roles", nil, "")
	if unauthRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated 401, got %d", unauthRes.Code)
	}

	// 2. Root admin accesses a brand-new permission never assigned in role_permissions -> 200 OK (code-level root bypass).
	futureRes := doJSON(http.MethodGet, "/api/v1/future-quantum-feature", nil, rootToken)
	if futureRes.Code != http.StatusOK {
		t.Fatalf("expected root admin to bypass unassigned future permission (200), got %d: %s", futureRes.Code, futureRes.Body.String())
	}

	// 3. Attempting to remove root role from the last active Root Admin -> 409 Conflict.
	stripRootRes := doJSON(http.MethodPut, "/api/v1/users/"+boot.RootUserID.String()+"/roles", map[string]any{
		"roleCodes": []string{"hotel-manager"},
		"reason":    "attempt to strip last root",
	}, rootToken)
	if stripRootRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 when stripping root role from last active root admin, got %d: %s", stripRootRes.Code, stripRootRes.Body.String())
	}

	// 4. Attempting to modify permissions of the root role -> 409 Conflict.
	stripRootPermsRes := doJSON(http.MethodPut, "/api/v1/roles/"+boot.RootRoleID.String()+"/permissions", map[string]any{
		"permissionCodes": []string{"booking.read"},
	}, rootToken)
	if stripRootPermsRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 when modifying root role permissions, got %d: %s", stripRootPermsRes.Code, stripRootPermsRes.Body.String())
	}

	// 5. Create two custom roles via Root Admin:
	//    - "delegated-assigner" with ONLY ["booking.read", "role_permission.assign", "role.create"]
	//    - "target-role" initially empty.
	createRole1 := doJSON(http.MethodPost, "/api/v1/roles", map[string]any{
		"code":        "delegated-assigner",
		"name":        "Delegated Assigner",
		"description": "Limited role assigner",
	}, rootToken)
	if createRole1.Code != http.StatusCreated {
		t.Fatalf("create delegated-assigner role failed (%d): %s", createRole1.Code, createRole1.Body.String())
	}
	var assignerRole rbacapp.Role
	_ = json.Unmarshal(createRole1.Body.Bytes(), &assignerRole)

	setAssignerPerms := doJSON(http.MethodPut, "/api/v1/roles/"+assignerRole.ID.String()+"/permissions", map[string]any{
		"permissionCodes": []string{"booking.read", "role_permission.assign", "role.read"},
	}, rootToken)
	if setAssignerPerms.Code != http.StatusOK {
		t.Fatalf("set assigner permissions failed (%d): %s", setAssignerPerms.Code, setAssignerPerms.Body.String())
	}

	createTargetRole := doJSON(http.MethodPost, "/api/v1/roles", map[string]any{
		"code": "target-custom-role",
		"name": "Target Custom Role",
	}, rootToken)
	if createTargetRole.Code != http.StatusCreated {
		t.Fatalf("create target-custom-role failed (%d): %s", createTargetRole.Code, createTargetRole.Body.String())
	}
	var targetRole rbacapp.Role
	_ = json.Unmarshal(createTargetRole.Body.Bytes(), &targetRole)

	// Create a staff user A and assign "delegated-assigner" role.
	staffA, err := identityRepo.CreateUser(ctx, domain.User{
		ID:          ids.NewUUID(),
		Kind:        domain.UserKindStaff,
		Email:       "staff.a@hotel.local",
		DisplayName: "Staff A",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create staff A: %v", err)
	}
	staffHash, _ := auth.HashPassword("StaffAPass123")
	_ = identityRepo.UpsertUserCredential(ctx, staffA.ID, staffHash, now)

	assignStaffARole := doJSON(http.MethodPut, "/api/v1/users/"+staffA.ID.String()+"/roles", map[string]any{
		"roleCodes": []string{"delegated-assigner"},
	}, rootToken)
	if assignStaffARole.Code != http.StatusOK {
		t.Fatalf("assign role to staff A failed (%d): %s", assignStaffARole.Code, assignStaffARole.Body.String())
	}

	staffAToken := loginAs("staff.a@hotel.local", "StaffAPass123")

	// 6. Authenticated staff A without permission.manage cannot create permissions -> 403 and 0 audit/DB mutations.
	var auditBefore int
	_ = pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&auditBefore)

	forbiddenCreatePerm := doJSON(http.MethodPost, "/api/v1/permissions", map[string]any{
		"code": "hacked.permission",
		"name": "Hacked",
	}, staffAToken)
	if forbiddenCreatePerm.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when creating permission without permission.manage, got %d", forbiddenCreatePerm.Code)
	}
	var auditAfter int
	_ = pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&auditAfter)
	if auditAfter != auditBefore {
		t.Fatalf("expected 0 audit mutations on 403 forbidden, got %d -> %d", auditBefore, auditAfter)
	}

	// 7. Privilege escalation check (Grantable-set):
	//    Staff A holds ["booking.read", "role_permission.assign", "role.read"] and tries to grant "payment.refund"
	//    to target-custom-role -> MUST be rejected with 403 Forbidden!
	escalateRes := doJSON(http.MethodPut, "/api/v1/roles/"+targetRole.ID.String()+"/permissions", map[string]any{
		"permissionCodes": []string{"booking.read", "payment.refund"},
	}, staffAToken)
	if escalateRes.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when granting permission outside caller's grantable set, got %d: %s", escalateRes.Code, escalateRes.Body.String())
	}

	// Granting a subset of Staff A's own permissions ("booking.read") succeeds -> 200 OK.
	validGrantRes := doJSON(http.MethodPut, "/api/v1/roles/"+targetRole.ID.String()+"/permissions", map[string]any{
		"permissionCodes": []string{"booking.read"},
	}, staffAToken)
	if validGrantRes.Code != http.StatusOK {
		t.Fatalf("expected 200 when granting permission within caller's grantable set, got %d: %s", validGrantRes.Code, validGrantRes.Body.String())
	}

	// 8. Deleting a role while Staff A is still assigned to it -> 409 Conflict.
	delAssignedRole := doJSON(http.MethodDelete, "/api/v1/roles/"+assignerRole.ID.String(), nil, rootToken)
	if delAssignedRole.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting role with assigned users, got %d", delAssignedRole.Code)
	}

	// 9. Removing Staff A's role bumps Staff A's authorization_version, so Staff A's existing JWT is immediately rejected (401).
	removeStaffARoles := doJSON(http.MethodPut, "/api/v1/users/"+staffA.ID.String()+"/roles", map[string]any{
		"roleCodes": []string{},
		"reason":    "revoke all roles from staff A",
	}, rootToken)
	if removeStaffARoles.Code != http.StatusOK {
		t.Fatalf("remove staff A roles failed (%d): %s", removeStaffARoles.Code, removeStaffARoles.Body.String())
	}

	staleStaffARes := doJSON(http.MethodGet, "/api/v1/roles", nil, staffAToken)
	if staleStaffARes.Code != http.StatusUnauthorized {
		t.Fatalf("expected Staff A's old access token to be rejected with 401 immediately after role removal, got %d", staleStaffARes.Code)
	}

	// Now that assignerRole has 0 assigned users, archiving/deleting it succeeds -> 204 No Content.
	delUnassignedRole := doJSON(http.MethodDelete, "/api/v1/roles/"+assignerRole.ID.String(), nil, rootToken)
	if delUnassignedRole.Code != http.StatusNoContent {
		t.Fatalf("expected 204 when archiving unassigned custom role, got %d: %s", delUnassignedRole.Code, delUnassignedRole.Body.String())
	}

	// Ensure unused import uuid is referenced.
	_ = uuid.Nil
}
