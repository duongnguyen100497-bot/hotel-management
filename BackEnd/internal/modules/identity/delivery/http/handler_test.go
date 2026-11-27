package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identityhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/delivery/http"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/domain"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/notification"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/smtp"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

type spySMTPSender struct {
	mu       sync.Mutex
	messages []smtp.Message
}

func (s *spySMTPSender) Send(_ context.Context, msg smtp.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

func (s *spySMTPSender) Last() (smtp.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return smtp.Message{}, false
	}
	return s.messages[len(s.messages)-1], true
}

func (s *spySMTPSender) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

func TestIdentityAuthFlow_RegisterLoginRefreshReuseAndLogout(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	now := time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	ids := id.NewUUIDGenerator()

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	tokens := auth.NewTokenManager("test-access-secret-32-bytes-minimum!!", 15*time.Minute, 7*24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens)

	var hookCalled int
	identitySvc.SetCustomerRegistrationHook(identityapp.CustomerRegistrationHookFunc(func(_ context.Context, u domain.User, phone string) error {
		hookCalled++
		if u.Kind != domain.UserKindCustomer || phone != "0901234567" {
			t.Errorf("unexpected hook user/phone: %+v / %s", u, phone)
		}
		return nil
	}))

	authMW := auth.AuthenticatorMiddleware(tokens, identitySvc)
	handler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokens,
		SecureCookie: false,
	})

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		handler.RegisterRoutes(api, authMW)
		api.GET("/protected", authMW, func(c *gin.Context) {
			p, _ := auth.PrincipalFromContext(c.Request.Context())
			platformhttp.WriteJSON(c, http.StatusOK, gin.H{"userId": p.UserID.String()})
		})
	})

	doJSON := func(method, path string, body any, bearer string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatalf("encode body: %v", err)
			}
		}
		req := httptest.NewRequest(method, path, &buf)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	findRefreshCookie := func(rec *httptest.ResponseRecorder) *http.Cookie {
		for _, ck := range rec.Result().Cookies() {
			if ck.Name == auth.RefreshCookieName {
				return ck
			}
		}
		return nil
	}

	// 1. Register customer -> 201 + access token + refresh cookie + hook invoked.
	regRes := doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":       "Alice@Example.com",
		"password":    "AlicePass123",
		"displayName": "Alice Nguyen",
		"phone":       "0901234567",
	}, "")
	if regRes.Code != http.StatusCreated {
		t.Fatalf("expected register 201, got %d: %s", regRes.Code, regRes.Body.String())
	}
	if hookCalled != 1 {
		t.Fatalf("expected OnCustomerRegistered hook to be called once, got %d", hookCalled)
	}
	regCookie := findRefreshCookie(regRes)
	if regCookie == nil || regCookie.Value == "" || !regCookie.HttpOnly {
		t.Fatalf("expected HttpOnly refresh cookie on register, got %+v", regCookie)
	}

	// 2. Duplicate registration -> 409 Conflict.
	dupRes := doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":       "alice@example.com",
		"password":    "AlicePass123",
		"displayName": "Alice Dup",
	}, "")
	if dupRes.Code != http.StatusConflict {
		t.Fatalf("expected duplicate register 409, got %d: %s", dupRes.Code, dupRes.Body.String())
	}

	// 3. Login with wrong password -> 401; non-existent email -> identical 401.
	badPassRes := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "alice@example.com",
		"password": "WrongPassword999",
	}, "")
	unknownEmailRes := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "nobody@example.com",
		"password": "WrongPassword999",
	}, "")
	if badPassRes.Code != http.StatusUnauthorized || unknownEmailRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected uniform 401, got %d and %d", badPassRes.Code, unknownEmailRes.Code)
	}

	// 4. Pending activation and disabled users cannot log in (401).
	pendingUser, err := identityRepo.CreateUser(ctx, domain.User{
		ID:          ids.NewUUID(),
		Kind:        domain.UserKindStaff,
		Email:       "pending@hotel.local",
		DisplayName: "Pending Staff",
		Status:      domain.UserStatusPendingActivation,
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create pending user: %v", err)
	}
	pwHash, _ := auth.HashPassword("StaffPass123")
	_ = identityRepo.UpsertUserCredential(ctx, pendingUser.ID, pwHash, now)

	pendingLogin := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "pending@hotel.local",
		"password": "StaffPass123",
	}, "")
	if pendingLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected pending_activation login 401, got %d", pendingLogin.Code)
	}

	// 5. Login with correct credentials -> 200.
	loginRes := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "alice@example.com",
		"password": "AlicePass123",
	}, "")
	if loginRes.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var loginPayload struct {
		AccessToken string           `json:"accessToken"`
		User        identityapp.User `json:"user"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	cookie1 := findRefreshCookie(loginRes)
	if cookie1 == nil || cookie1.Value == "" {
		t.Fatalf("expected refresh cookie on login")
	}

	// Protected endpoint succeeds with valid access token.
	protRes := doJSON(http.MethodGet, "/api/v1/protected", nil, loginPayload.AccessToken)
	if protRes.Code != http.StatusOK {
		t.Fatalf("expected protected 200, got %d: %s", protRes.Code, protRes.Body.String())
	}

	// 6. Refresh token rotation -> 200 with new cookie2.
	refRes1 := doJSON(http.MethodPost, "/api/v1/auth/refresh", nil, "", cookie1)
	if refRes1.Code != http.StatusOK {
		t.Fatalf("expected first refresh 200, got %d: %s", refRes1.Code, refRes1.Body.String())
	}
	cookie2 := findRefreshCookie(refRes1)
	if cookie2 == nil || cookie2.Value == "" || cookie2.Value == cookie1.Value {
		t.Fatalf("expected rotated refresh cookie different from cookie1")
	}

	// 7. Replaying old cookie1 triggers reuse detection -> 401 AND revokes cookie2's entire family!
	reuseRes := doJSON(http.MethodPost, "/api/v1/auth/refresh", nil, "", cookie1)
	if reuseRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected replayed cookie1 to return 401, got %d", reuseRes.Code)
	}
	familyRevokedRes := doJSON(http.MethodPost, "/api/v1/auth/refresh", nil, "", cookie2)
	if familyRevokedRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected cookie2 in same family to be revoked after reuse detection, got %d", familyRevokedRes.Code)
	}

	// 8. Fresh login, then logout -> subsequent refresh and access token both return 401.
	loginRes2 := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "alice@example.com",
		"password": "AlicePass123",
	}, "")
	var loginPayload2 struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(loginRes2.Body.Bytes(), &loginPayload2)
	cookie3 := findRefreshCookie(loginRes2)

	logoutRes := doJSON(http.MethodPost, "/api/v1/auth/logout", nil, loginPayload2.AccessToken)
	if logoutRes.Code != http.StatusNoContent {
		t.Fatalf("expected logout 204, got %d: %s", logoutRes.Code, logoutRes.Body.String())
	}

	postLogoutRefresh := doJSON(http.MethodPost, "/api/v1/auth/refresh", nil, "", cookie3)
	if postLogoutRefresh.Code != http.StatusUnauthorized {
		t.Fatalf("expected refresh after logout to return 401, got %d", postLogoutRefresh.Code)
	}
	postLogoutProtected := doJSON(http.MethodGet, "/api/v1/protected", nil, loginPayload2.AccessToken)
	if postLogoutProtected.Code != http.StatusUnauthorized {
		t.Fatalf("expected access token after logout to return 401, got %d", postLogoutProtected.Code)
	}

	// 9. Bumping authorization_version immediately invalidates an active access token.
	loginRes3 := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "alice@example.com",
		"password": "AlicePass123",
	}, "")
	var loginPayload3 struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(loginRes3.Body.Bytes(), &loginPayload3)

	if err := identitySvc.BumpAuthVersion(ctx, loginPayload.User.ID); err != nil {
		t.Fatalf("bump auth version: %v", err)
	}
	staleTokenRes := doJSON(http.MethodGet, "/api/v1/protected", nil, loginPayload3.AccessToken)
	if staleTokenRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected access token with stale authorization_version to be rejected with 401, got %d", staleTokenRes.Code)
	}
}

func TestPasswordResetFlow_OutboxEncryptedTokenAndSessionRevocation(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	now := time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	ids := id.NewUUIDGenerator()

	cipher, err := auth.NewPayloadCipher("refresh-secret-used-for-aes-gcm-outbox-encryption")
	if err != nil {
		t.Fatalf("NewPayloadCipher: %v", err)
	}

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	tokens := auth.NewTokenManager("test-access-secret-32-bytes-minimum!!", 15*time.Minute, 7*24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens).
		WithPayloadCipher(cipher)

	spySender := &spySMTPSender{}
	outboxWorker := notification.NewWorker(pg.TxRunner, auditSvc, clk, nil, notification.WorkerConfig{
		BatchSize: 10,
	})
	_ = notification.NewEmailDispatcher(notification.EmailDispatcherConfig{
		Sender:          spySender,
		Cipher:          cipher,
		FrontendBaseURL: "http://localhost:5173",
		PublicBaseURL:   "http://localhost:8080",
	}, outboxWorker)

	authMW := auth.AuthenticatorMiddleware(tokens, identitySvc)
	handler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokens,
		SecureCookie: false,
	})

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		handler.RegisterRoutes(api, authMW)
		api.GET("/protected", authMW, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	})

	doJSON := func(method, path string, body any, bearer string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// Register a customer and capture their active access token.
	regRes := doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":       "bob@example.com",
		"password":    "OldPassword123",
		"displayName": "Bob Tran",
	}, "")
	if regRes.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", regRes.Code, regRes.Body.String())
	}
	var regPayload struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(regRes.Body.Bytes(), &regPayload)

	// 1. Password reset request for non-existent email -> 202 Accepted and 0 outbox events.
	nonExistentRes := doJSON(http.MethodPost, "/api/v1/auth/password-reset-requests", map[string]any{
		"email": "ghost@example.com",
	}, "")
	if nonExistentRes.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for non-existent email, got %d", nonExistentRes.Code)
	}
	claimed, err := outboxWorker.ProcessOnce(ctx)
	if err != nil || claimed != 0 {
		t.Fatalf("expected 0 outbox events for non-existent email, got claimed=%d err=%v", claimed, err)
	}

	// 2. Password reset request for active user -> 202 Accepted + outbox event with encrypted token.
	existRes := doJSON(http.MethodPost, "/api/v1/auth/password-reset-requests", map[string]any{
		"email": "bob@example.com",
	}, "")
	if existRes.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for existing email, got %d", existRes.Code)
	}

	// Verify outbox payload in DB does NOT contain the raw token before processing.
	var rawOutboxPayload string
	if err := pg.Pool.QueryRow(ctx, `SELECT payload::text FROM outbox_events WHERE event_type = 'identity.password_reset_requested' LIMIT 1`).Scan(&rawOutboxPayload); err != nil {
		t.Fatalf("query outbox payload: %v", err)
	}
	if !strings.Contains(rawOutboxPayload, "encryptedToken") {
		t.Fatalf("expected encryptedToken in outbox payload: %s", rawOutboxPayload)
	}

	claimed, err = outboxWorker.ProcessOnce(ctx)
	if err != nil || claimed != 1 {
		t.Fatalf("expected 1 outbox event processed, got claimed=%d err=%v", claimed, err)
	}
	if spySender.Count() != 1 {
		t.Fatalf("expected 1 email sent, got %d", spySender.Count())
	}
	sentMail, _ := spySender.Last()

	// Extract raw token from reset link in email body.
	re := regexp.MustCompile(`http://localhost:5173/admin/reset-password\?token=([A-Za-z0-9_%-]+)`)
	matches := re.FindStringSubmatch(sentMail.TextBody)
	if len(matches) != 2 {
		t.Fatalf("expected reset link in email body, got:\n%s", sentMail.TextBody)
	}
	rawToken, err := url.QueryUnescape(matches[1])
	if err != nil || rawToken == "" {
		t.Fatalf("unescape token: %v", err)
	}
	if strings.Contains(rawOutboxPayload, rawToken) {
		t.Fatalf("security violation: raw token must NEVER appear in plaintext inside outbox_events.payload")
	}

	// 3. Confirm password reset with extracted token -> 200 OK.
	confirmRes := doJSON(http.MethodPost, "/api/v1/auth/password-reset-confirm", map[string]any{
		"token":    rawToken,
		"password": "NewPassword456",
	}, "")
	if confirmRes.Code != http.StatusOK {
		t.Fatalf("expected password reset confirm 200, got %d: %s", confirmRes.Code, confirmRes.Body.String())
	}

	// 4. Prior access token is immediately rejected (401) because sessions were revoked and authorization_version bumped.
	oldSessionCheck := doJSON(http.MethodGet, "/api/v1/protected", nil, regPayload.AccessToken)
	if oldSessionCheck.Code != http.StatusUnauthorized {
		t.Fatalf("expected old access token to be invalidated after password reset, got %d", oldSessionCheck.Code)
	}

	// 5. Reusing the same reset token a second time fails with 401.
	reuseConfirm := doJSON(http.MethodPost, "/api/v1/auth/password-reset-confirm", map[string]any{
		"token":    rawToken,
		"password": "AnotherPassword789",
	}, "")
	if reuseConfirm.Code != http.StatusUnauthorized {
		t.Fatalf("expected second use of reset token to return 401, got %d", reuseConfirm.Code)
	}

	// 6. Old password no longer logs in (401); new password logs in (200).
	oldLogin := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "bob@example.com",
		"password": "OldPassword123",
	}, "")
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login to fail with 401, got %d", oldLogin.Code)
	}
	newLogin := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "bob@example.com",
		"password": "NewPassword456",
	}, "")
	if newLogin.Code != http.StatusOK {
		t.Fatalf("expected new password login to succeed with 200, got %d", newLogin.Code)
	}
}
