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
	"github.com/shopspring/decimal"
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

func TestRoomStateTransitionsAndHistoryIntegration(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	baseTime := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	fixedClk := clock.NewFixed(baseTime)
	ids := id.NewUUIDGenerator()
	tokens := auth.NewTokenManager("test-access-secret-32-chars-minimum", 24*time.Hour, 48*time.Hour, fixedClk)

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), fixedClk, ids)
	identitySvc := identityapp.NewService(pg.TxRunner, identitypg.NewRepository(pg.Pool), auditSvc, auditSvc, fixedClk, ids).
		WithTokenManager(tokens)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacpg.NewRepository(pg.Pool), identitySvc, auditSvc, fixedClk, ids)

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

	propertySvc := propertyapp.NewService(pg.TxRunner, propertypg.NewRepository(pg.Pool), rbacSvc, auditSvc, fixedClk, ids)
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

	rt, err := propertySvc.CreateRoomType(ctx, rootPrincipal, propertyapp.CreateRoomTypeParams{
		Code:            "STD_QUEEN",
		Name:            "Standard Queen",
		BaseNightlyRate: decimal.RequireFromString("1200000"),
		Capacity:        2,
	})
	if err != nil {
		t.Fatalf("CreateRoomType: %v", err)
	}
	room201, err := propertySvc.CreateRoom(ctx, rootPrincipal, propertyapp.CreateRoomParams{
		Code:       "201",
		Name:       "Room 201",
		RoomTypeID: rt.ID,
		Floor:      2,
	})
	if err != nil {
		t.Fatalf("CreateRoom 201: %v", err)
	}
	_, err = propertySvc.CreateRoom(ctx, rootPrincipal, propertyapp.CreateRoomParams{
		Code:       "202",
		Name:       "Room 202",
		RoomTypeID: rt.ID,
		Floor:      2,
	})
	if err != nil {
		t.Fatalf("CreateRoom 202: %v", err)
	}

	doJSON := func(method, path string, payload any) *httptest.ResponseRecorder {
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
		req.Header.Set("Authorization", "Bearer "+rootToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// 1. Enter maintenance without reason -> 422
	recNoReason := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/maintenance", map[string]any{
		"action": "enter",
	})
	if recNoReason.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enter maintenance without reason status = %d, want 422", recNoReason.Code)
	}

	// 2. Enter maintenance at T+1h -> operational count drops from 2 to 1; exit at T+3h -> operational count back to 2.
	tEnter := baseTime.Add(1 * time.Hour)
	fixedClk.Set(tEnter)
	recEnter := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/maintenance", map[string]any{
		"action": "enter",
		"reason": "Fix bathroom pipe",
	})
	if recEnter.Code != http.StatusOK {
		t.Fatalf("enter maintenance status = %d, body = %s", recEnter.Code, recEnter.Body.String())
	}

	tExit := baseTime.Add(3 * time.Hour)
	fixedClk.Set(tExit)
	recExit := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/maintenance", map[string]any{
		"action": "exit",
		"reason": "Pipe replaced",
	})
	if recExit.Code != http.StatusOK {
		t.Fatalf("exit maintenance status = %d, body = %s", recExit.Code, recExit.Body.String())
	}

	// Verify point-in-time CountOperationalRoomsAt reconstruction across T+30m, T+2h (during maintenance), and T+4h (after exit)!
	if total, maint, op, err := propertySvc.CountOperationalRoomsAt(ctx, baseTime.Add(30*time.Minute)); err != nil || total != 2 || maint != 0 || op != 2 {
		t.Fatalf("before maintenance counts = (%d,%d,%d,%v), want (2,0,2,nil)", total, maint, op, err)
	}
	if total, maint, op, err := propertySvc.CountOperationalRoomsAt(ctx, baseTime.Add(2*time.Hour)); err != nil || total != 2 || maint != 1 || op != 1 {
		t.Fatalf("during maintenance counts = (%d,%d,%d,%v), want (2,1,1,nil)", total, maint, op, err)
	}
	if total, maint, op, err := propertySvc.CountOperationalRoomsAt(ctx, baseTime.Add(4*time.Hour)); err != nil || total != 2 || maint != 0 || op != 2 {
		t.Fatalf("after maintenance counts = (%d,%d,%d,%v), want (2,0,2,nil)", total, maint, op, err)
	}

	// Verify 2 room_state_history rows and 2 audit_events recorded so far.
	histRec := doJSON(http.MethodGet, "/api/v1/rooms/"+room201.ID.String()+"/state-history", nil)
	if histRec.Code != http.StatusOK {
		t.Fatalf("GET state-history status = %d, body = %s", histRec.Code, histRec.Body.String())
	}
	var histPage platformhttp.PageResponse[propertyapp.RoomStateHistoryEntry]
	if err := json.Unmarshal(histRec.Body.Bytes(), &histPage); err != nil {
		t.Fatalf("decode state-history: %v", err)
	}
	if len(histPage.Data) != 2 {
		t.Fatalf("state-history len = %d, want 2", len(histPage.Data))
	}
	if histPage.Data[0].FromStatus != "maintenance" || histPage.Data[0].ToStatus != "available" {
		t.Errorf("latest history entry = %+v", histPage.Data[0])
	}
	if histPage.Data[1].FromStatus != "available" || histPage.Data[1].ToStatus != "maintenance" {
		t.Errorf("older history entry = %+v", histPage.Data[1])
	}

	var auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'property.room_state_changed' AND resource_id = $1`, room201.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_events: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("room_state_changed audit count = %d, want 2", auditCount)
	}

	// 3. MarkOccupied via RoomStateService -> trying maintenance or ready on occupied room MUST return 409 and NOT alter DB!
	fixedClk.Set(baseTime.Add(5 * time.Hour))
	occRoom, err := propertySvc.MarkOccupied(ctx, room201.ID, &rootLogin.User.ID, "check-in BK-001")
	if err != nil || occRoom.OperationalStatus != "occupied" {
		t.Fatalf("MarkOccupied: room=%+v err=%v", occRoom, err)
	}

	recOccMaint := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/maintenance", map[string]any{
		"action": "enter",
		"reason": "Attempt while occupied",
	})
	if recOccMaint.Code != http.StatusConflict {
		t.Fatalf("occupied -> maintenance status = %d, want 409", recOccMaint.Code)
	}
	recOccReady := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/ready", nil)
	if recOccReady.Code != http.StatusConflict {
		t.Fatalf("occupied -> ready status = %d, want 409", recOccReady.Code)
	}

	// Confirm room is still occupied in DB!
	curRoom, err := propertySvc.GetRoomByID(ctx, room201.ID)
	if err != nil || curRoom.OperationalStatus != "occupied" {
		t.Fatalf("expected room still occupied, got %+v (err=%v)", curRoom, err)
	}

	// 4. MarkCleaning -> POST /ready transitions cleaning -> available!
	fixedClk.Set(baseTime.Add(6 * time.Hour))
	cleanRoom, err := propertySvc.MarkCleaning(ctx, room201.ID, &rootLogin.User.ID, "check-out BK-001")
	if err != nil || cleanRoom.OperationalStatus != "cleaning" {
		t.Fatalf("MarkCleaning: room=%+v err=%v", cleanRoom, err)
	}

	fixedClk.Set(baseTime.Add(7 * time.Hour))
	recReady := doJSON(http.MethodPost, "/api/v1/rooms/"+room201.ID.String()+"/ready", map[string]any{
		"reason": "Housekeeping inspected",
	})
	if recReady.Code != http.StatusOK {
		t.Fatalf("cleaning -> ready status = %d, body = %s", recReady.Code, recReady.Body.String())
	}
	afterReady, err := propertySvc.GetRoomByID(ctx, room201.ID)
	if err != nil || afterReady.OperationalStatus != "available" {
		t.Fatalf("expected room available after ready, got %+v (err=%v)", afterReady, err)
	}
}
