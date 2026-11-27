package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
)

func TestExpiryScanner_AndRedisReconciler(t *testing.T) {
	h := setupBookingTestHarness(t)
	redisClient, store := setupHoldRedis(t, h)
	ctx := context.Background()

	rt, rooms := h.seedRoomTypeAndRooms(t, "REC_RM", 1000000, 2, "R101", "R102")
	uid, custID := h.seedCustomerWithUser(t, "Reconcile Guest")
	actor := auth.Principal{UserID: uid, Kind: "customer"}
	vatID := h.activeVATID(t)

	// 1. Create an active hold and a pending booking that will expire when clock advances.
	holdRes, err := h.bookingSvc.CreateHold(ctx, actor, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-12",
		Adults:   2,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
	})
	if err != nil {
		t.Fatalf("create hold: %v", err)
	}

	pendingBookingID := uuid.New()
	bookingExpiresAt := h.clk.Now().Add(5 * time.Minute)
	if _, err := h.pg.Pool.Exec(ctx, `
		INSERT INTO bookings (
			id, reference_code, customer_id, room_id, room_type_id,
			stay, adults, children, status,
			nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
			subtotal_amount, vat_amount, total_amount, currency, expires_at
		) VALUES (
			$1, 'BK-EXP-001', $2, $3, $4,
			daterange('2026-12-15'::date, '2026-12-17'::date, '[)'), 2, 0, 'pending',
			1000000, $5, 0.1000,
			2000000, 200000, 2200000, 'VND', $6
		)
	`, pendingBookingID, custID, rooms[1].ID, rt.ID, vatID, bookingExpiresAt); err != nil {
		t.Fatalf("insert pending booking: %v", err)
	}

	// 2. Test DB -> Redis reconciliation after Redis FLUSHALL (simulating Redis restart).
	if err := redisClient.Raw().FlushAll(ctx).Err(); err != nil {
		t.Fatalf("redis flushall: %v", err)
	}
	keysAfterFlush, _ := store.ScanHoldKeys(ctx)
	if len(keysAfterFlush) != 0 {
		t.Fatalf("expected 0 keys after FLUSHALL, got %d", len(keysAfterFlush))
	}

	// Also plant an orphan key in Redis that does not exist in PostgreSQL.
	orphanKey := "hold:room:" + uuid.NewString() + ":2026-12-25:2026-12-26"
	if ok, err := store.AcquireHoldKeys(ctx, []string{orphanKey}, "orphan-token", 600); err != nil || !ok {
		t.Fatalf("plant orphan key: ok=%v err=%v", ok, err)
	}

	recStats, err := h.bookingSvc.ReconcileRedisOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile redis: %v", err)
	}
	if recStats.RestoredKeys != 1 || recStats.OrphansDeleted != 1 {
		t.Fatalf("expected 1 restored key and 1 orphan deleted, got %+v", recStats)
	}

	keysAfterReconcile, err := store.ScanHoldKeys(ctx)
	if err != nil {
		t.Fatalf("scan keys after reconcile: %v", err)
	}
	if len(keysAfterReconcile) != 1 || keysAfterReconcile[holdRes.Holds[0].RedisKey] != holdRes.HoldToken {
		t.Fatalf("expected restored hold key with matching token and orphan gone, got %v", keysAfterReconcile)
	}

	// 3. Advance clock past both hold and booking expiration -> run DB expiry scanner.
	h.clk.Advance(20 * time.Minute)
	expStats, err := h.bookingSvc.ScanAndExpireOnce(ctx)
	if err != nil {
		t.Fatalf("scan and expire: %v", err)
	}
	if expStats.ExpiredHolds != 1 || expStats.ExpiredBookings != 1 {
		t.Fatalf("expected 1 expired hold and 1 expired booking, got %+v", expStats)
	}

	var holdStatus, bookingStatus string
	var historyCount int
	if err := h.pg.Pool.QueryRow(ctx, `SELECT status FROM reservation_holds WHERE id = $1`, holdRes.HoldID).Scan(&holdStatus); err != nil {
		t.Fatalf("query hold status: %v", err)
	}
	if err := h.pg.Pool.QueryRow(ctx, `SELECT status FROM bookings WHERE id = $1`, pendingBookingID).Scan(&bookingStatus); err != nil {
		t.Fatalf("query booking status: %v", err)
	}
	if err := h.pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM booking_status_history WHERE booking_id = $1 AND to_status = 'expired'`, pendingBookingID).Scan(&historyCount); err != nil {
		t.Fatalf("query booking history: %v", err)
	}
	if holdStatus != "expired" || bookingStatus != "expired" || historyCount != 1 {
		t.Fatalf("expected expired hold, expired booking, and 1 history row; got hold=%s booking=%s history=%d", holdStatus, bookingStatus, historyCount)
	}

	// Verify Redis hold key was deleted after expiry scan.
	keysAfterExpire, err := store.ScanHoldKeys(ctx)
	if err != nil {
		t.Fatalf("scan keys after expire: %v", err)
	}
	if len(keysAfterExpire) != 0 {
		t.Fatalf("expected 0 Redis keys after expiry scan, got %v", keysAfterExpire)
	}
}
