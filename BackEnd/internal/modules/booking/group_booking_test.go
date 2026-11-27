package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

func TestGroupBooking_AtomicCreation_Idempotency_Rollback_AndDerivedStatus(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, _ = setupHoldRedis(t, h)
	ctx := context.Background()

	rt, rooms := h.seedRoomTypeAndRooms(t, "GRP_BK", 1000000, 2, "GB101", "GB102", "GB103")
	uidA, custA := h.seedCustomerWithUser(t, "Group Customer A")
	uidB, _ := h.seedCustomerWithUser(t, "Group Customer B")
	actorA := auth.Principal{UserID: uidA, Kind: "customer"}
	actorB := auth.Principal{UserID: uidB, Kind: "customer"}

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

	// 1. Create a 3-room group hold.
	holdRes, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-12",
		Adults:   6,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err != nil {
		t.Fatalf("create group hold: %v", err)
	}

	// 2. Convert group hold -> 1 booking_group + 3 child bookings in 1 tx.
	grpBody, _ := json.Marshal(map[string]any{
		"groupHoldId": holdRes.GroupHoldID,
		"holdToken":   holdRes.HoldToken,
		"adults":      2,
		"representativeGuest": map[string]any{
			"fullName": "Group Representative",
			"phone":    "+84901234567",
		},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/booking-groups", bytes.NewReader(grpBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "grp-idem-001")
	req1.Header.Set("X-Test-Actor", "customerA")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 for group booking, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var resp1 struct {
		Data bookingapp.BookingGroup `json:"data"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("unmarshal group response: %v", err)
	}
	grp := resp1.Data
	if grp.Status != "pending" || len(grp.Bookings) != 3 {
		t.Fatalf("expected pending group with 3 bookings, got status=%s count=%d", grp.Status, len(grp.Bookings))
	}
	// 3 rooms * 2 nights * 1,000,000 * 1.10 VAT = 6,600,000
	if !grp.TotalAmount.Equal(decimal.NewFromInt(6600000)) {
		t.Fatalf("expected group total 6600000, got %s", grp.TotalAmount)
	}

	// 3. Idempotent replay returns exact same group (200 OK).
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/booking-groups", bytes.NewReader(grpBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "grp-idem-001")
	req2.Header.Set("X-Test-Actor", "customerA")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on group idempotent replay, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 struct {
		Data bookingapp.BookingGroup `json:"data"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Data.ID != grp.ID {
		t.Fatalf("idempotent replay returned different group ID: %s vs %s", resp2.Data.ID, grp.ID)
	}

	// 4. Confirm 1/3 child room -> group status becomes partially_confirmed; confirm remaining 2/3 -> confirmed.
	if _, err := h.bookingSvc.ConfirmBooking(ctx, h.adminActor, grp.Bookings[0].ID, "partial confirm"); err != nil {
		t.Fatalf("confirm child 0: %v", err)
	}
	after1, err := h.bookingSvc.GetBookingGroup(ctx, actorA, grp.ID)
	if err != nil {
		t.Fatalf("get group after 1 confirm: %v", err)
	}
	if after1.Status != "partially_confirmed" {
		t.Fatalf("expected partially_confirmed after 1/3 confirmed, got %s", after1.Status)
	}

	if _, err := h.bookingSvc.ConfirmBooking(ctx, h.adminActor, grp.Bookings[1].ID, "confirm 2"); err != nil {
		t.Fatalf("confirm child 1: %v", err)
	}
	if _, err := h.bookingSvc.ConfirmBooking(ctx, h.adminActor, grp.Bookings[2].ID, "confirm 3"); err != nil {
		t.Fatalf("confirm child 2: %v", err)
	}
	afterAll, err := h.bookingSvc.GetBookingGroup(ctx, actorA, grp.ID)
	if err != nil {
		t.Fatalf("get group after all confirmed: %v", err)
	}
	if afterAll.Status != "confirmed" {
		t.Fatalf("expected confirmed after 3/3 confirmed, got %s", afterAll.Status)
	}

	// Ownership guard: Customer B cannot view Customer A's booking group.
	getB := httptest.NewRequest(http.MethodGet, "/api/v1/booking-groups/"+grp.ID.String(), nil)
	getB.Header.Set("X-Test-Actor", "customerB")
	recB := httptest.NewRecorder()
	router.ServeHTTP(recB, getB)
	if recB.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for customer B viewing customer A's group, got %d", recB.Code)
	}

	// 5. Atomic rollback test: create another 3-room group hold, then sneak a conflicting DB booking onto room 3 before converting.
	hold2, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-20",
		CheckOut: "2026-12-22",
		Adults:   6,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err != nil {
		t.Fatalf("create hold2: %v", err)
	}
	vatID := h.activeVATID(t)
	if err := h.insertRawBooking(ctx, custA, rooms[2].ID, rt.ID, vatID, "2026-12-20", "2026-12-22", "confirmed"); err != nil {
		t.Fatalf("plant conflicting booking on room 3: %v", err)
	}

	grpBody2, _ := json.Marshal(map[string]any{
		"groupHoldId": hold2.GroupHoldID,
		"holdToken":   hold2.HoldToken,
		"adults":      2,
		"representativeGuest": map[string]any{
			"fullName": "Rollback Test",
			"phone":    "+84901234568",
		},
	})
	reqFail := httptest.NewRequest(http.MethodPost, "/api/v1/booking-groups", bytes.NewReader(grpBody2))
	reqFail.Header.Set("Content-Type", "application/json")
	reqFail.Header.Set("Idempotency-Key", "grp-idem-fail")
	reqFail.Header.Set("X-Test-Actor", "customerA")
	recFail := httptest.NewRecorder()
	router.ServeHTTP(recFail, reqFail)
	if recFail.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when 1 of 3 rooms overlaps in DB, got %d: %s", recFail.Code, recFail.Body.String())
	}

	var grpCount, childCount int
	_ = h.pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM booking_groups WHERE id = $1`, *hold2.GroupHoldID).Scan(&grpCount)
	_ = h.pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM bookings WHERE booking_group_id = $1`, *hold2.GroupHoldID).Scan(&childCount)
	if grpCount != 0 || childCount != 0 {
		t.Fatalf("expected full rollback (0 groups, 0 child bookings), got groups=%d children=%d", grpCount, childCount)
	}
}
