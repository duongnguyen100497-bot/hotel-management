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
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/notification"
	peopleapp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/application"
	peoplehttp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/delivery/http"
	peoplepg "github.com/trandinh0506/hotel-management/backend/internal/modules/people/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
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

func TestEmployeeCRUD_AtomicEmailActivationAndDeactivationFlow(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	now := time.Date(2026, 11, 25, 17, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	ids := id.NewUUIDGenerator()

	cipher, err := auth.NewPayloadCipher("refresh-secret-for-aes-gcm-outbox-encryption")
	if err != nil {
		t.Fatalf("NewPayloadCipher: %v", err)
	}

	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	tokens := auth.NewTokenManager("test-access-secret-32-bytes-minimum!!", 15*time.Minute, 7*24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens).
		WithPayloadCipher(cipher)
	rbacRepo := rbacpg.NewRepository(pg.Pool)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacRepo, identitySvc, auditSvc, clk, ids)
	peopleRepo := peoplepg.NewRepository(pg.Pool)
	peopleSvc := peopleapp.NewService(pg.TxRunner, peopleRepo, identitySvc, rbacSvc, auditSvc, auditSvc, clk, ids)

	if _, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "RootPass123!",
		DisplayName: "Root Admin",
	}); err != nil {
		t.Fatalf("bootstrap root admin: %v", err)
	}

	spySender := &spySMTPSender{}
	outboxWorker := notification.NewWorker(pg.TxRunner, auditSvc, clk, nil, notification.WorkerConfig{
		BatchSize: 10,
	})
	_ = notification.NewEmailDispatcher(notification.EmailDispatcherConfig{
		Sender:          spySender,
		Cipher:          cipher,
		FrontendBaseURL: "http://localhost:5173",
	}, outboxWorker)

	authMW := auth.AuthenticatorMiddleware(tokens, identitySvc)
	idHandler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokens,
		SecureCookie: false,
	})
	peopleHandler := peoplehttp.NewHandler(peopleSvc, rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		idHandler.RegisterRoutes(api, authMW)
		peopleHandler.RegisterRoutes(api, authMW)
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

	loginAs := func(email, password string) (int, string) {
		res := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    email,
			"password": password,
		}, "")
		var payload struct {
			AccessToken string `json:"accessToken"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &payload)
		return res.Code, payload.AccessToken
	}

	_, rootToken := loginAs("root@hotel.local", "RootPass123!")

	// 1. Admin creates employee (NO password field accepted!) -> 201 Created in pending_activation status.
	createRes := doJSON(http.MethodPost, "/api/v1/admin/employees", map[string]any{
		"employeeCode":  "EMP-001",
		"fullName":      "Le Thi Reception",
		"phone":         "0912345678",
		"email":         "reception@hotel.local",
		"positionTitle": "Front Desk Receptionist",
		"roleCodes":     []string{"front-desk"},
	}, rootToken)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create employee failed (%d): %s", createRes.Code, createRes.Body.String())
	}
	var emp peopleapp.Employee
	if err := json.Unmarshal(createRes.Body.Bytes(), &emp); err != nil {
		t.Fatalf("decode employee: %v", err)
	}
	if emp.AccountStatus != identityapp.UserStatusPendingActivation {
		t.Fatalf("expected accountStatus=pending_activation, got %s", emp.AccountStatus)
	}

	// Verify outbox event payload has encryptedToken and never raw token.
	var rawOutbox string
	if err := pg.Pool.QueryRow(ctx, `SELECT payload::text FROM outbox_events WHERE event_type = 'identity.employee_activation_requested' ORDER BY occurred_at DESC LIMIT 1`).Scan(&rawOutbox); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	if !strings.Contains(rawOutbox, "encryptedToken") {
		t.Fatalf("expected encryptedToken in outbox payload: %s", rawOutbox)
	}

	// 2. Outbox processes first activation email.
	if claimed, err := outboxWorker.ProcessOnce(ctx); err != nil || claimed != 1 {
		t.Fatalf("process outbox: claimed=%d err=%v", claimed, err)
	}
	firstMail, ok := spySender.Last()
	if !ok {
		t.Fatalf("expected first activation email")
	}
	re := regexp.MustCompile(`http://localhost:5173/admin/activate\?token=([A-Za-z0-9_%-]+)`)
	m1 := re.FindStringSubmatch(firstMail.TextBody)
	if len(m1) != 2 {
		t.Fatalf("expected activation link in first email:\n%s", firstMail.TextBody)
	}
	firstToken, _ := url.QueryUnescape(m1[1])
	if strings.Contains(rawOutbox, firstToken) {
		t.Fatalf("security violation: raw activation token must never appear in outbox_events.payload")
	}

	// 3. Resend activation invalidates firstToken and issues secondToken.
	resendRes := doJSON(http.MethodPost, "/api/v1/admin/employees/"+emp.ID.String()+"/resend-activation", nil, rootToken)
	if resendRes.Code != http.StatusAccepted {
		t.Fatalf("resend activation failed (%d): %s", resendRes.Code, resendRes.Body.String())
	}
	if claimed, err := outboxWorker.ProcessOnce(ctx); err != nil || claimed != 1 {
		t.Fatalf("process second outbox: claimed=%d err=%v", claimed, err)
	}
	secondMail, _ := spySender.Last()
	m2 := re.FindStringSubmatch(secondMail.TextBody)
	secondToken, _ := url.QueryUnescape(m2[1])

	// Trying to activate with the invalidated firstToken -> 401.
	oldTokenAct := doJSON(http.MethodPost, "/api/v1/auth/activate", map[string]any{
		"token":    firstToken,
		"password": "ReceptionPass123",
	}, "")
	if oldTokenAct.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalidated first activation token to return 401, got %d", oldTokenAct.Code)
	}

	// 4. Employee activates account with secondToken and chooses password -> 200 OK + session issued.
	actRes := doJSON(http.MethodPost, "/api/v1/auth/activate", map[string]any{
		"token":    secondToken,
		"password": "ReceptionPass123",
	}, "")
	if actRes.Code != http.StatusOK {
		t.Fatalf("activate account failed (%d): %s", actRes.Code, actRes.Body.String())
	}

	// Reusing secondToken again fails with 401.
	reuseAct := doJSON(http.MethodPost, "/api/v1/auth/activate", map[string]any{
		"token":    secondToken,
		"password": "ReceptionPass123",
	}, "")
	if reuseAct.Code != http.StatusUnauthorized {
		t.Fatalf("expected second use of activation token to return 401, got %d", reuseAct.Code)
	}

	// 5. Employee logs in with the newly set password -> 200 OK.
	code, staffToken := loginAs("reception@hotel.local", "ReceptionPass123")
	if code != http.StatusOK || staffToken == "" {
		t.Fatalf("expected activated staff login 200, got %d", code)
	}

	// 6. Update employee with optimistic locking (version=1 -> 200; stale version=1 again -> 409).
	updRes := doJSON(http.MethodPatch, "/api/v1/admin/employees/"+emp.ID.String(), map[string]any{
		"fullName":      "Le Thi Senior Reception",
		"phone":         "0988888888",
		"positionTitle": "Senior Front Desk",
		"version":       1,
	}, rootToken)
	if updRes.Code != http.StatusOK {
		t.Fatalf("update employee failed (%d): %s", updRes.Code, updRes.Body.String())
	}
	staleUpdRes := doJSON(http.MethodPatch, "/api/v1/admin/employees/"+emp.ID.String(), map[string]any{
		"fullName": "Stale Update",
		"version":  1,
	}, rootToken)
	if staleUpdRes.Code != http.StatusConflict {
		t.Fatalf("expected 409 version_conflict on stale version, got %d: %s", staleUpdRes.Code, staleUpdRes.Body.String())
	}

	// 7. Deactivate employee -> locks user account and immediately revokes active sessions.
	deactRes := doJSON(http.MethodPost, "/api/v1/admin/employees/"+emp.ID.String()+"/deactivate", map[string]any{
		"reason": "end of contract",
	}, rootToken)
	if deactRes.Code != http.StatusOK {
		t.Fatalf("deactivate employee failed (%d): %s", deactRes.Code, deactRes.Body.String())
	}

	// Staff member can no longer log in (401).
	postDeactLoginCode, _ := loginAs("reception@hotel.local", "ReceptionPass123")
	if postDeactLoginCode != http.StatusUnauthorized {
		t.Fatalf("expected deactivated employee login to fail with 401, got %d", postDeactLoginCode)
	}
}

func TestCustomerManagement_WalkInAndSelfServiceMeOwnership(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	now := time.Date(2026, 11, 25, 18, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(now)
	ids := id.NewUUIDGenerator()

	cipher, _ := auth.NewPayloadCipher("refresh-secret-for-aes-gcm-outbox-encryption")
	auditSvc := auditapp.NewService(auditpg.NewRepository(pg.Pool), clk, ids)
	identityRepo := identitypg.NewRepository(pg.Pool)
	tokens := auth.NewTokenManager("test-access-secret-32-bytes-minimum!!", 15*time.Minute, 7*24*time.Hour, clk)
	identitySvc := identityapp.NewService(pg.TxRunner, identityRepo, auditSvc, auditSvc, clk, ids).
		WithTokenManager(tokens).
		WithPayloadCipher(cipher)
	rbacRepo := rbacpg.NewRepository(pg.Pool)
	rbacSvc := rbacapp.NewService(pg.TxRunner, rbacRepo, identitySvc, auditSvc, clk, ids)
	peopleRepo := peoplepg.NewRepository(pg.Pool)
	peopleSvc := peopleapp.NewService(pg.TxRunner, peopleRepo, identitySvc, rbacSvc, auditSvc, auditSvc, clk, ids)

	// Wire customer registration hook so POST /api/v1/auth/register creates both users and customers atomically.
	identitySvc.SetCustomerRegistrationHook(peopleSvc)

	if _, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       "root@hotel.local",
		Password:    "RootPass123!",
		DisplayName: "Root Admin",
	}); err != nil {
		t.Fatalf("bootstrap root admin: %v", err)
	}

	authMW := auth.AuthenticatorMiddleware(tokens, identitySvc)
	idHandler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokens,
		SecureCookie: false,
	})
	peopleHandler := peoplehttp.NewHandler(peopleSvc, rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{}, func(api *gin.RouterGroup) {
		idHandler.RegisterRoutes(api, authMW)
		peopleHandler.RegisterRoutes(api, authMW)
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

	loginAs := func(email, password string) string {
		res := doJSON(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    email,
			"password": password,
		}, "")
		if res.Code != http.StatusOK {
			t.Fatalf("login %s failed (%d): %s", email, res.Code, res.Body.String())
		}
		var payload struct {
			AccessToken string `json:"accessToken"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &payload)
		return payload.AccessToken
	}

	rootToken := loginAs("root@hotel.local", "RootPass123!")

	// 1. Customer A registers -> creates both users and customers (source='online').
	regARes := doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":       "customer.a@example.com",
		"password":    "CustomerAPass123",
		"displayName": "Customer A",
		"phone":       "0901111111",
	}, "")
	if regARes.Code != http.StatusCreated {
		t.Fatalf("register customer A failed (%d): %s", regARes.Code, regARes.Body.String())
	}
	var regAPayload struct {
		AccessToken string           `json:"accessToken"`
		User        identityapp.User `json:"user"`
	}
	_ = json.Unmarshal(regARes.Body.Bytes(), &regAPayload)
	tokenA1 := regAPayload.AccessToken

	// Verify /api/v1/me returns Customer A's user and online customer profile.
	meARes := doJSON(http.MethodGet, "/api/v1/me", nil, tokenA1)
	if meARes.Code != http.StatusOK {
		t.Fatalf("get /me failed (%d): %s", meARes.Code, meARes.Body.String())
	}
	var meProfile peopleapp.MeProfile
	_ = json.Unmarshal(meARes.Body.Bytes(), &meProfile)
	if meProfile.Customer == nil || meProfile.Customer.Source != peopleapp.CustomerSourceOnline || meProfile.Customer.Phone != "0901111111" {
		t.Fatalf("expected online customer profile on /me, got %+v", meProfile.Customer)
	}

	// Update /api/v1/me (phone, address, nationalityCode, governmentId).
	patchMeRes := doJSON(http.MethodPatch, "/api/v1/me", map[string]any{
		"displayName":     "Customer A Updated",
		"phone":           "0909999999",
		"nationalityCode": "vn",
		"governmentId":    "079123456789",
		"address":         "123 Le Loi, HCMC",
	}, tokenA1)
	if patchMeRes.Code != http.StatusOK {
		t.Fatalf("patch /me failed (%d): %s", patchMeRes.Code, patchMeRes.Body.String())
	}

	// 2. Customer B registers.
	regBRes := doJSON(http.MethodPost, "/api/v1/auth/register", map[string]any{
		"email":       "customer.b@example.com",
		"password":    "CustomerBPass123",
		"displayName": "Customer B",
		"phone":       "0902222222",
	}, "")
	var regBPayload struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(regBRes.Body.Bytes(), &regBPayload)
	tokenB := regBPayload.AccessToken

	// 3. Negative ownership guard:
	//    - Customer A cannot access staff /api/v1/customers endpoint (403 Forbidden).
	//    - Customer A cannot delete Customer B's session via /api/v1/me/sessions/{id} (404 Not Found).
	custAccessStaff := doJSON(http.MethodGet, "/api/v1/customers", nil, tokenA1)
	if custAccessStaff.Code != http.StatusForbidden {
		t.Fatalf("expected customer accessing /api/v1/customers to get 403, got %d", custAccessStaff.Code)
	}

	sessBRes := doJSON(http.MethodGet, "/api/v1/me/sessions", nil, tokenB)
	var sessBPayload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(sessBRes.Body.Bytes(), &sessBPayload)
	if len(sessBPayload.Data) != 1 {
		t.Fatalf("expected 1 session for Customer B, got %d", len(sessBPayload.Data))
	}
	sessionBID := sessBPayload.Data[0].ID

	crossDeleteRes := doJSON(http.MethodDelete, "/api/v1/me/sessions/"+sessionBID, nil, tokenA1)
	if crossDeleteRes.Code != http.StatusNotFound {
		t.Fatalf("expected Customer A deleting Customer B's session to return 404, got %d", crossDeleteRes.Code)
	}

	// 4. Customer A opens a second login session (tokenA2), then changes password from tokenA2 via POST /api/v1/me/password
	//    -> tokenA1 (other session) is revoked immediately, while tokenA2 stays valid!
	tokenA2 := loginAs("customer.a@example.com", "CustomerAPass123")
	changePwRes := doJSON(http.MethodPost, "/api/v1/me/password", map[string]any{
		"currentPassword": "CustomerAPass123",
		"newPassword":     "CustomerANewPass789",
	}, tokenA2)
	if changePwRes.Code != http.StatusOK {
		t.Fatalf("change own password failed (%d): %s", changePwRes.Code, changePwRes.Body.String())
	}

	meWithOldSession := doJSON(http.MethodGet, "/api/v1/me", nil, tokenA1)
	if meWithOldSession.Code != http.StatusUnauthorized {
		t.Fatalf("expected other session tokenA1 to be revoked after /me/password, got %d", meWithOldSession.Code)
	}
	meWithCurrentSession := doJSON(http.MethodGet, "/api/v1/me", nil, tokenA2)
	if meWithCurrentSession.Code != http.StatusOK {
		t.Fatalf("expected current session tokenA2 to remain valid after /me/password, got %d", meWithCurrentSession.Code)
	}

	// 5. Staff creates a walk-in customer (user_id = NULL, source = 'walk_in') -> 201 Created.
	rawGovID := "001099887766"
	walkInRes := doJSON(http.MethodPost, "/api/v1/customers", map[string]any{
		"fullName":        "Walk In Guest",
		"phone":           "0933445566",
		"nationalityCode": "VN",
		"governmentId":    rawGovID,
		"address":         "456 Nguyen Hue",
	}, rootToken)
	if walkInRes.Code != http.StatusCreated {
		t.Fatalf("create walk-in customer failed (%d): %s", walkInRes.Code, walkInRes.Body.String())
	}
	var walkInCust peopleapp.Customer
	_ = json.Unmarshal(walkInRes.Body.Bytes(), &walkInCust)
	if walkInCust.UserID != nil || walkInCust.Source != peopleapp.CustomerSourceWalkIn {
		t.Fatalf("expected walk-in customer with nil UserID, got %+v", walkInCust)
	}

	// 6. Staff lists customers with cursor pagination -> governmentId is masked in list items,
	//    and audit_events never contains the raw governmentId!
	listRes := doJSON(http.MethodGet, "/api/v1/customers?q=Walk&limit=10", nil, rootToken)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list customers failed (%d): %s", listRes.Code, listRes.Body.String())
	}
	if strings.Contains(listRes.Body.String(), rawGovID) {
		t.Fatalf("expected raw governmentId to be masked in customer list response: %s", listRes.Body.String())
	}
	if !strings.Contains(listRes.Body.String(), "****7766") {
		t.Fatalf("expected masked governmentId ****7766 in list response: %s", listRes.Body.String())
	}

	var rawGovAuditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events WHERE after_data::text LIKE '%' || $1 || '%' OR before_data::text LIKE '%' || $1 || '%'`, rawGovID).Scan(&rawGovAuditCount); err != nil {
		t.Fatalf("check audit for raw government_id: %v", err)
	}
	if rawGovAuditCount != 0 {
		t.Fatalf("security violation: raw government_id must never be recorded in audit_events")
	}

	// 7. Archive walk-in customer -> 200 OK.
	archRes := doJSON(http.MethodPost, "/api/v1/customers/"+walkInCust.ID.String()+"/archive", map[string]any{
		"reason": "duplicate walk-in profile",
	}, rootToken)
	if archRes.Code != http.StatusOK {
		t.Fatalf("archive customer failed (%d): %s", archRes.Code, archRes.Body.String())
	}
}
