package property_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	propertyhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/delivery/http"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestServiceCatalogCRUDAndDeactivationIntegration(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	clk := clock.NewSystem()
	ids := id.NewUUIDGenerator()
	tokens := auth.NewTokenManager("test-access-secret-32-chars-minimum", 15*time.Minute, 24*time.Hour, clk)

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identitySvc := identityapp.NewService(pg.TxRunner, identitypg.NewRepository(pg.Pool), auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacpg.NewRepository(pg.Pool), identitySvc, auditSvc, clk, ids)

	if _, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "StrongPass!2026",
		DisplayName: "Root Admin",
	}); err != nil {
		t.Fatalf("BootstrapRootAdmin: %v", err)
	}
	rootLogin, err := identitySvc.Login(ctx, identityapp.LoginParams{
		Email:    "root@hotel.local",
		Password: "StrongPass!2026",
	})
	if err != nil {
		t.Fatalf("Login root: %v", err)
	}
	rootToken := rootLogin.AccessToken
	rootPrincipal := auth.Principal{UserID: rootLogin.User.ID, Kind: rootLogin.User.Kind}

	// Create a Front-Desk user (has service.read, NOT service.manage)
	fdUser, err := identitySvc.CreateActiveAdminUser(ctx, identityapp.CreateActiveAdminParams{
		Email:       "fd@hotel.local",
		Password:    "StrongPass!2026",
		DisplayName: "Front Desk",
	})
	if err != nil {
		t.Fatalf("CreateActiveAdminUser: %v", err)
	}
	if _, err := rbacSvc.SetUserRoles(ctx, rootPrincipal, rbacapp.AssignUserRolesParams{
		TargetUserID: fdUser.ID,
		RoleCodes:    []string{rbacapp.RoleFrontDesk},
	}); err != nil {
		t.Fatalf("SetUserRoles: %v", err)
	}
	fdLogin, err := identitySvc.Login(ctx, identityapp.LoginParams{
		Email:    "fd@hotel.local",
		Password: "StrongPass!2026",
	})
	if err != nil {
		t.Fatalf("Login fd: %v", err)
	}
	fdToken := fdLogin.AccessToken

	propertySvc := propertyapp.NewService(pg.TxRunner, propertypg.NewRepository(pg.Pool), rbacSvc, auditSvc, clk, ids)
	authMiddleware := auth.AuthenticatorMiddleware(tokens, identitySvc)
	propHandler := propertyhttp.NewHandler(propertyhttp.HandlerConfig{
		Service:    propertySvc,
		Authorizer: rbacSvc,
		Tokens:     tokens,
		Verifier:   identitySvc,
	})
	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		propHandler.RegisterRoutes(api, authMiddleware)
	})

	doJSON := func(method, path, bearer string, payload any) *httptest.ResponseRecorder {
		var body *bytes.Reader
		if payload != nil {
			b, _ := json.Marshal(payload)
			body = bytes.NewReader(b)
		} else {
			body = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, body)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 1. Negative RBAC: Front-desk cannot create service (403), but CAN read services (200).
	{
		recDeny := doJSON(http.MethodPost, "/api/v1/services", fdToken, map[string]any{
			"code":      "LAUNDRY_SHIRT",
			"name":      "Giặt ủi áo sơ mi",
			"category":  "laundry",
			"unitPrice": "50000",
		})
		if recDeny.Code != http.StatusForbidden {
			t.Fatalf("front-desk POST /services status = %d, want 403", recDeny.Code)
		}

		recAllowRead := doJSON(http.MethodGet, "/api/v1/services", fdToken, nil)
		if recAllowRead.Code != http.StatusOK {
			t.Fatalf("front-desk GET /services status = %d, want 200", recAllowRead.Code)
		}
	}

	// 2. Create 2 services (laundry + minibar), verify duplicate code (409) & optimistic lock (409).
	var laundryItem propertyapp.ServiceCatalogItem
	{
		rec1 := doJSON(http.MethodPost, "/api/v1/services", rootToken, map[string]any{
			"code":        "LAUNDRY_SHIRT",
			"name":        "Giặt ủi áo sơ mi",
			"category":    "laundry",
			"unitPrice":   "50000",
			"description": "Giá chưa gồm VAT",
		})
		if rec1.Code != http.StatusCreated {
			t.Fatalf("create service 1 status = %d, body = %s", rec1.Code, rec1.Body.String())
		}
		var out struct {
			Data propertyapp.ServiceCatalogItem `json:"data"`
		}
		_ = json.Unmarshal(rec1.Body.Bytes(), &out)
		laundryItem = out.Data

		dup := doJSON(http.MethodPost, "/api/v1/services", rootToken, map[string]any{
			"code":      "laundry_shirt",
			"name":      "Dup",
			"category":  "laundry",
			"unitPrice": "50000",
		})
		if dup.Code != http.StatusConflict {
			t.Fatalf("duplicate service code status = %d, want 409", dup.Code)
		}

		rec2 := doJSON(http.MethodPost, "/api/v1/services", rootToken, map[string]any{
			"code":      "MINIBAR_COLA",
			"name":      "Nước ngọt lon",
			"category":  "minibar",
			"unitPrice": "35000",
		})
		if rec2.Code != http.StatusCreated {
			t.Fatalf("create service 2 status = %d", rec2.Code)
		}

		// Update laundry price to 60000 with version=1 -> version becomes 2; stale version=1 returns 409.
		patchOK := doJSON(http.MethodPatch, "/api/v1/services/"+laundryItem.ID.String(), rootToken, map[string]any{
			"unitPrice": "60000",
			"version":   1,
		})
		if patchOK.Code != http.StatusOK {
			t.Fatalf("PATCH /services status = %d, body = %s", patchOK.Code, patchOK.Body.String())
		}
		patchStale := doJSON(http.MethodPatch, "/api/v1/services/"+laundryItem.ID.String(), rootToken, map[string]any{
			"unitPrice": "70000",
			"version":   1,
		})
		if patchStale.Code != http.StatusConflict {
			t.Fatalf("stale PATCH /services status = %d, want 409", patchStale.Code)
		}
	}

	// 3. Filter by category=laundry, then deactivate laundryItem and verify GetActiveService rejects it with 409!
	{
		catRec := doJSON(http.MethodGet, "/api/v1/services?category=laundry", fdToken, nil)
		var catPage platformhttp.PageResponse[propertyapp.ServiceCatalogItem]
		_ = json.Unmarshal(catRec.Body.Bytes(), &catPage)
		if len(catPage.Data) != 1 || catPage.Data[0].ID != laundryItem.ID {
			t.Fatalf("category=laundry got %+v", catPage.Data)
		}

		// Before deactivate: GetActiveService succeeds with updated 60000 price.
		activeItem, err := propertySvc.GetActiveService(ctx, laundryItem.ID)
		if err != nil || activeItem.UnitPrice.String() != "60000" {
			t.Fatalf("GetActiveService before deactivate: item=%+v err=%v", activeItem, err)
		}

		// Deactivate
		deactRec := doJSON(http.MethodPost, "/api/v1/services/"+laundryItem.ID.String()+"/deactivate", rootToken, nil)
		if deactRec.Code != http.StatusOK {
			t.Fatalf("POST /services/deactivate status = %d, body = %s", deactRec.Code, deactRec.Body.String())
		}

		// Filter isActive=true returns only MINIBAR_COLA
		actRec := doJSON(http.MethodGet, "/api/v1/services?isActive=true", fdToken, nil)
		var actPage platformhttp.PageResponse[propertyapp.ServiceCatalogItem]
		_ = json.Unmarshal(actRec.Body.Bytes(), &actPage)
		if len(actPage.Data) != 1 || actPage.Data[0].Code != "MINIBAR_COLA" {
			t.Fatalf("isActive=true got %+v", actPage.Data)
		}

		// GetActiveService now rejects with Conflict, while GetServiceByID still returns historical record!
		if _, err := propertySvc.GetActiveService(ctx, laundryItem.ID); err == nil {
			t.Fatal("expected GetActiveService to fail on deactivated service")
		} else if appErr, ok := apperror.As(err); !ok || appErr.Code != apperror.CodeConflict {
			t.Fatalf("expected conflict error from GetActiveService, got %v", err)
		}
		if hist, err := propertySvc.GetServiceByID(ctx, laundryItem.ID); err != nil || hist.IsActive {
			t.Fatalf("expected deactivated record preserved in DB, got %+v (err=%v)", hist, err)
		}

		// Hard DELETE endpoint must NOT exist (returns 405 Method Not Allowed).
		delRec := doJSON(http.MethodDelete, "/api/v1/services/"+laundryItem.ID.String(), rootToken, nil)
		if delRec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("DELETE /services/:id status = %d, want 405", delRec.Code)
		}
	}
}
