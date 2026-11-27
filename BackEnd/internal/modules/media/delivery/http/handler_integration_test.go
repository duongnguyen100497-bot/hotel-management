package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	mediaapp "github.com/trandinh0506/hotel-management/backend/internal/modules/media/application"
	mediahttp "github.com/trandinh0506/hotel-management/backend/internal/modules/media/delivery/http"
	mediapg "github.com/trandinh0506/hotel-management/backend/internal/modules/media/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/notification"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	propertyhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/delivery/http"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	platformminio "github.com/trandinh0506/hotel-management/backend/internal/platform/minio"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

type mediaTestHarness struct {
	pg           *testutil.PostgresTestEnv
	minio        *testutil.MinIOTestEnv
	clk          *clock.FixedClock
	router       *gin.Engine
	tokens       *auth.TokenManager
	identitySvc  *identityapp.Service
	rbacSvc      *rbacapp.Service
	propertySvc  *propertyapp.Service
	mediaSvc     *mediaapp.Service
	outboxWorker *notification.Worker
	adminUserID  uuid.UUID
	adminToken   string
}

func setupMediaHarness(t *testing.T, maxUploadBytes int64) *mediaTestHarness {
	t.Helper()
	pg := testutil.StartPostgres(t)
	minioEnv := testutil.StartMinIO(t)

	fixedNow := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedNow)
	ids := id.NewUUIDGenerator()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	outboxWorker := notification.NewWorker(pg.TxRunner, auditSvc, clk, logger, notification.WorkerConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
	})

	tokens := auth.NewTokenManager("test-access-secret-32-bytes-long!!", 15*time.Minute, 24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identitypg.NewRepository(pg.Pool), auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacpg.NewRepository(pg.Pool), identitySvc, auditSvc, clk, ids)
	propertySvc := propertyapp.NewService(pg.TxRunner, propertypg.NewRepository(pg.Pool), rbacSvc, auditSvc, clk, ids)
	mediaSvc := mediaapp.NewService(pg.TxRunner, mediapg.NewRepository(pg.Pool), minioEnv.Client, propertySvc, rbacSvc, auditSvc, auditSvc, clk, ids).
		WithUploadMaxBytes(maxUploadBytes).
		WithLogger(logger)

	propertySvc.SetMediaResolver(mediaSvc)
	outboxWorker.Register(mediaapp.EventMediaObjectDeleteRequested, mediaSvc.HandleObjectDeleteRequested)

	ctx := context.Background()
	if _, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "AdminPass!2026",
		DisplayName: "Root Admin",
	}); err != nil {
		t.Fatalf("bootstrap root admin: %v", err)
	}
	loginRes, err := identitySvc.Login(ctx, identityapp.LoginParams{
		Email:    "root@hotel.local",
		Password: "AdminPass!2026",
	})
	if err != nil {
		t.Fatalf("login root admin: %v", err)
	}

	authMiddleware := auth.AuthenticatorMiddleware(tokens, identitySvc)
	propertyHandler := propertyhttp.NewHandler(propertyhttp.HandlerConfig{
		Service:    propertySvc,
		Authorizer: rbacSvc,
		Tokens:     tokens,
		Verifier:   identitySvc,
	})
	mediaHandler := mediahttp.NewHandler(mediaSvc, rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{
		Logger:         logger,
		BodyLimitBytes: platformhttp.DefaultBodyLimitBytes,
	}, func(api *gin.RouterGroup) {
		propertyHandler.RegisterRoutes(api, authMiddleware)
		mediaHandler.RegisterRoutes(api, authMiddleware)
	})

	return &mediaTestHarness{
		pg:           pg,
		minio:        minioEnv,
		clk:          clk,
		router:       router,
		tokens:       tokens,
		identitySvc:  identitySvc,
		rbacSvc:      rbacSvc,
		propertySvc:  propertySvc,
		mediaSvc:     mediaSvc,
		outboxWorker: outboxWorker,
		adminUserID:  loginRes.User.ID,
		adminToken:   loginRes.AccessToken,
	}
}

func makeValidPNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8((x * 7) % 255), G: uint8((y * 11) % 255), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func makeValidJPEGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 140, B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func putToPresignedURL(t *testing.T, uploadURL, contentType string, body []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new presigned PUT request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute presigned PUT to MinIO: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("presigned PUT failed with status %d: %s", resp.StatusCode, string(raw))
	}
}

func (h *mediaTestHarness) doJSON(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestMediaLifecycle_PresignedUploadCompleteLinkCatalogAndOutboxCleanup(t *testing.T) {
	h := setupMediaHarness(t, 5*1024*1024)
	ctx := context.Background()

	// 1. Create a RoomType and a physical Room.
	adminPrincipal := auth.Principal{UserID: h.adminUserID, Kind: "admin"}
	rt, err := h.propertySvc.CreateRoomType(ctx, adminPrincipal, propertyapp.CreateRoomTypeParams{
		Code:            "DELUXE_OCEAN",
		Name:            "Deluxe Ocean View",
		Description:     "Ocean view suite",
		BaseNightlyRate: decimal.RequireFromString("1800000"),
		Currency:        "VND",
		Capacity:        2,
	})
	if err != nil {
		t.Fatalf("create room type: %v", err)
	}
	rm, err := h.propertySvc.CreateRoom(ctx, adminPrincipal, propertyapp.CreateRoomParams{
		Code:       "101",
		Name:       "Room 101",
		RoomTypeID: rt.ID,
		Floor:      1,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	// 2. Request presigned PUT URL via POST /api/v1/media/uploads (attempt path traversal in fileName to verify it is ignored).
	upRec := h.doJSON(t, http.MethodPost, "/api/v1/media/uploads", h.adminToken, map[string]any{
		"fileName":    "../../etc/passwd/deluxe-cover.png",
		"contentType": "image/png",
	})
	if upRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from POST /media/uploads, got %d: %s", upRec.Code, upRec.Body.String())
	}

	var upResp struct {
		Data struct {
			MediaID     uuid.UUID `json:"mediaId"`
			UploadURL   string    `json:"uploadUrl"`
			ContentType string    `json:"contentType"`
			MaxBytes    int64     `json:"maxBytes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(upRec.Body.Bytes(), &upResp); err != nil {
		t.Fatalf("unmarshal upload ticket: %v", err)
	}
	if upResp.Data.MediaID == uuid.Nil || upResp.Data.UploadURL == "" {
		t.Fatalf("expected valid mediaId and uploadUrl, got %+v", upResp.Data)
	}
	if strings.Contains(upResp.Data.UploadURL, "passwd") || strings.Contains(upResp.Data.UploadURL, "deluxe-cover") {
		t.Fatalf("object key must be random UUID, never client fileName: %s", upResp.Data.UploadURL)
	}

	// 3. Upload real PNG image bytes to Docker MinIO using the presigned PUT URL.
	pngBytes := makeValidPNGBytes(t, 120, 80)
	putToPresignedURL(t, upResp.Data.UploadURL, "image/png", pngBytes)

	// 4. Call POST /api/v1/media/{id}/complete to validate magic bytes & bitmap dimensions.
	compRec := h.doJSON(t, http.MethodPost, "/api/v1/media/"+upResp.Data.MediaID.String()+"/complete", h.adminToken, map[string]any{})
	if compRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST /media/:id/complete, got %d: %s", compRec.Code, compRec.Body.String())
	}

	var compResp struct {
		Data struct {
			ID       uuid.UUID `json:"id"`
			Status   string    `json:"status"`
			MimeType string    `json:"mimeType"`
			ByteSize int64     `json:"byteSize"`
			Width    int       `json:"width"`
			Height   int       `json:"height"`
			SHA256   string    `json:"sha256"`
			URL      string    `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(compRec.Body.Bytes(), &compResp); err != nil {
		t.Fatalf("unmarshal complete response: %v", err)
	}
	if compResp.Data.Status != "ready" || compResp.Data.Width != 120 || compResp.Data.Height != 80 || compResp.Data.MimeType != "image/png" || len(compResp.Data.SHA256) != 64 {
		t.Fatalf("unexpected completed media asset metadata: %+v", compResp.Data)
	}

	// 5. Upload a second image (JPEG) and link both to the room, then flip cover image via PATCH /api/v1/rooms/{id}/media/{mediaId}.
	upRec2 := h.doJSON(t, http.MethodPost, "/api/v1/media/uploads", h.adminToken, map[string]any{
		"fileName":    "room-second.jpg",
		"contentType": "image/jpeg",
	})
	if upRec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for second upload, got %d: %s", upRec2.Code, upRec2.Body.String())
	}
	var upResp2 struct {
		Data struct {
			MediaID   uuid.UUID `json:"mediaId"`
			UploadURL string    `json:"uploadUrl"`
		} `json:"data"`
	}
	_ = json.Unmarshal(upRec2.Body.Bytes(), &upResp2)
	jpgBytes := makeValidJPEGBytes(t, 160, 100)
	putToPresignedURL(t, upResp2.Data.UploadURL, "image/jpeg", jpgBytes)

	compRec2 := h.doJSON(t, http.MethodPost, "/api/v1/media/"+upResp2.Data.MediaID.String()+"/complete", h.adminToken, map[string]any{
		"roomId":    rm.ID,
		"sortOrder": 2,
		"isCover":   false,
	})
	if compRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 completing second upload, got %d: %s", compRec2.Code, compRec2.Body.String())
	}

	// Link first image to the room via POST /api/v1/rooms/{id}/media as initial cover.
	linkRec := h.doJSON(t, http.MethodPost, "/api/v1/rooms/"+rm.ID.String()+"/media", h.adminToken, map[string]any{
		"mediaId":   upResp.Data.MediaID,
		"sortOrder": 1,
		"isCover":   true,
	})
	if linkRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from POST /rooms/:id/media, got %d: %s", linkRec.Code, linkRec.Body.String())
	}

	// Promote second image to cover via PATCH /api/v1/rooms/{id}/media/{mediaId}.
	patchRec := h.doJSON(t, http.MethodPatch, "/api/v1/rooms/"+rm.ID.String()+"/media/"+upResp2.Data.MediaID.String(), h.adminToken, map[string]any{
		"sortOrder": 0,
		"isCover":   true,
	})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from PATCH /rooms/:id/media/:mediaId, got %d: %s", patchRec.Code, patchRec.Body.String())
	}

	// 6. Verify public catalog GET /api/v1/catalog/room-types returns the MinIO image URLs!
	catRec := h.doJSON(t, http.MethodGet, "/api/v1/catalog/room-types", "", nil)
	if catRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /catalog/room-types, got %d: %s", catRec.Code, catRec.Body.String())
	}
	var catResp struct {
		Data []struct {
			ID       uuid.UUID              `json:"id"`
			Code     string                 `json:"code"`
			CoverURL string                 `json:"coverUrl"`
			Media    []propertyapp.MediaItem `json:"media"`
		} `json:"data"`
	}
	if err := json.Unmarshal(catRec.Body.Bytes(), &catResp); err != nil {
		t.Fatalf("unmarshal catalog response: %v", err)
	}
	if len(catResp.Data) != 1 {
		t.Fatalf("expected 1 room type in catalog, got %d", len(catResp.Data))
	}
	if catResp.Data[0].CoverURL == "" || len(catResp.Data[0].Media) != 2 {
		t.Fatalf("expected catalog room type to include coverUrl and 2 media items, got %+v", catResp.Data[0])
	}
	if catResp.Data[0].Media[0].MediaID != upResp2.Data.MediaID || !catResp.Data[0].Media[0].IsCover {
		t.Fatalf("expected second media item to be promoted cover first in list, got %+v", catResp.Data[0].Media)
	}

	// 7. Delete first media asset via DELETE /api/v1/media/{id} and run outbox worker to verify object deletion from MinIO.
	var objKey1 string
	if err := h.pg.Pool.QueryRow(ctx, `SELECT object_key FROM media_assets WHERE id = $1`, upResp.Data.MediaID).Scan(&objKey1); err != nil {
		t.Fatalf("scan object_key: %v", err)
	}

	delRec := h.doJSON(t, http.MethodDelete, "/api/v1/media/"+upResp.Data.MediaID.String(), h.adminToken, nil)
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from DELETE /media/:id, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Before worker runs, DB status is deletion_pending and room_media link is removed.
	var statusAfterDelete string
	if err := h.pg.Pool.QueryRow(ctx, `SELECT status FROM media_assets WHERE id = $1`, upResp.Data.MediaID).Scan(&statusAfterDelete); err != nil {
		t.Fatalf("query status after delete: %v", err)
	}
	if statusAfterDelete != "deletion_pending" {
		t.Fatalf("expected deletion_pending before outbox worker runs, got %s", statusAfterDelete)
	}

	processed, err := h.outboxWorker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("outboxWorker.ProcessOnce: %v", err)
	}
	if processed < 1 {
		t.Fatalf("expected outbox worker to process media.object_delete_requested event, processed=%d", processed)
	}

	// Verify DB row is now 'deleted' and object is physically removed from MinIO.
	if err := h.pg.Pool.QueryRow(ctx, `SELECT status FROM media_assets WHERE id = $1`, upResp.Data.MediaID).Scan(&statusAfterDelete); err != nil {
		t.Fatalf("query status after worker: %v", err)
	}
	if statusAfterDelete != "deleted" {
		t.Fatalf("expected deleted status after outbox worker, got %s", statusAfterDelete)
	}
	if _, err := h.minio.Client.StatObject(ctx, objKey1); !errors.Is(err, platformminio.ErrObjectNotFound) {
		t.Fatalf("expected MinIO object %q to be deleted (ErrObjectNotFound), got err=%v", objKey1, err)
	}
}

func TestMediaUpload_RejectsSpoofedSVGAndOversizedAndCleansStalePending(t *testing.T) {
	// Set maxUploadBytes = 2048 bytes (2 KiB) to test oversized rejection easily.
	h := setupMediaHarness(t, 2048)
	ctx := context.Background()

	// 1. Upload a spoofed SVG file claiming to be a .png image -> Complete must return 422 and keep status != ready.
	svgTicketRec := h.doJSON(t, http.MethodPost, "/api/v1/media/uploads", h.adminToken, map[string]any{
		"fileName":    "spoofed.png",
		"contentType": "image/png",
	})
	if svgTicketRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for ticket, got %d", svgTicketRec.Code)
	}
	var svgTicket struct {
		Data struct {
			MediaID   uuid.UUID `json:"mediaId"`
			UploadURL string    `json:"uploadUrl"`
		} `json:"data"`
	}
	_ = json.Unmarshal(svgTicketRec.Body.Bytes(), &svgTicket)

	spoofedSVG := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`)
	putToPresignedURL(t, svgTicket.Data.UploadURL, "image/png", spoofedSVG)

	svgCompRec := h.doJSON(t, http.MethodPost, "/api/v1/media/"+svgTicket.Data.MediaID.String()+"/complete", h.adminToken, map[string]any{})
	if svgCompRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 rejecting spoofed SVG, got %d: %s", svgCompRec.Code, svgCompRec.Body.String())
	}

	var statusAfterSpoof string
	_ = h.pg.Pool.QueryRow(ctx, `SELECT status FROM media_assets WHERE id = $1`, svgTicket.Data.MediaID).Scan(&statusAfterSpoof)
	if statusAfterSpoof != "pending" {
		t.Fatalf("expected spoofed upload to remain pending (never ready), got %s", statusAfterSpoof)
	}

	// 2. Upload an image larger than maxUploadBytes (2048 bytes) -> Complete must return 422.
	bigTicketRec := h.doJSON(t, http.MethodPost, "/api/v1/media/uploads", h.adminToken, map[string]any{
		"fileName":    "large.jpg",
		"contentType": "image/jpeg",
	})
	var bigTicket struct {
		Data struct {
			MediaID   uuid.UUID `json:"mediaId"`
			UploadURL string    `json:"uploadUrl"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bigTicketRec.Body.Bytes(), &bigTicket)

	bigJPEG := makeValidJPEGBytes(t, 400, 400)
	if len(bigJPEG) <= 2048 {
		t.Fatalf("expected test JPEG > 2048 bytes, got %d", len(bigJPEG))
	}
	putToPresignedURL(t, bigTicket.Data.UploadURL, "image/jpeg", bigJPEG)

	bigCompRec := h.doJSON(t, http.MethodPost, "/api/v1/media/"+bigTicket.Data.MediaID.String()+"/complete", h.adminToken, map[string]any{})
	if bigCompRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 rejecting oversized image, got %d: %s", bigCompRec.Code, bigCompRec.Body.String())
	}

	// 3. Advance clock by 2 hours (> 1 hour stale threshold) and run ReconcileStalePendingUploads.
	h.clk.Advance(2 * time.Hour)
	cleaned, err := h.mediaSvc.ReconcileStalePendingUploads(ctx, 50)
	if err != nil {
		t.Fatalf("ReconcileStalePendingUploads: %v", err)
	}
	if cleaned != 2 {
		t.Fatalf("expected 2 stale pending uploads cleaned, got %d", cleaned)
	}

	_ = h.pg.Pool.QueryRow(ctx, `SELECT status FROM media_assets WHERE id = $1`, svgTicket.Data.MediaID).Scan(&statusAfterSpoof)
	if statusAfterSpoof != "deleted" {
		t.Fatalf("expected stale pending upload to be marked deleted, got %s", statusAfterSpoof)
	}

	// 4. Negative RBAC test: customer without media.manage gets 403 Forbidden.
	custRes, err := h.identitySvc.RegisterCustomer(ctx, identityapp.RegisterCustomerParams{
		Email:       "guest@hotel.local",
		Password:    "GuestPass!2026",
		DisplayName: "Guest User",
	})
	if err != nil {
		t.Fatalf("register customer: %v", err)
	}
	forbiddenRec := h.doJSON(t, http.MethodPost, "/api/v1/media/uploads", custRes.AccessToken, map[string]any{
		"fileName":    "room.png",
		"contentType": "image/png",
	})
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer without media.manage, got %d: %s", forbiddenRec.Code, forbiddenRec.Body.String())
	}
}
