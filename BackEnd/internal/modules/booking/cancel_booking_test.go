package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

type recordingLifecycleHook struct {
	cancelledCalls []struct {
		BookingID          uuid.UUID
		FreeCancelEligible bool
	}
}

func (r *recordingLifecycleHook) OnBookingExpired(context.Context, uuid.UUID, *uuid.UUID) error {
	return nil
}

func (r *recordingLifecycleHook) OnBookingCancelled(_ context.Context, bookingID uuid.UUID, _ *uuid.UUID, _ uuid.UUID, isFreeCancelEligible bool) error {
	r.cancelledCalls = append(r.cancelledCalls, struct {
		BookingID          uuid.UUID
		FreeCancelEligible bool
	}{BookingID: bookingID, FreeCancelEligible: isFreeCancelEligible})
	return nil
}

func TestCancelSingleChild_AndAllOrNothingBatchCancel(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, _ = setupHoldRedis(t, h)
	ctx := context.Background()

	hook := &recordingLifecycleHook{}
	h.bookingSvc.SetLifecycleHook(hook)

	rt, _ := h.seedRoomTypeAndRooms(t, "CXL_RM", 1000000, 2, "C101", "C102", "C103")
	uidA, _ := h.seedCustomerWithUser(t, "Cancel Guest A")
	actorA := auth.Principal{UserID: uidA, Kind: "customer"}

	handler := bookinghttp.NewHandler(h.bookingSvc, staffOnlyAuthz{})
	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		handler.RegisterRoutes(api, func(c *gin.Context) {
			switch c.GetHeader("X-Test-Actor") {
			case "customerA":
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), actorA))
			case "staff":
				c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), h.adminActor))
			}
			c.Next()
		})
	})

	// Create 3-room group booking and confirm all 3 rooms.
	holdRes, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-12",
		Adults:   6,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err != nil {
		t.Fatalf("create group hold: %v", err)
	}
	grp, _, err := h.bookingSvc.CreateBookingGroup(ctx, actorA, bookingapp.CreateBookingGroupParams{
		IdempotencyKey: "cxl-grp-1",
		GroupHoldID:    *holdRes.GroupHoldID,
		HoldToken:      holdRes.HoldToken,
		Adults:         2,
		RepresentativeGuest: &bookingapp.GuestSnapshot{
			FullName: "Cancel Rep",
			Phone:    "+84905555555",
		},
	})
	if err != nil {
		t.Fatalf("create booking group: %v", err)
	}
	child0, child1, child2 := grp.Bookings[0].ID, grp.Bookings[1].ID, grp.Bookings[2].ID
	if _, err := h.bookingSvc.ConfirmPaidBookings(ctx, []uuid.UUID{child0, child1, child2}, &h.adminActor.UserID, "paid"); err != nil {
		t.Fatalf("confirm group bookings: %v", err)
	}

	// 1. Cancel 1 of 3 child rooms -> sibling rooms stay confirmed, group becomes partially_cancelled.
	cxlBody := []byte(`{"reason":"guest changed plan"}`)
	cxlReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+child0.String()+"/cancel", bytes.NewReader(cxlBody))
	cxlReq.Header.Set("Content-Type", "application/json")
	cxlReq.Header.Set("X-Test-Actor", "customerA")
	cxlRec := httptest.NewRecorder()
	router.ServeHTTP(cxlRec, cxlReq)
	if cxlRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on single cancel, got %d: %s", cxlRec.Code, cxlRec.Body.String())
	}

	grpAfterSingle, err := h.bookingSvc.GetBookingGroup(ctx, actorA, grp.ID)
	if err != nil {
		t.Fatalf("get group after single cancel: %v", err)
	}
	if grpAfterSingle.Status != "partially_cancelled" {
		t.Fatalf("expected partially_cancelled, got %s", grpAfterSingle.Status)
	}
	for _, b := range grpAfterSingle.Bookings {
		if b.ID == child0 && b.Status != "cancelled" {
			t.Fatalf("expected child0 cancelled, got %s", b.Status)
		}
		if (b.ID == child1 || b.ID == child2) && b.Status != "confirmed" {
			t.Fatalf("expected sibling %s to remain confirmed, got %s", b.ID, b.Status)
		}
	}
	// Free cancel check: now=2026-12-01, checkIn=2026-12-10, free_cancel_hours=48 -> eligible = true!
	if len(hook.cancelledCalls) != 1 || !hook.cancelledCalls[0].FreeCancelEligible {
		t.Fatalf("expected lifecycle hook called once with freeCancelEligible=true, got %+v", hook.cancelledCalls)
	}

	// 2. All-or-Nothing Batch Cancel failure test:
	// Mark child2 as checked_in -> batch cancel [child1, child2] must return 409 and leave child1 confirmed with 0 new history/audit rows!
	if _, err := h.bookingSvc.MarkCheckedIn(ctx, child2, &h.adminActor.UserID, "front desk checkin"); err != nil {
		t.Fatalf("mark child2 checked_in: %v", err)
	}

	var histBefore int
	_ = h.pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM booking_status_history WHERE booking_id = $1`, child1).Scan(&histBefore)

	failBatchBody, _ := json.Marshal(map[string]any{
		"bookingIds": []uuid.UUID{child1, child2},
		"reason":     "batch cancel attempt",
	})
	failReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/batch/cancel", bytes.NewReader(failBatchBody))
	failReq.Header.Set("Content-Type", "application/json")
	failReq.Header.Set("X-Test-Actor", "customerA")
	failRec := httptest.NewRecorder()
	router.ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on batch cancel with checked_in room, got %d: %s", failRec.Code, failRec.Body.String())
	}

	// Verify child1 was NOT cancelled and 0 new history rows were written.
	b1After, err := h.bookingSvc.GetBookingByID(ctx, child1)
	if err != nil {
		t.Fatalf("get child1: %v", err)
	}
	if b1After.Status != "confirmed" {
		t.Fatalf("expected child1 to remain confirmed after failed batch cancel, got %s", b1After.Status)
	}
	var histAfter int
	_ = h.pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM booking_status_history WHERE booking_id = $1`, child1).Scan(&histAfter)
	if histAfter != histBefore {
		t.Fatalf("expected 0 new history rows on rolled back batch cancel, got %d -> %d", histBefore, histAfter)
	}

	// 3. Valid All-or-Nothing Batch Cancel of 3 fresh bookings -> all 3 cancelled atomically.
	hold2, err := h.bookingSvc.CreateHold(ctx, actorA, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-20",
		CheckOut: "2026-12-22",
		Adults:   6,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err != nil {
		t.Fatalf("create hold2: %v", err)
	}
	grp2, _, err := h.bookingSvc.CreateBookingGroup(ctx, actorA, bookingapp.CreateBookingGroupParams{
		IdempotencyKey: "cxl-grp-2",
		GroupHoldID:    *hold2.GroupHoldID,
		HoldToken:      hold2.HoldToken,
		Adults:         2,
		RepresentativeGuest: &bookingapp.GuestSnapshot{
			FullName: "Batch Rep",
			Phone:    "+84906666666",
		},
	})
	if err != nil {
		t.Fatalf("create group2: %v", err)
	}
	okBatchBody, _ := json.Marshal(map[string]any{
		"bookingIds": []uuid.UUID{grp2.Bookings[0].ID, grp2.Bookings[1].ID, grp2.Bookings[2].ID},
		"reason":     "cancel whole trip",
	})
	okReq := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/batch/cancel", bytes.NewReader(okBatchBody))
	okReq.Header.Set("Content-Type", "application/json")
	okReq.Header.Set("X-Test-Actor", "customerA")
	okRec := httptest.NewRecorder()
	router.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid batch cancel, got %d: %s", okRec.Code, okRec.Body.String())
	}
	grp2After, err := h.bookingSvc.GetBookingGroup(ctx, actorA, grp2.ID)
	if err != nil {
		t.Fatalf("get group2 after batch cancel: %v", err)
	}
	if grp2After.Status != "cancelled" {
		t.Fatalf("expected group2 status=cancelled after cancelling all 3 rooms, got %s", grp2After.Status)
	}
}
