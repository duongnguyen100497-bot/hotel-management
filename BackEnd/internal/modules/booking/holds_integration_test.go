package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	bookingredis "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/redis"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	platformredis "github.com/trandinh0506/hotel-management/backend/internal/platform/redis"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

type failingHoldStore struct{}

func (failingHoldStore) AcquireHoldKeys(context.Context, []string, string, int) (bool, error) {
	return false, errors.New("redis connection refused")
}
func (failingHoldStore) ReleaseHoldKeys(context.Context, []string, string) (int64, error) {
	return 0, errors.New("redis connection refused")
}
func (failingHoldStore) ScanHoldKeys(context.Context) (map[string]string, error) {
	return nil, errors.New("redis connection refused")
}

func injectPrincipalMiddleware(p auth.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

func (h *bookingTestHarness) seedCustomerWithUser(t *testing.T, name string) (userID uuid.UUID, customerID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	userID = uuid.New()
	customerID = uuid.New()
	if _, err := h.pg.Pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, kind, status)
		VALUES ($1, $2, $3, 'customer', 'active')
	`, userID, "cust-"+userID.String()[:8]+"@example.com", name); err != nil {
		t.Fatalf("seed customer user: %v", err)
	}
	if _, err := h.pg.Pool.Exec(ctx, `
		INSERT INTO customers (id, user_id, source, full_name, phone, email)
		VALUES ($1, $2, 'online', $3, $4, $5)
	`, customerID, userID, name, "+8491"+customerID.String()[:7], "cust-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	return userID, customerID
}

func setupHoldRedis(t *testing.T, h *bookingTestHarness) (*platformredis.Client, *bookingredis.HoldStore) {
	t.Helper()
	rd := testutil.StartRedis(t)
	client, err := platformredis.NewClient(rd.URL)
	if err != nil {
		t.Fatalf("init redis client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	store := bookingredis.NewHoldStore(client)
	h.bookingSvc.WithHoldStore(store)
	return client, store
}

func TestHoldConcurrency_LastRoom_ExactlyOneWins(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, store := setupHoldRedis(t, h)
	_ = store

	rt, _ := h.seedRoomTypeAndRooms(t, "LAST_RM", 1000000, 2, "L101")

	const goroutines = 10
	actors := make([]auth.Principal, goroutines)
	for i := 0; i < goroutines; i++ {
		uid, _ := h.seedCustomerWithUser(t, "Contender")
		actors[i] = auth.Principal{UserID: uid, Kind: "customer"}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, err := h.bookingSvc.CreateHold(context.Background(), actors[idx], bookingapp.CreateHoldParams{
				CheckIn:  "2026-12-10",
				CheckOut: "2026-12-12",
				Adults:   2,
				Children: 0,
				Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
			})
			results[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	wins := 0
	conflicts := 0
	for _, err := range results {
		if err == nil {
			wins++
		} else {
			conflicts++
		}
	}
	if wins != 1 || conflicts != goroutines-1 {
		t.Fatalf("expected exactly 1 win and %d conflicts, got wins=%d conflicts=%d", goroutines-1, wins, conflicts)
	}
}

func TestGroupHold_SharedTTL_AndAllOrNothing(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, store := setupHoldRedis(t, h)
	ctx := context.Background()

	rt, rooms := h.seedRoomTypeAndRooms(t, "GRP_RM", 1200000, 2, "GR101", "GR102", "GR103")
	uid1, _ := h.seedCustomerWithUser(t, "Group Leader 1")
	actor1 := auth.Principal{UserID: uid1, Kind: "customer"}

	// 1. Group hold of 3 rooms: all 3 room keys + group key must share group_hold_ttl_minutes (30m = 1800s).
	res, err := h.bookingSvc.CreateHold(ctx, actor1, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-15",
		CheckOut: "2026-12-17",
		Adults:   6,
		Children: 0,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err != nil {
		t.Fatalf("create group hold: %v", err)
	}
	if res.HoldMode != "group" || res.GroupHoldID == nil || res.TTLSeconds != 1800 {
		t.Fatalf("expected group hold with 1800s TTL, got mode=%s groupID=%v ttl=%d", res.HoldMode, res.GroupHoldID, res.TTLSeconds)
	}
	if len(res.Holds) != 3 {
		t.Fatalf("expected 3 room holds, got %d", len(res.Holds))
	}

	groupKey := "hold:group:" + res.GroupHoldID.String()
	groupTTL, err := store.KeyTTL(ctx, groupKey)
	if err != nil || groupTTL <= 1700*time.Second || groupTTL > 1800*time.Second {
		t.Fatalf("expected group key TTL ~1800s, got %v (err=%v)", groupTTL, err)
	}
	for _, hr := range res.Holds {
		ttl, err := store.KeyTTL(ctx, hr.RedisKey)
		if err != nil || ttl <= 1700*time.Second || ttl > 1800*time.Second {
			t.Fatalf("expected room key %s TTL ~1800s, got %v (err=%v)", hr.RedisKey, ttl, err)
		}
	}

	// 2. Release group hold and verify all keys are removed.
	if err := h.bookingSvc.ReleaseHold(ctx, actor1, bookingapp.ReleaseHoldParams{
		ID:        *res.GroupHoldID,
		HoldToken: res.HoldToken,
	}); err != nil {
		t.Fatalf("release group hold: %v", err)
	}
	keysAfterRelease, err := store.ScanHoldKeys(ctx)
	if err != nil {
		t.Fatalf("scan hold keys: %v", err)
	}
	if len(keysAfterRelease) != 0 {
		t.Fatalf("expected 0 Redis hold keys after release, got %v", keysAfterRelease)
	}

	// 3. All-or-nothing: pre-lock 1 of the 3 rooms directly in Redis; requesting 3 rooms must fail with 0 new keys created.
	preKey := "hold:room:" + rooms[0].ID.String() + ":2026-12-20:2026-12-22"
	ok, err := store.AcquireHoldKeys(ctx, []string{preKey}, "external-token", 600)
	if err != nil || !ok {
		t.Fatalf("pre-lock room in redis: ok=%v err=%v", ok, err)
	}
	_, err = h.bookingSvc.CreateHold(ctx, actor1, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-20",
		CheckOut: "2026-12-22",
		Adults:   6,
		Children: 0,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 3}},
	})
	if err == nil {
		t.Fatalf("expected conflict when 1 of 3 rooms is already held in Redis")
	}
	keysNow, err := store.ScanHoldKeys(ctx)
	if err != nil {
		t.Fatalf("scan keys after failed group hold: %v", err)
	}
	if len(keysNow) != 1 || keysNow[preKey] != "external-token" {
		t.Fatalf("expected only pre-existing key to remain (all-or-nothing), got %v", keysNow)
	}
}

func TestHoldDynamicTTL_AndRedisFailClosed503(t *testing.T) {
	h := setupBookingTestHarness(t)
	_, _ = setupHoldRedis(t, h)
	ctx := context.Background()

	rt, _ := h.seedRoomTypeAndRooms(t, "TTL_RM", 900000, 2, "T101", "T102")
	uid, _ := h.seedCustomerWithUser(t, "Dynamic TTL Guest")
	actor := auth.Principal{UserID: uid, Kind: "customer"}

	// Hold 1 with default individual_hold_ttl_minutes = 15 (900s).
	hold1, err := h.bookingSvc.CreateHold(ctx, actor, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-11",
		Adults:   1,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
	})
	if err != nil {
		t.Fatalf("hold1 failed: %v", err)
	}
	if hold1.TTLSeconds != 900 {
		t.Fatalf("expected hold1 TTL 900s, got %d", hold1.TTLSeconds)
	}

	// Admin updates individual_hold_ttl_minutes from 15 -> 25 (1500s).
	cfg, err := h.propertySvc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	_, err = h.propertySvc.UpdateSettings(ctx, h.adminActor, propertyapp.UpdateSettingsParams{
		Name:                     cfg.Name,
		Address:                  cfg.Address,
		Phone:                    cfg.Phone,
		Email:                    cfg.Email,
		Timezone:                 cfg.Timezone,
		Currency:                 cfg.Currency,
		CheckInTime:              cfg.CheckInTime,
		CheckOutTime:             cfg.CheckOutTime,
		IndividualHoldTTLMinutes: 25,
		GroupHoldTTLMinutes:      cfg.GroupHoldTTLMinutes,
		FreeCancelHours:          cfg.FreeCancelHours,
		ExpectedVersion:          cfg.Version,
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	// Hold 2 must receive 25 minutes (1500s); hold1's DB expires_at must remain unchanged.
	hold2, err := h.bookingSvc.CreateHold(ctx, actor, bookingapp.CreateHoldParams{
		CheckIn:  "2026-12-10",
		CheckOut: "2026-12-11",
		Adults:   1,
		Items:    []bookingapp.QuoteItemRequest{{RoomTypeID: rt.ID, RoomCount: 1}},
	})
	if err != nil {
		t.Fatalf("hold2 failed: %v", err)
	}
	if hold2.TTLSeconds != 1500 {
		t.Fatalf("expected hold2 TTL 1500s, got %d", hold2.TTLSeconds)
	}
	var dbExp1 time.Time
	if err := h.pg.Pool.QueryRow(ctx, `SELECT expires_at FROM reservation_holds WHERE id = $1`, hold1.HoldID).Scan(&dbExp1); err != nil {
		t.Fatalf("query hold1 expires_at: %v", err)
	}
	if !dbExp1.Equal(hold1.ExpiresAt) {
		t.Fatalf("hold1 expires_at changed unexpectedly: want %v got %v", hold1.ExpiresAt, dbExp1)
	}

	// Fail-closed test: swap holdStore with failingHoldStore -> HTTP POST /api/v1/holds must return 503.
	h.bookingSvc.WithHoldStore(failingHoldStore{})
	handler := bookinghttp.NewHandler(h.bookingSvc, allowAllAuthz{})
	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		handler.RegisterRoutes(api, injectPrincipalMiddleware(actor))
	})

	body, _ := json.Marshal(map[string]any{
		"checkIn":  "2026-12-14",
		"checkOut": "2026-12-15",
		"adults":   1,
		"items":    []map[string]any{{"roomTypeId": rt.ID, "roomCount": 1}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/holds", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable when Redis fails, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify 0 active holds were persisted for 2026-12-14..2026-12-15.
	var count int
	if err := h.pg.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reservation_holds
		WHERE lower(stay) = '2026-12-14'::date AND status = 'active'
	`).Scan(&count); err != nil {
		t.Fatalf("count failed holds: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 DB holds persisted on Redis 503, got %d", count)
	}
}
