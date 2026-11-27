package booking_test

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
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

type staffOnlyAuthz struct{}

func (staffOnlyAuthz) Authorize(_ context.Context, p auth.Principal, _ string) error {
	if p.Kind != "staff" {
		return apperror.Forbidden("staff permission required")
	}
	return nil
}
func (staffOnlyAuthz) IsRootUser(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil }

func TestIndividualBooking_HoldToBooking_Idempotency_Snapshots_AndStaffConfirm(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, store := setupHoldRedis(t, h)
	ctx := context.Background()

	rt, _ := h.seedRoomTypeAndRooms(t, "IND_RM", 1000000, 2, "I101", "I102")
	uidA, custA := h.seedCustomerWithUser(t, "Customer A Original")
	uidB, _ := h.seedCustomerWithUser(t, "Customer B")
	actorA := auth.Principal{UserID: uidA, Kind: "customer"}
	actorB := auth.Principal{UserID: uidB, Kind: "customer"}

	// Build a router that switches principal based on a test header.
	handler := bookinghttp.NewHandler(h.bookingSvc, staffOnlyAuthz{})
	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		handler.RegisterRoutes(api, func(c *gin.Context) {
			switch c.GetHeader("X-Test-Actor") {
			case "customerA":
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), actorA))
			case "customerB":
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), actorB))
			case "staff":
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), h.adminActor))
			}
			c.Next()
		})
	})

	// 1. Customer A creates a hold.
	holdRes, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-12",
		Adults:   2,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
	})
	if err != nil {
		t.Fatalf("create hold: %v", err)
	}

	// 2. Customer B trying to use Customer A's hold must be rejected with 403.
	badOwnerBody, _ := json.Marshal(map[string]any{
		"holdId":    holdRes.HoldID,
		"holdToken": holdRes.HoldToken,
		"adults":    2,
		"representativeGuest": map[string]any{
			"fullName": "Guest B",
			"phone":    "+84900000001",
		},
	})
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(badOwnerBody))
	reqB.Header.Set("Content-Type", "application/json")
	reqB.Header.Set("Idempotency-Key", "idem-b-1")
	reqB.Header.Set("X-Test-Actor", "customerB")
	recB := httptest.NewRecorder()
	router.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong hold owner, got %d: %s", recB.Code, recB.Body.String())
	}

	// 3. Wrong holdToken must return 422.
	badTokenBody, _ := json.Marshal(map[string]any{
		"holdId":    holdRes.HoldID,
		"holdToken": "wrong-token",
		"adults":    2,
		"representativeGuest": map[string]any{
			"fullName": "Stay Guest One",
			"phone":    "+84909999999",
		},
	})
	reqBadTok := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(badTokenBody))
	reqBadTok.Header.Set("Content-Type", "application/json")
	reqBadTok.Header.Set("Idempotency-Key", "idem-bad-tok")
	reqBadTok.Header.Set("X-Test-Actor", "customerA")
	recBadTok := httptest.NewRecorder()
	router.ServeHTTP(recBadTok, reqBadTok)
	if recBadTok.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for wrong holdToken, got %d: %s", recBadTok.Code, recBadTok.Body.String())
	}

	// 4. Customer A books for a different representative guest with Idempotency-Key.
	bookBody, _ := json.Marshal(map[string]any{
		"holdId":         holdRes.HoldID,
		"holdToken":      holdRes.HoldToken,
		"adults":         2,
		"children":       0,
		"specialRequest": "High floor",
		"representativeGuest": map[string]any{
			"fullName":        "Nguyen Van Luu Tru",
			"phone":           "+84988888888",
			"email":           "guest@stay.local",
			"nationalityCode": "VN",
			"specialRequest":  "Extra pillow",
		},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(bookBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "idem-cust-a-001")
	req1.Header.Set("X-Test-Actor", "customerA")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var resp1 struct {
		Data bookingapp.Booking `json:"data"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("unmarshal booking1: %v", err)
	}
	b1 := resp1.Data
	if b1.Status != "pending" || !b1.TotalAmount.Equal(decimal.NewFromInt(2200000)) {
		t.Fatalf("unexpected booking1: status=%s total=%s", b1.Status, b1.TotalAmount)
	}

	// 5. Replay with same Idempotency-Key must return exact same booking ID (200 OK) and not create a 2nd row.
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(bookBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "idem-cust-a-001")
	req2.Header.Set("X-Test-Actor", "customerA")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on idempotent replay, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 struct {
		Data bookingapp.Booking `json:"data"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Data.ID != b1.ID {
		t.Fatalf("idempotent replay returned different booking ID: %s vs %s", resp2.Data.ID, b1.ID)
	}

	// 6. Mutate customer full_name and change active VAT rate to 8% -> GET booking must keep immutable snapshots.
	if _, err := h.pg.Pool.Exec(ctx, `UPDATE customers SET full_name = 'Customer A Renamed' WHERE id = $1`, custA); err != nil {
		t.Fatalf("update customer name: %v", err)
	}
	if _, err := h.pricingSvc.CreateVATRate(ctx, h.adminActor, pricingapp.CreateVATRateParams{
		Code:     "VAT_8",
		Name:     "VAT 8%",
		Rate:     decimal.RequireFromString("0.08"),
		IsActive: true,
	}); err != nil {
		t.Fatalf("create VAT 8%%: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/bookings/"+b1.ID.String(), nil)
	getReq.Header.Set("X-Test-Actor", "customerA")
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get booking failed: %d %s", getRec.Code, getRec.Body.String())
	}
	var getResp struct {
		Data bookingapp.Booking `json:"data"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &getResp)
	if getResp.Data.RepresentativeGuest == nil || getResp.Data.RepresentativeGuest.FullName != "Nguyen Van Luu Tru" {
		t.Fatalf("guest snapshot mutated or missing: %+v", getResp.Data.RepresentativeGuest)
	}
	if !getResp.Data.VatRateSnapshot.Equal(decimal.RequireFromString("0.1000")) || !getResp.Data.TotalAmount.Equal(decimal.NewFromInt(2200000)) {
		t.Fatalf("pricing/VAT snapshot mutated: rate=%s total=%s", getResp.Data.VatRateSnapshot, getResp.Data.TotalAmount)
	}

	// Customer B cannot read Customer A's booking (403).
	getReqB := httptest.NewRequest(http.MethodGet, "/api/v1/bookings/"+b1.ID.String(), nil)
	getReqB.Header.Set("X-Test-Actor", "customerB")
	getRecB := httptest.NewRecorder()
	router.ServeHTTP(getRecB, getReqB)
	if getRecB.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for customer B reading customer A's booking, got %d", getRecB.Code)
	}

	// 7. Customer calling confirm must get 403; staff calling confirm transitions pending -> confirmed & releases Redis key.
	custConfirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+b1.ID.String()+"/confirm", nil)
	custConfirmReq.Header.Set("X-Test-Actor", "customerA")
	custConfirmRec := httptest.NewRecorder()
	router.ServeHTTP(custConfirmRec, custConfirmReq)
	if custConfirmRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when customer tries to confirm, got %d", custConfirmRec.Code)
	}

	staffConfirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+b1.ID.String()+"/confirm", nil)
	staffConfirmReq.Header.Set("X-Test-Actor", "staff")
	staffConfirmRec := httptest.NewRecorder()
	router.ServeHTTP(staffConfirmRec, staffConfirmReq)
	if staffConfirmRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on staff confirm, got %d: %s", staffConfirmRec.Code, staffConfirmRec.Body.String())
	}
	var confResp struct {
		Data bookingapp.Booking `json:"data"`
	}
	_ = json.Unmarshal(staffConfirmRec.Body.Bytes(), &confResp)
	if confResp.Data.Status != "confirmed" || confResp.Data.ExpiresAt != nil {
		t.Fatalf("expected confirmed booking with nil expiresAt, got status=%s expiresAt=%v", confResp.Data.Status, confResp.Data.ExpiresAt)
	}

	keysAfterConfirm, _ := store.ScanHoldKeys(ctx)
	if len(keysAfterConfirm) != 0 {
		t.Fatalf("expected Redis hold key released after confirm, got %v", keysAfterConfirm)
	}

	// 8. Expired hold must return 409 Conflict when trying to book.
	hold2, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-20",
		CheckOut: "2026-12-21",
		Adults:   1,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
	})
	if err != nil {
		t.Fatalf("create hold2: %v", err)
	}
	h.clk.Advance(30 * time.Minute)
	expBody, _ := json.Marshal(map[string]any{
		"holdId":    hold2.HoldID,
		"holdToken": hold2.HoldToken,
		"adults":    1,
		"representativeGuest": map[string]any{
			"fullName": "Late Guest",
			"phone":    "+84901111111",
		},
	})
	expReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(expBody))
	expReq.Header.Set("Content-Type", "application/json")
	expReq.Header.Set("Idempotency-Key", "idem-exp-1")
	expReq.Header.Set("X-Test-Actor", "customerA")
	expRec := httptest.NewRecorder()
	router.ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for expired hold, got %d: %s", expRec.Code, expRec.Body.String())
	}
}
