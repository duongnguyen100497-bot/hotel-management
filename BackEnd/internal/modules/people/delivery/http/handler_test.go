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
