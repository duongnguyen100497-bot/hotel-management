package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	bookingpg "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/postgres"
	peopleapp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/application"
	peoplepg "github.com/trandinh0506/hotel-management/backend/internal/modules/people/persistence/postgres"
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	pricingpg "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/persistence/postgres"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	propertyhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/delivery/http"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

type allowAllAuthz struct{}

func (allowAllAuthz) Authorize(context.Context, auth.Principal, string) error { return nil }
func (allowAllAuthz) IsRootUser(context.Context, uuid.UUID) (bool, error)     { return true, nil }

type bookingTestHarness struct {
	pg          *testutil.PostgresTestEnv
	clk         *clock.FixedClock
	propertySvc *propertyapp.Service
	pricingSvc  *pricingapp.Service
	bookingSvc  *bookingapp.Service
	router      *gin.Engine
	adminActor  auth.Principal
}

func setupBookingTestHarness(t *testing.T) *bookingTestHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pg := testutil.StartPostgres(t)
	fixedNow := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedNow)
	ids := id.NewUUIDGenerator()
	authz := allowAllAuthz{}

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	propertySvc := propertyapp.NewService(pg.TxRunner, propertypg.NewRepository(pg.Pool), authz, auditSvc, clk, ids)
	pricingSvc := pricingapp.NewService(pg.TxRunner, pricingpg.NewRepository(pg.Pool), propertySvc, authz, auditSvc, clk, ids)
	peopleSvc := peopleapp.NewService(pg.TxRunner, peoplepg.NewRepository(pg.Pool), nil, nil, auditSvc, auditSvc, clk, ids)
	bookingSvc := bookingapp.NewService(pg.TxRunner, bookingpg.NewRepository(pg.Pool), propertySvc, propertySvc, pricingSvc, peopleSvc, authz, auditSvc, auditSvc, clk, ids)
	propertySvc.SetAvailabilityReader(bookingSvc)
	peopleSvc.SetCustomerArchiveChecker(bookingSvc)

	bookingHandler := bookinghttp.NewHandler(bookingSvc, authz)
	propertyHandler := propertyhttp.NewHandler(propertyhttp.HandlerConfig{
		Service:    propertySvc,
		Authorizer: authz,
	})

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		propertyHandler.RegisterRoutes(api, nil)
		bookingHandler.RegisterRoutes(api, nil)
	})

	adminUserID := uuid.New()
	if _, err := pg.Pool.Exec(context.Background(), `
		INSERT INTO users (id, email, display_name, kind, status)
		VALUES ($1, $2, 'Admin Tester', 'staff', 'active')
	`, adminUserID, "admin-"+adminUserID.String()[:8]+"@hotel.local"); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}

	return &bookingTestHarness{
		pg:          pg,
		clk:         clk,
		propertySvc: propertySvc,
		pricingSvc:  pricingSvc,
		bookingSvc:  bookingSvc,
		router:      router,
		adminActor:  auth.Principal{UserID: adminUserID, Kind: "staff"},
	}
}

func (h *bookingTestHarness) seedCustomer(t *testing.T, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	custID := uuid.New()
	_, err := h.pg.Pool.Exec(ctx, `
		INSERT INTO customers (id, source, full_name, phone, email)
		VALUES ($1, 'walk_in', $2, $3, $4)
	`, custID, name, "+8490"+custID.String()[:7], custID.String()[:8]+"@example.com")
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	return custID
}

func (h *bookingTestHarness) seedRoomTypeAndRooms(t *testing.T, code string, rate int64, capacity int, roomCodes ...string) (propertyapp.RoomType, []propertyapp.Room) {
	t.Helper()
	ctx := context.Background()
	rt, err := h.propertySvc.CreateRoomType(ctx, h.adminActor, propertyapp.CreateRoomTypeParams{
		Code:            code,
		Name:            "Room Type " + code,
		BaseNightlyRate: decimal.NewFromInt(rate),
		Currency:        "VND",
		Capacity:        capacity,
	})
	if err != nil {
		t.Fatalf("create room type %s: %v", code, err)
	}
	rooms := make([]propertyapp.Room, 0, len(roomCodes))
	for i, rc := range roomCodes {
		rm, err := h.propertySvc.CreateRoom(ctx, h.adminActor, propertyapp.CreateRoomParams{
			Code:       rc,
			Name:       "Room " + rc,
			RoomTypeID: rt.ID,
			Floor:      i + 1,
		})
		if err != nil {
			t.Fatalf("create room %s: %v", rc, err)
		}
		rooms = append(rooms, rm)
	}
	return rt, rooms
}

func (h *bookingTestHarness) activeVATID(t *testing.T) uuid.UUID {
	t.Helper()
	vat, err := h.pricingSvc.GetActiveVATRate(context.Background())
	if err != nil || vat == nil {
		t.Fatalf("get active VAT rate: %v", err)
	}
	return vat.ID
}

func (h *bookingTestHarness) insertRawBooking(ctx context.Context, customerID, roomID, roomTypeID, vatRateID uuid.UUID, checkIn, checkOut, status string) error {
	_, err := h.pg.Pool.Exec(ctx, `
		INSERT INTO bookings (
			id, reference_code, customer_id, room_id, room_type_id,
			stay, adults, children, status,
			nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
			subtotal_amount, vat_amount, total_amount, currency
		) VALUES (
			$1, $2, $3, $4, $5,
			daterange($6::date, $7::date, '[)'), 2, 0, $8,
			1000000, $9, 0.1000,
			2000000, 200000, 2200000, 'VND'
		)
	`, uuid.New(), "BK-"+uuid.NewString()[:8], customerID, roomID, roomTypeID, checkIn, checkOut, status, vatRateID)
	return err
}

func TestGiSTExclusionConstraint_NoDoubleBooking(t *testing.T) {
	h := setupBookingTestHarness(t)
	ctx := context.Background()

	custID := h.seedCustomer(t, "Concurrent Guest")
	rt, rooms := h.seedRoomTypeAndRooms(t, "DLX_GIST", 1000000, 2, "G101")
	roomID := rooms[0].ID
	vatID := h.activeVATID(t)

	// 1. Concurrent overlapping inserts for the same room must allow exactly 1 and reject the other with 23P01.
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = h.insertRawBooking(ctx, custID, roomID, rt.ID, vatID, "2026-12-10", "2026-12-13", "pending")
		}(i)
	}
	close(start)
	wg.Wait()

	successCount := 0
	exclusionCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		} else if db.IsExclusionViolation(err) {
			exclusionCount++
		} else {
			t.Fatalf("unexpected error on concurrent insert: %v", err)
		}
	}
	if successCount != 1 || exclusionCount != 1 {
		t.Fatalf("expected 1 success and 1 exclusion violation (23P01), got success=%d exclusion=%d", successCount, exclusionCount)
	}

	// 2. Adjacent half-open stay [2026-12-13, 2026-12-15) must succeed without overlap.
	if err := h.insertRawBooking(ctx, custID, roomID, rt.ID, vatID, "2026-12-13", "2026-12-15", "confirmed"); err != nil {
		t.Fatalf("adjacent stay should succeed: %v", err)
	}

	// 3. Overlapping stay when existing booking is cancelled/expired/checked_out must succeed.
	for _, inactiveStatus := range []string{"cancelled", "expired", "checked_out"} {
		if err := h.insertRawBooking(ctx, custID, roomID, rt.ID, vatID, "2026-12-20", "2026-12-22", inactiveStatus); err != nil {
			t.Fatalf("insert %s booking failed: %v", inactiveStatus, err)
		}
	}
	if err := h.insertRawBooking(ctx, custID, roomID, rt.ID, vatID, "2026-12-20", "2026-12-22", "checked_in"); err != nil {
		t.Fatalf("active booking overlapping only inactive bookings should succeed: %v", err)
	}
}

func TestAvailabilityQuoteAndCatalogFilter(t *testing.T) {
	h := setupBookingTestHarness(t)
	ctx := context.Background()

	custID := h.seedCustomer(t, "Quote Guest")
	rt, rooms := h.seedRoomTypeAndRooms(t, "SUITE_Q", 1500000, 2, "Q101", "Q102", "Q103")
	vatID := h.activeVATID(t)

	// Put Q101 into maintenance.
	if _, err := h.propertySvc.TransitionMaintenance(ctx, h.adminActor, propertyapp.TransitionMaintenanceParams{
		RoomID: rooms[0].ID,
		Action: "enter",
		Reason: "AC repair",
	}); err != nil {
		t.Fatalf("enter maintenance: %v", err)
	}
	// Book Q102 for 2026-12-05..2026-12-08.
	if err := h.insertRawBooking(ctx, custID, rooms[1].ID, rt.ID, vatID, "2026-12-05", "2026-12-08", "confirmed"); err != nil {
		t.Fatalf("insert booking on Q102: %v", err)
	}

	// Only Q103 should be available for 2026-12-06..2026-12-09 (3 nights).
	body, _ := json.Marshal(map[string]any{
		"checkIn":  "2026-12-06",
		"checkOut": "2026-12-09",
		"items": []map[string]any{
			{"roomTypeId": rt.ID, "roomCount": 1},
		},
		"adults":   2,
		"children": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/availability/quote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data bookingapp.AvailabilityQuote `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal quote response: %v", err)
	}
	q := resp.Data
	if !q.Available {
		t.Fatalf("expected available=true")
	}
	if q.Nights != 3 {
		t.Fatalf("expected 3 nights, got %d", q.Nights)
	}
	// 3 nights * 1,500,000 = 4,500,000; VAT 10% = 450,000; Total = 4,950,000
	if !q.SubtotalAmount.Equal(decimal.NewFromInt(4500000)) {
		t.Fatalf("expected subtotal 4500000, got %s", q.SubtotalAmount)
	}
	if !q.VatAmount.Equal(decimal.NewFromInt(450000)) {
		t.Fatalf("expected vat 450000, got %s", q.VatAmount)
	}
	if !q.TotalAmount.Equal(decimal.NewFromInt(4950000)) {
		t.Fatalf("expected total 4950000, got %s", q.TotalAmount)
	}
	if len(q.Items) != 1 || q.Items[0].AvailableCount != 1 {
		t.Fatalf("expected availableCount=1 (excluding maintenance + booked), got %+v", q.Items)
	}

	// Now also book Q103 -> catalog search with checkIn/checkOut must exclude this room type.
	if err := h.insertRawBooking(ctx, custID, rooms[2].ID, rt.ID, vatID, "2026-12-06", "2026-12-09", "pending"); err != nil {
		t.Fatalf("insert booking on Q103: %v", err)
	}

	catReq := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/room-types?checkIn=2026-12-06&checkOut=2026-12-09", nil)
	catRec := httptest.NewRecorder()
	h.router.ServeHTTP(catRec, catReq)
	if catRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for catalog search, got %d: %s", catRec.Code, catRec.Body.String())
	}
	var catResp struct {
		Data []propertyapp.RoomType `json:"data"`
	}
	if err := json.Unmarshal(catRec.Body.Bytes(), &catResp); err != nil {
		t.Fatalf("unmarshal catalog: %v", err)
	}
	for _, item := range catResp.Data {
		if item.ID == rt.ID {
			t.Fatalf("expected fully booked room type %s to be excluded from date-filtered catalog", rt.Code)
		}
	}

	// Past checkIn must return 422.
	pastBody := []byte(fmt.Sprintf(`{"checkIn":"2026-11-30","checkOut":"2026-12-02","items":[{"roomTypeId":"%s","roomCount":1}],"adults":1,"children":0}`, rt.ID))
	pastReq := httptest.NewRequest(http.MethodPost, "/api/v1/availability/quote", bytes.NewReader(pastBody))
	pastReq.Header.Set("Content-Type", "application/json")
	pastRec := httptest.NewRecorder()
	h.router.ServeHTTP(pastRec, pastReq)
	if pastRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for past checkIn, got %d: %s", pastRec.Code, pastRec.Body.String())
	}
}
