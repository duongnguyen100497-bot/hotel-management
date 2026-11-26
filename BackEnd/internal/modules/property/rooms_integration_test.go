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
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestRoomTypesRoomsAndPublicCatalogIntegration(t *testing.T) {
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

	// Create a Front-Desk staff user (has property.read, does NOT have room.create) for negative RBAC test.
	fdUser, err := identitySvc.CreateActiveAdminUser(ctx, identityapp.CreateActiveAdminParams{
		Email:       "frontdesk@hotel.local",
		Password:    "StrongPass!2026",
		DisplayName: "Front Desk Staff",
	})
	if err != nil {
		t.Fatalf("CreateActiveAdminUser frontdesk: %v", err)
	}
	rootPrincipal := auth.Principal{UserID: rootLogin.User.ID, Kind: rootLogin.User.Kind}
	if _, err := rbacSvc.SetUserRoles(ctx, rootPrincipal, rbacapp.AssignUserRolesParams{
		TargetUserID: fdUser.ID,
		RoleCodes:    []string{rbacapp.RoleFrontDesk},
	}); err != nil {
		t.Fatalf("SetUserRoles front-desk: %v", err)
	}
	fdLogin, err := identitySvc.Login(ctx, identityapp.LoginParams{
		Email:    "frontdesk@hotel.local",
		Password: "StrongPass!2026",
	})
	if err != nil {
		t.Fatalf("Login frontdesk: %v", err)
	}
	fdToken := fdLogin.AccessToken

	// Register a customer for negative RBAC test.
	custReg, err := identitySvc.RegisterCustomer(ctx, identityapp.RegisterCustomerParams{
		Email:       "guest@example.com",
		Password:    "StrongPass!2026",
		DisplayName: "Guest User",
	})
	if err != nil {
		t.Fatalf("RegisterCustomer: %v", err)
	}
	custToken := custReg.AccessToken

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
		var bodyReader *bytes.Reader
		if payload != nil {
			b, _ := json.Marshal(payload)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, path, bodyReader)
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

	// 1. Create 3 room types: Deluxe (bookable, 1.8M, cap 2), Family Suite (bookable, 3.5M, cap 4), StaffDorm (isBookable=false).
	var deluxeRT, suiteRT, hiddenRT propertyapp.RoomType
	{
		rec := doJSON(http.MethodPost, "/api/v1/room-types", rootToken, map[string]any{
			"code":            "DELUXE_KING",
			"name":            "Deluxe King Room",
			"description":     "City view king room",
			"baseNightlyRate": "1800000",
			"capacity":        2,
			"areaSqm":         "35.5",
			"bedType":         "1 King Bed",
			"amenities":       []string{"WiFi", "MiniBar", "wifi"},
			"isBookable":      true,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create deluxe room_type status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Data propertyapp.RoomType `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		deluxeRT = out.Data
		if len(deluxeRT.Amenities) != 2 {
			t.Fatalf("expected deduplicated amenities len 2, got %v", deluxeRT.Amenities)
		}

		// Duplicate code -> 409 Conflict
		dupRec := doJSON(http.MethodPost, "/api/v1/room-types", rootToken, map[string]any{
			"code":            "deluxe_king",
			"name":            "Dup",
			"baseNightlyRate": "1800000",
			"capacity":        2,
		})
		if dupRec.Code != http.StatusConflict {
			t.Fatalf("duplicate room_type code status = %d, want 409", dupRec.Code)
		}

		recSuite := doJSON(http.MethodPost, "/api/v1/room-types", rootToken, map[string]any{
			"code":            "FAMILY_SUITE",
			"name":            "Family Ocean Suite",
			"baseNightlyRate": "3500000",
			"capacity":        4,
			"isBookable":      true,
		})
		if recSuite.Code != http.StatusCreated {
			t.Fatalf("create suite status = %d, body = %s", recSuite.Code, recSuite.Body.String())
		}
		_ = json.Unmarshal(recSuite.Body.Bytes(), &out)
		suiteRT = out.Data

		recHidden := doJSON(http.MethodPost, "/api/v1/room-types", rootToken, map[string]any{
			"code":            "INTERNAL_HOLD",
			"name":            "Non-bookable Room Type",
			"baseNightlyRate": "900000",
			"capacity":        2,
			"isBookable":      false,
		})
		if recHidden.Code != http.StatusCreated {
			t.Fatalf("create hidden room_type status = %d", recHidden.Code)
		}
		_ = json.Unmarshal(recHidden.Body.Bytes(), &out)
		hiddenRT = out.Data
	}

	// 2. Negative RBAC: Customer & Front-Desk calling POST /api/v1/rooms must get 403 Forbidden.
	for _, token := range []string{custToken, fdToken} {
		rec := doJSON(http.MethodPost, "/api/v1/rooms", token, map[string]any{
			"code":       "101",
			"name":       "Room 101",
			"roomTypeId": deluxeRT.ID,
			"floor":      1,
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("negative RBAC POST /rooms status = %d, want 403, body = %s", rec.Code, rec.Body.String())
		}
	}

	// 3. Create physical rooms, test duplicate room code (409), optimistic lock (409), and forbid operationalStatus in PATCH (422).
	var room101, room102 propertyapp.Room
	{
		rec101 := doJSON(http.MethodPost, "/api/v1/rooms", rootToken, map[string]any{
			"code":       "101",
			"name":       "Phòng 101",
			"roomTypeId": deluxeRT.ID,
			"floor":      1,
		})
		if rec101.Code != http.StatusCreated {
			t.Fatalf("create room 101 status = %d, body = %s", rec101.Code, rec101.Body.String())
		}
		var out struct {
			Data propertyapp.Room `json:"data"`
		}
		_ = json.Unmarshal(rec101.Body.Bytes(), &out)
		room101 = out.Data
		if room101.OperationalStatus != "available" || room101.Capacity != 2 {
			t.Fatalf("unexpected room101 defaults: %+v", room101)
		}

		// Duplicate room code -> 409
		dupRoom := doJSON(http.MethodPost, "/api/v1/rooms", rootToken, map[string]any{
			"code":       "101",
			"name":       "Dup 101",
			"roomTypeId": deluxeRT.ID,
			"floor":      1,
		})
		if dupRoom.Code != http.StatusConflict {
			t.Fatalf("duplicate room code status = %d, want 409", dupRoom.Code)
		}

		rec102 := doJSON(http.MethodPost, "/api/v1/rooms", rootToken, map[string]any{
			"code":       "102",
			"name":       "Phòng 102",
			"roomTypeId": deluxeRT.ID,
			"floor":      1,
		})
		if rec102.Code != http.StatusCreated {
			t.Fatalf("create room 102 status = %d", rec102.Code)
		}
		_ = json.Unmarshal(rec102.Body.Bytes(), &out)
		room102 = out.Data

		// Attempting to mutate operationalStatus via PATCH /rooms/:id returns 422!
		badPatch := doJSON(http.MethodPatch, "/api/v1/rooms/"+room101.ID.String(), rootToken, map[string]any{
			"operationalStatus": "maintenance",
			"version":           1,
		})
		if badPatch.Code != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH /rooms with operationalStatus status = %d, want 422", badPatch.Code)
		}

		// Valid PATCH /rooms/:id bumps version to 2; stale version 1 returns 409.
		okPatch := doJSON(http.MethodPatch, "/api/v1/rooms/"+room101.ID.String(), rootToken, map[string]any{
			"name":        "Phòng 101 VIP",
			"description": "Corner balcony",
			"version":     1,
		})
		if okPatch.Code != http.StatusOK {
			t.Fatalf("valid PATCH /rooms status = %d, body = %s", okPatch.Code, okPatch.Body.String())
		}
		stalePatch := doJSON(http.MethodPatch, "/api/v1/rooms/"+room101.ID.String(), rootToken, map[string]any{
			"name":    "Stale Update",
			"version": 1,
		})
		if stalePatch.Code != http.StatusConflict {
			t.Fatalf("stale PATCH /rooms status = %d, want 409", stalePatch.Code)
		}
	}

	// 4. Archive guard: occupied room cannot be archived (409), available room can be archived (200).
	{
		if _, err := pg.Pool.Exec(ctx, `UPDATE rooms SET operational_status = 'occupied' WHERE id = $1`, room102.ID); err != nil {
			t.Fatalf("seed occupied status: %v", err)
		}
		occArchive := doJSON(http.MethodPost, "/api/v1/rooms/"+room102.ID.String()+"/archive", rootToken, nil)
		if occArchive.Code != http.StatusConflict {
			t.Fatalf("archive occupied room status = %d, want 409", occArchive.Code)
		}

		if _, err := pg.Pool.Exec(ctx, `UPDATE rooms SET operational_status = 'available' WHERE id = $1`, room102.ID); err != nil {
			t.Fatalf("restore available status: %v", err)
		}
		okArchive := doJSON(http.MethodPost, "/api/v1/rooms/"+room102.ID.String()+"/archive", rootToken, nil)
		if okArchive.Code != http.StatusOK {
			t.Fatalf("archive available room status = %d, body = %s", okArchive.Code, okArchive.Body.String())
		}
	}

	// 5. Public catalog search only returns bookable & non-archived room types, supports capacity/price filters & pagination.
	{
		// Archive another temporary room type to ensure archived room types never leak into public catalog.
		recArchRT := doJSON(http.MethodPatch, "/api/v1/room-types/"+hiddenRT.ID.String(), rootToken, map[string]any{
			"archived": true,
			"version":  hiddenRT.Version,
		})
		if recArchRT.Code != http.StatusOK {
			t.Fatalf("archive room_type status = %d, body = %s", recArchRT.Code, recArchRT.Body.String())
		}

		// Public list (no auth) -> should return exactly 2 bookable room types (DELUXE_KING & FAMILY_SUITE).
		pubRec := doJSON(http.MethodGet, "/api/v1/catalog/room-types?sort=price_asc&limit=1", "", nil)
		if pubRec.Code != http.StatusOK {
			t.Fatalf("GET /catalog/room-types status = %d, body = %s", pubRec.Code, pubRec.Body.String())
		}
		var page1 platformhttp.PageResponse[propertyapp.RoomType]
		if err := json.Unmarshal(pubRec.Body.Bytes(), &page1); err != nil {
			t.Fatalf("decode catalog page1: %v", err)
		}
		if len(page1.Data) != 1 || page1.Data[0].Code != "DELUXE_KING" || page1.Page.NextCursor == nil {
			t.Fatalf("unexpected catalog page1: %+v", page1)
		}
		if page1.Data[0].OperationalRoomCount != 1 {
			t.Errorf("OperationalRoomCount = %d, want 1 (room102 archived)", page1.Data[0].OperationalRoomCount)
		}

		pubRec2 := doJSON(http.MethodGet, "/api/v1/catalog/room-types?sort=price_asc&limit=1&cursor="+*page1.Page.NextCursor, "", nil)
		if pubRec2.Code != http.StatusOK {
			t.Fatalf("GET /catalog/room-types page2 status = %d, body = %s", pubRec2.Code, pubRec2.Body.String())
		}
		var page2 platformhttp.PageResponse[propertyapp.RoomType]
		_ = json.Unmarshal(pubRec2.Body.Bytes(), &page2)
		if len(page2.Data) != 1 || page2.Data[0].ID != suiteRT.ID || page2.Page.NextCursor != nil {
			t.Fatalf("unexpected catalog page2: %+v", page2)
		}

		// Filter by capacity=3 -> only FAMILY_SUITE
		capRec := doJSON(http.MethodGet, "/api/v1/catalog/room-types?capacity=3", "", nil)
		var capPage platformhttp.PageResponse[propertyapp.RoomType]
		_ = json.Unmarshal(capRec.Body.Bytes(), &capPage)
		if len(capPage.Data) != 1 || capPage.Data[0].Code != "FAMILY_SUITE" {
			t.Fatalf("capacity=3 filter got %+v", capPage.Data)
		}

		// Public get by code: DELUXE_KING -> 200; archived/non-bookable INTERNAL_HOLD -> 404.
		codeOK := doJSON(http.MethodGet, "/api/v1/catalog/room-types/DELUXE_KING", "", nil)
		if codeOK.Code != http.StatusOK {
			t.Fatalf("GET /catalog/room-types/DELUXE_KING status = %d", codeOK.Code)
		}
		codeHidden := doJSON(http.MethodGet, "/api/v1/catalog/room-types/INTERNAL_HOLD", "", nil)
		if codeHidden.Code != http.StatusNotFound {
			t.Fatalf("GET /catalog/room-types/INTERNAL_HOLD status = %d, want 404", codeHidden.Code)
		}
	}
}
