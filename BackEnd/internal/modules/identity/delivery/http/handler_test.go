package http_test

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
	identityhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/delivery/http"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/domain"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

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
