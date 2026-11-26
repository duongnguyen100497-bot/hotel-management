package pricing_test

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
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	pricinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/delivery/http"
	pricingpg "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/persistence/postgres"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	propertyhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/delivery/http"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestSettingsAndVATRatesIntegration(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	clk := clock.NewSystem()
	ids := id.NewUUIDGenerator()
	tokens := auth.NewTokenManager("test-access-secret-32-chars-minimum", 15*time.Minute, 24*time.Hour, clk)

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identitySvc := identityapp.NewService(pg.TxRunner, identitypg.NewRepository(pg.Pool), auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacpg.NewRepository(pg.Pool), identitySvc, auditSvc, clk, ids)

	bootRes, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "StrongPass!2026",
		DisplayName: "Root Admin",
	})
	if err != nil {
		t.Fatalf("BootstrapRootAdmin: %v", err)
	}

	loginRes, err := identitySvc.Login(ctx, identityapp.LoginParams{
		Email:    "root@hotel.local",
		Password: "StrongPass!2026",
	})
	if err != nil {
		t.Fatalf("Login root: %v", err)
	}
	rootToken := loginRes.AccessToken

	// Also register a regular customer to verify negative RBAC (403).
	custReg, err := identitySvc.RegisterCustomer(ctx, identityapp.RegisterCustomerParams{
		Email:       "customer@example.com",
		Password:    "StrongPass!2026",
		DisplayName: "Customer One",
	})
	if err != nil {
		t.Fatalf("RegisterCustomer: %v", err)
	}
	custToken := custReg.AccessToken
	_ = bootRes

	propertySvc := propertyapp.NewService(pg.TxRunner, propertypg.NewRepository(pg.Pool), rbacSvc, auditSvc, clk, ids)
	pricingSvc := pricingapp.NewService(pg.TxRunner, pricingpg.NewRepository(pg.Pool), propertySvc, rbacSvc, auditSvc, clk, ids)

	authMiddleware := auth.AuthenticatorMiddleware(tokens, identitySvc)
	propHandler := propertyhttp.NewHandler(propertyhttp.HandlerConfig{
		Service:    propertySvc,
		Authorizer: rbacSvc,
		Tokens:     tokens,
		Verifier:   identitySvc,
	})
	priceHandler := pricinghttp.NewHandler(pricingSvc, rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		propHandler.RegisterRoutes(api, authMiddleware)
		priceHandler.RegisterRoutes(api, authMiddleware)
	})

	// 1. Public GET /api/v1/hotel/settings works unauthenticated and returns defaults (15m / 30m / 48h).
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/hotel/settings", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("public GET /hotel/settings status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode public settings: %v", err)
		}
		if got := int(resp.Data["individualHoldTtlMinutes"].(float64)); got != 15 {
			t.Errorf("default individualHoldTtlMinutes = %d, want 15", got)
		}
		if got := int(resp.Data["groupHoldTtlMinutes"].(float64)); got != 30 {
			t.Errorf("default groupHoldTtlMinutes = %d, want 30", got)
		}
		if _, hasVersion := resp.Data["version"]; hasVersion {
			t.Errorf("public settings should not expose internal version")
		}
	}

	// 2. Staff GET /api/v1/hotel/settings returns full settings including version = 1.
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/hotel/settings", nil)
		req.Header.Set("Authorization", "Bearer "+rootToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("staff GET /hotel/settings status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data propertyapp.HotelSettings `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode staff settings: %v", err)
		}
		if resp.Data.Version != 1 {
			t.Fatalf("initial version = %d, want 1", resp.Data.Version)
		}
	}

	// 3. Customer calling PUT /api/v1/hotel/settings gets 403 Forbidden.
	{
		body, _ := json.Marshal(map[string]any{
			"name":                     "Hacked Hotel",
			"address":                  "Addr",
			"phone":                    "0909",
			"email":                    "a@b.com",
			"timezone":                 "Asia/Ho_Chi_Minh",
			"currency":                 "VND",
			"checkInTime":              "14:00",
			"checkOutTime":             "12:00",
			"individualHoldTtlMinutes": 20,
			"groupHoldTtlMinutes":      45,
			"freeCancelHours":          24,
			"version":                  1,
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/hotel/settings", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+custToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("customer PUT /hotel/settings status = %d, want 403", rec.Code)
		}
	}

	// 4. Out-of-bounds TTL (0 or -5) returns 422 Validation Failed.
	for _, badTTL := range []int{0, -5, 121} {
		body, _ := json.Marshal(map[string]any{
			"name":                     "Grand Saigon Hotel",
			"address":                  "1 Dong Khoi",
			"phone":                    "+84-28-3823-0000",
			"email":                    "contact@hotel.local",
			"timezone":                 "Asia/Ho_Chi_Minh",
			"currency":                 "VND",
			"checkInTime":              "14:00",
			"checkOutTime":             "12:00",
			"individualHoldTtlMinutes": badTTL,
			"groupHoldTtlMinutes":      45,
			"freeCancelHours":          48,
			"version":                  1,
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/hotel/settings", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+rootToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("bad TTL %d status = %d, want 422, body = %s", badTTL, rec.Code, rec.Body.String())
		}
	}

	// 5. Valid PUT /api/v1/hotel/settings updates individual=20, group=45, freeCancelHours=36 and writes audit_events.
	{
		body, _ := json.Marshal(map[string]any{
			"name":                     "Grand Saigon Luxury Hotel",
			"address":                  "1 Dong Khoi, District 1",
			"phone":                    "+84-28-3823-9999",
			"email":                    "info@hotel.local",
			"timezone":                 "Asia/Ho_Chi_Minh",
			"currency":                 "VND",
			"checkInTime":              "15:00",
			"checkOutTime":             "11:30",
			"individualHoldTtlMinutes": 20,
			"groupHoldTtlMinutes":      45,
			"freeCancelHours":          36,
			"version":                  1,
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/hotel/settings", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+rootToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("valid PUT /hotel/settings status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data propertyapp.HotelSettings `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode updated settings: %v", err)
		}
		if resp.Data.IndividualHoldTTLMinutes != 20 || resp.Data.GroupHoldTTLMinutes != 45 || resp.Data.FreeCancelHours != 36 {
			t.Fatalf("unexpected updated settings: %+v", resp.Data)
		}
		if resp.Data.Version != 2 {
			t.Fatalf("updated version = %d, want 2", resp.Data.Version)
		}

		// Stale version = 1 now returns 409 Conflict!
		reqStale := httptest.NewRequest(http.MethodPut, "/api/v1/hotel/settings", bytes.NewReader(body))
		reqStale.Header.Set("Authorization", "Bearer "+rootToken)
		reqStale.Header.Set("Content-Type", "application/json")
		recStale := httptest.NewRecorder()
		router.ServeHTTP(recStale, reqStale)
		if recStale.Code != http.StatusConflict {
			t.Fatalf("stale version PUT /hotel/settings status = %d, want 409", recStale.Code)
		}

		var auditCount int
		if err := pg.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'property.hotel_settings_updated'`).Scan(&auditCount); err != nil {
			t.Fatalf("query settings audit count: %v", err)
		}
		if auditCount != 1 {
			t.Fatalf("audit count = %d, want 1", auditCount)
		}
	}

	// 6. VAT Rates: default seeded VAT_10 is active; QuoteCalculator uses 10% initially.
	var vat10ID uuid.UUID
	{
		snap10, err := pricingSvc.Calculate(ctx, decimal.RequireFromString("2000000"))
		if err != nil {
			t.Fatalf("Calculate with default VAT_10: %v", err)
		}
		if !snap10.VatAmount.Equal(decimal.RequireFromString("200000")) || !snap10.TotalAmount.Equal(decimal.RequireFromString("2200000")) {
			t.Fatalf("unexpected VAT_10 quote: %+v", snap10)
		}
		if snap10.VatRateID == nil {
			t.Fatal("expected non-nil VatRateID for seeded VAT_10")
		}
		vat10ID = *snap10.VatRateID
	}

	// 7. Create VAT_8 inactive, then activate VAT_8 via PATCH -> atomically deactivates VAT_10 in the same TX!
	var vat8 pricingapp.VATRate
	{
		createBody, _ := json.Marshal(map[string]any{
			"code":     "VAT_8",
			"name":     "Thuế GTGT ưu đãi 8%",
			"rate":     "0.08",
			"isActive": false,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/vat-rates", bytes.NewReader(createBody))
		req.Header.Set("Authorization", "Bearer "+rootToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST /vat-rates status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Data pricingapp.VATRate `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode created VAT: %v", err)
		}
		vat8 = resp.Data
		if vat8.IsActive {
			t.Fatalf("expected VAT_8 initially inactive")
		}

		// Activate VAT_8 via PATCH /api/v1/vat-rates/{id}
		patchBody, _ := json.Marshal(map[string]any{
			"isActive": true,
			"version":  vat8.Version,
		})
		patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/vat-rates/"+vat8.ID.String(), bytes.NewReader(patchBody))
		patchReq.Header.Set("Authorization", "Bearer "+rootToken)
		patchReq.Header.Set("Content-Type", "application/json")
		patchRec := httptest.NewRecorder()
		router.ServeHTTP(patchRec, patchReq)
		if patchRec.Code != http.StatusOK {
			t.Fatalf("PATCH /vat-rates/%s status = %d, body = %s", vat8.ID, patchRec.Code, patchRec.Body.String())
		}

		// Stale version PATCH returns 409 Conflict.
		staleReq := httptest.NewRequest(http.MethodPatch, "/api/v1/vat-rates/"+vat8.ID.String(), bytes.NewReader(patchBody))
		staleReq.Header.Set("Authorization", "Bearer "+rootToken)
		staleReq.Header.Set("Content-Type", "application/json")
		staleRec := httptest.NewRecorder()
		router.ServeHTTP(staleRec, staleReq)
		if staleRec.Code != http.StatusConflict {
			t.Fatalf("stale PATCH /vat-rates status = %d, want 409", staleRec.Code)
		}

		// Verify VAT_10 is now inactive and VAT_8 is the single active rate.
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/vat-rates", nil)
		listReq.Header.Set("Authorization", "Bearer "+rootToken)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK {
			t.Fatalf("GET /vat-rates status = %d", listRec.Code)
		}
		var listResp struct {
			Data []pricingapp.VATRate `json:"data"`
		}
		if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("decode list VAT: %v", err)
		}
		activeCount := 0
		for _, r := range listResp.Data {
			if r.IsActive {
				activeCount++
				if r.ID != vat8.ID {
					t.Errorf("active VAT ID = %s, want %s", r.ID, vat8.ID)
				}
			}
			if r.ID == vat10ID && r.IsActive {
				t.Errorf("expected VAT_10 (%s) to be deactivated", vat10ID)
			}
		}
		if activeCount != 1 {
			t.Fatalf("active VAT count = %d, want 1", activeCount)
		}

		// QuoteCalculator now uses 8%.
		snap8, err := pricingSvc.Calculate(ctx, decimal.RequireFromString("2000000"))
		if err != nil {
			t.Fatalf("Calculate with VAT_8: %v", err)
		}
		if !snap8.VatAmount.Equal(decimal.RequireFromString("160000")) || !snap8.TotalAmount.Equal(decimal.RequireFromString("2160000")) {
			t.Fatalf("unexpected VAT_8 quote: %+v", snap8)
		}
	}

	// 8. Deactivate VAT_8 via POST /api/v1/vat-rates/{id}/deactivate -> 0% active VAT, row still preserved.
	{
		deactReq := httptest.NewRequest(http.MethodPost, "/api/v1/vat-rates/"+vat8.ID.String()+"/deactivate", nil)
		deactReq.Header.Set("Authorization", "Bearer "+rootToken)
		deactRec := httptest.NewRecorder()
		router.ServeHTTP(deactRec, deactReq)
		if deactRec.Code != http.StatusOK {
			t.Fatalf("POST /vat-rates/deactivate status = %d, body = %s", deactRec.Code, deactRec.Body.String())
		}

		snapZero, err := pricingSvc.Calculate(ctx, decimal.RequireFromString("2000000"))
		if err != nil {
			t.Fatalf("Calculate after deactivate: %v", err)
		}
		if !snapZero.VatAmount.IsZero() || !snapZero.TotalAmount.Equal(decimal.RequireFromString("2000000")) {
			t.Fatalf("expected 0 VAT after deactivating active rate, got %+v", snapZero)
		}
	}
}
