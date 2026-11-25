package application

import (
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module callers.
type (
	User           = domain.User
	UserCredential = domain.UserCredential
	AuthSession    = domain.AuthSession
	OneTimeToken   = domain.OneTimeToken
)

const (
	UserKindCustomer = domain.UserKindCustomer
	UserKindStaff    = domain.UserKindStaff
	UserKindAdmin    = domain.UserKindAdmin

	UserStatusPendingActivation = domain.UserStatusPendingActivation
	UserStatusActive            = domain.UserStatusActive
	UserStatusDisabled          = domain.UserStatusDisabled

	maxFailedLoginAttempts = 5
	loginLockoutDuration   = 15 * time.Minute
	passwordResetTokenTTL  = 30 * time.Minute
	activationTokenTTL     = 24 * time.Hour

	EventPasswordResetRequested      = "identity.password_reset_requested"
	EventEmployeeActivationRequested = "identity.employee_activation_requested"
)

// dummyArgon2idHash is verified when a login email does not exist so timing remains consistent.
var dummyArgon2idHash string

func init() {
	h, err := auth.HashPasswordWithParams("DummyPassword123!", auth.DefaultPasswordParams())
	if err != nil {
		panic(err)
	}
	dummyArgon2idHash = h
}

// Repository defines the persistence contract for identity accounts and sessions.
type Repository interface {
	CreateUser(ctx context.Context, user domain.User) (domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetUserByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByEmailForUpdate(ctx context.Context, email string) (domain.User, error)
	UpdateUserStatus(ctx context.Context, id uuid.UUID, status string, now time.Time) (domain.User, error)
	UpdateUserDisplayName(ctx context.Context, id uuid.UUID, displayName string, now time.Time) (domain.User, error)
	TouchUserLastLogin(ctx context.Context, id uuid.UUID, now time.Time) error
	BumpUserAuthorizationVersion(ctx context.Context, id uuid.UUID, now time.Time) error
	BumpUsersAuthorizationVersion(ctx context.Context, ids []uuid.UUID, now time.Time) error

	UpsertUserCredential(ctx context.Context, userID uuid.UUID, passwordHash string, changedAt time.Time) error
	GetUserCredential(ctx context.Context, userID uuid.UUID) (domain.UserCredential, error)
	GetUserCredentialForUpdate(ctx context.Context, userID uuid.UUID) (domain.UserCredential, error)
	RecordFailedLogin(ctx context.Context, userID uuid.UUID, count int, lockedUntil *time.Time) error
	ResetFailedLogin(ctx context.Context, userID uuid.UUID) error

	InsertAuthSession(ctx context.Context, sess domain.AuthSession) (domain.AuthSession, error)
	GetAuthSessionByHashForUpdate(ctx context.Context, hash []byte) (domain.AuthSession, error)
	GetAuthSessionByID(ctx context.Context, id uuid.UUID) (domain.AuthSession, error)
	ListActiveSessionsByUser(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AuthSession, error)
	RotateAuthSession(ctx context.Context, id uuid.UUID, revokedAt time.Time, replacedBy uuid.UUID) error
	RevokeAuthSessionByID(ctx context.Context, sessionID, userID uuid.UUID, revokedAt time.Time) (int64, error)
	RevokeAuthSessionFamily(ctx context.Context, familyID uuid.UUID, revokedAt time.Time) error
	RevokeAllUserAuthSessions(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
	RevokeOtherUserAuthSessions(ctx context.Context, userID, keepSessionID uuid.UUID, revokedAt time.Time) error

	InsertPasswordResetToken(ctx context.Context, tok domain.OneTimeToken) (domain.OneTimeToken, error)
	GetPasswordResetTokenByHashForUpdate(ctx context.Context, hash []byte) (domain.OneTimeToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
	InvalidateUnusedPasswordResetTokensByUser(ctx context.Context, userID uuid.UUID, usedAt time.Time) error

	InsertActivationToken(ctx context.Context, tok domain.OneTimeToken) (domain.OneTimeToken, error)
	GetActivationTokenByHashForUpdate(ctx context.Context, hash []byte) (domain.OneTimeToken, error)
	MarkActivationTokenUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error
	InvalidateUnusedActivationTokensByUser(ctx context.Context, userID uuid.UUID, usedAt time.Time) error
}

// CustomerRegistrationHook is invoked inside the same PostgreSQL transaction when a customer registers.
// Phase 1 T1.6 wires people.CreateCustomerForUser; Phase 4 T4.1 wires wallet.EnsureWallet.
type CustomerRegistrationHook interface {
	OnCustomerRegistered(ctx context.Context, user domain.User, phone string) error
}

// CustomerRegistrationHookFunc adapts a function to CustomerRegistrationHook.
type CustomerRegistrationHookFunc func(ctx context.Context, user domain.User, phone string) error

func (f CustomerRegistrationHookFunc) OnCustomerRegistered(ctx context.Context, user domain.User, phone string) error {
	return f(ctx, user, phone)
}

// CreateActiveAdminParams defines parameters for bootstrapping an active admin account.
type CreateActiveAdminParams struct {
	Email       string
	Password    string
	DisplayName string
	Username    string
}

// RegisterCustomerParams defines parameters for customer self-registration.
type RegisterCustomerParams struct {
	Email       string
	Password    string
	DisplayName string
	Phone       string
	ClientIP    string
	UserAgent   string
}

// LoginParams defines parameters for authenticating a user with email & password.
type LoginParams struct {
	Email     string
	Password  string
	ClientIP  string
	UserAgent string
}

// RefreshParams defines parameters for rotating a refresh token cookie.
type RefreshParams struct {
	RawRefreshToken string
	ClientIP        string
	UserAgent       string
}

// LogoutParams defines parameters for revoking the caller's current or all sessions.
type LogoutParams struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	All       bool
}

// AuthTokens bundles the newly minted access token, refresh token, and authenticated user profile.
type AuthTokens struct {
	User         domain.User
	Session      domain.AuthSession
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
}

// Service provides identity account use cases and exported cross-module operations.
type Service struct {
	txRunner     db.TxRunner
	repo         Repository
	audit        auditapp.Recorder
	outbox       auditapp.OutboxEnqueuer
	tokens       *auth.TokenManager
	cipher       *auth.PayloadCipher
	clk          clock.Clock
	ids          id.Generator
	customerHook CustomerRegistrationHook
}

// NewService constructs an identity application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	audit auditapp.Recorder,
	outbox auditapp.OutboxEnqueuer,
	clk clock.Clock,
	ids id.Generator,
) *Service {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if ids == nil {
		ids = id.NewUUIDGenerator()
	}
	return &Service{
		txRunner: txRunner,
		repo:     repo,
		audit:    audit,
		outbox:   outbox,
		clk:      clk,
		ids:      ids,
	}
}

// WithTokenManager configures the JWT & refresh token manager on the identity service.
func (s *Service) WithTokenManager(tm *auth.TokenManager) *Service {
	s.tokens = tm
	return s
}

// WithPayloadCipher configures AES-256-GCM encryption for sensitive tokens stored in outbox payloads.
func (s *Service) WithPayloadCipher(cipher *auth.PayloadCipher) *Service {
	s.cipher = cipher
	return s
}

// SetCustomerRegistrationHook registers the transactional hook invoked during customer registration.
func (s *Service) SetCustomerRegistrationHook(hook CustomerRegistrationHook) {
	s.customerHook = hook
}

// Compile-time check that Service implements auth.AccountStateVerifier.
var _ auth.AccountStateVerifier = (*Service)(nil)

// VerifyAccountState checks the live user status, authorization_version, and session state for AuthenticatorMiddleware.
func (s *Service) VerifyAccountState(ctx context.Context, userID, sessionID uuid.UUID) (auth.AccountState, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return auth.AccountState{}, err
	}
	sess, err := s.repo.GetAuthSessionByID(ctx, sessionID)
	if err != nil {
		return auth.AccountState{}, err
	}
	now := s.clk.Now()
	sessActive := sess.UserID == user.ID && sess.RevokedAt == nil && sess.ExpiresAt.After(now)
	return auth.AccountState{
		UserID:               user.ID,
		Kind:                 user.Kind,
		Status:               user.Status,
		AuthorizationVersion: user.AuthorizationVersion,
		SessionActive:        sessActive,
	}, nil
}

// RegisterCustomer creates an active customer user account, Argon2id credentials, triggers OnCustomerRegistered,
// and opens an authenticated session within a single PostgreSQL transaction.
func (s *Service) RegisterCustomer(ctx context.Context, params RegisterCustomerParams) (AuthTokens, error) {
	if s.tokens == nil {
		return AuthTokens{}, apperror.Internal(nil)
	}
	email, err := normalizeEmail(params.Email)
	if err != nil {
		return AuthTokens{}, err
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return AuthTokens{}, apperror.Validation("invalid display name", apperror.FieldError{
			Field:   "displayName",
			Message: "display name is required",
		})
	}
	if err := auth.ValidatePasswordPolicy(params.Password); err != nil {
		return AuthTokens{}, err
	}
	passwordHash, err := auth.HashPassword(params.Password)
	if err != nil {
		return AuthTokens{}, apperror.Internal(err)
	}

	now := s.clk.Now()
	var result AuthTokens

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.CreateUser(txCtx, domain.User{
			ID:          s.ids.NewUUID(),
			Kind:        domain.UserKindCustomer,
			Email:       email,
			DisplayName: displayName,
			Status:      domain.UserStatusActive,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		if err := s.repo.UpsertUserCredential(txCtx, user.ID, passwordHash, now); err != nil {
			return err
		}
		if s.customerHook != nil {
			if err := s.customerHook.OnCustomerRegistered(txCtx, user, strings.TrimSpace(params.Phone)); err != nil {
				return err
			}
		}

		tokens, err := s.issueSessionInTx(txCtx, user, s.ids.NewUUID(), params.ClientIP, params.UserAgent)
		if err != nil {
			return err
		}
		if err := s.repo.TouchUserLastLogin(txCtx, user.ID, now); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &user.ID,
				Action:        "identity.customer_registered",
				ResourceType:  "user",
				ResourceID:    &user.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				IPHash:        auth.HashIdentifier(params.ClientIP),
				AfterData: map[string]any{
					"email":        user.Email,
					"display_name": user.DisplayName,
					"kind":         user.Kind,
					"status":       user.Status,
				},
			}); err != nil {
				return err
			}
		}
		result = tokens
		return nil
	})
	return result, err
}

// Login authenticates an active user with Argon2id, enforces temporary account lockout on repeated failures,
// and returns uniform 401 errors to prevent account enumeration.
func (s *Service) Login(ctx context.Context, params LoginParams) (AuthTokens, error) {
	if s.tokens == nil {
		return AuthTokens{}, apperror.Internal(nil)
	}
	email := strings.ToLower(strings.TrimSpace(params.Email))
	if email == "" || params.Password == "" {
		return AuthTokens{}, apperror.Unauthenticated("invalid credentials")
	}

	now := s.clk.Now()
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Burn equal Argon2id CPU time to prevent timing-based account enumeration.
		_, _ = auth.VerifyPassword(params.Password, dummyArgon2idHash)
		return AuthTokens{}, apperror.Unauthenticated("invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		_, _ = auth.VerifyPassword(params.Password, dummyArgon2idHash)
		return AuthTokens{}, apperror.Unauthenticated("invalid credentials")
	}

	var result AuthTokens
	var authFailed bool

	txErr := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		lockedUser, err := s.repo.GetUserByIDForUpdate(txCtx, user.ID)
		if err != nil {
			return err
		}
		if lockedUser.Status != domain.UserStatusActive {
			authFailed = true
			return nil
		}

		cred, err := s.repo.GetUserCredentialForUpdate(txCtx, lockedUser.ID)
		if err != nil {
			authFailed = true
			return nil
		}

		if cred.LockedUntil != nil && cred.LockedUntil.After(now) {
			authFailed = true
			return nil
		}

		ok, err := auth.VerifyPassword(params.Password, cred.PasswordHash)
		if err != nil || !ok {
			authFailed = true
			nextCount := cred.FailedLoginCount + 1
			var lockUntil *time.Time
			if nextCount >= maxFailedLoginAttempts {
				lu := now.Add(loginLockoutDuration)
				lockUntil = &lu
			}
			return s.repo.RecordFailedLogin(txCtx, lockedUser.ID, nextCount, lockUntil)
		}

		if cred.FailedLoginCount > 0 || cred.LockedUntil != nil {
			if err := s.repo.ResetFailedLogin(txCtx, lockedUser.ID); err != nil {
				return err
			}
		}
		if err := s.repo.TouchUserLastLogin(txCtx, lockedUser.ID, now); err != nil {
			return err
		}

		tokens, err := s.issueSessionInTx(txCtx, lockedUser, s.ids.NewUUID(), params.ClientIP, params.UserAgent)
		if err != nil {
			return err
		}
		result = tokens
		return nil
	})
	if txErr != nil {
		return AuthTokens{}, apperror.Unauthenticated("invalid credentials")
	}
	if authFailed {
		return AuthTokens{}, apperror.Unauthenticated("invalid credentials")
	}
	return result, nil
}

// RefreshSession rotates a refresh token within its family.
// If a previously rotated/revoked token is replayed (reuse detection), the entire token family
// is revoked immediately, an audit alert is recorded, and 401 is returned.
func (s *Service) RefreshSession(ctx context.Context, params RefreshParams) (AuthTokens, error) {
	if s.tokens == nil {
		return AuthTokens{}, apperror.Internal(nil)
	}
	rawToken := strings.TrimSpace(params.RawRefreshToken)
	if rawToken == "" {
		return AuthTokens{}, apperror.Unauthenticated("missing refresh token")
	}

	tokenHash := auth.HashOpaqueToken(rawToken)
	now := s.clk.Now()

	var (
		result     AuthTokens
		authFailed bool
	)

	txErr := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		sess, err := s.repo.GetAuthSessionByHashForUpdate(txCtx, tokenHash)
		if err != nil {
			authFailed = true
			return nil
		}

		// Reuse detection: if this token was already rotated or revoked, revoke the whole family!
		if sess.RevokedAt != nil || sess.ReplacedBy != nil {
			authFailed = true
			if err := s.repo.RevokeAuthSessionFamily(txCtx, sess.FamilyID, now); err != nil {
				return err
			}
			if s.audit != nil {
				_ = s.audit.Record(txCtx, auditapp.AuditEntry{
					ActorUserID:   &sess.UserID,
					Action:        "identity.refresh_token_reuse_detected",
					ResourceType:  "auth_session",
					ResourceID:    &sess.ID,
					RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
					CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
					IPHash:        auth.HashIdentifier(params.ClientIP),
					Reason:        "replayed rotated refresh token; revoked entire token family",
					AfterData: map[string]any{
						"family_id":  sess.FamilyID.String(),
						"session_id": sess.ID.String(),
					},
				})
			}
			return nil
		}

		if !sess.ExpiresAt.After(now) {
			authFailed = true
			return nil
		}

		user, err := s.repo.GetUserByID(txCtx, sess.UserID)
		if err != nil || user.Status != domain.UserStatusActive {
			authFailed = true
			return nil
		}

		newTokens, err := s.issueSessionInTx(txCtx, user, sess.FamilyID, params.ClientIP, params.UserAgent)
		if err != nil {
			return err
		}

		if err := s.repo.RotateAuthSession(txCtx, sess.ID, now, newTokens.Session.ID); err != nil {
			return err
		}

		result = newTokens
		return nil
	})
	if txErr != nil || authFailed {
		return AuthTokens{}, apperror.Unauthenticated("invalid or expired refresh token")
	}
	return result, nil
}

// Logout revokes the caller's current session (or all active sessions if params.All is true).
func (s *Service) Logout(ctx context.Context, params LogoutParams) error {
	now := s.clk.Now()
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if params.All {
			return s.repo.RevokeAllUserAuthSessions(txCtx, params.UserID, now)
		}
		_, err := s.repo.RevokeAuthSessionByID(txCtx, params.SessionID, params.UserID, now)
		return err
	})
}

// PasswordResetRequestParams defines parameters for initiating a password reset.
type PasswordResetRequestParams struct {
	Email    string
	ClientIP string
}

// PasswordResetConfirmParams defines parameters for completing a password reset with a single-use token.
type PasswordResetConfirmParams struct {
	RawToken    string
	NewPassword string
	ClientIP    string
}

type passwordResetOutboxPayload struct {
	UserID         uuid.UUID `json:"userId"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"displayName"`
	EncryptedToken string    `json:"encryptedToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

// RequestPasswordReset generates a 32-byte single-use token (stored as SHA-256 in password_reset_tokens,
// encrypted with AES-256-GCM in outbox_events) if the email belongs to an active user.
// It always returns nil for non-existent or inactive emails to prevent account enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, params PasswordResetRequestParams) error {
	email, err := normalizeEmail(params.Email)
	if err != nil {
		return err
	}
	if s.cipher == nil || s.outbox == nil {
		return apperror.Internal(nil)
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user.Status != domain.UserStatusActive {
		return nil
	}

	rawToken, tokenHash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return apperror.Internal(err)
	}
	encryptedToken, err := s.cipher.Encrypt(rawToken)
	if err != nil {
		return apperror.Internal(err)
	}

	now := s.clk.Now()
	expiresAt := now.Add(passwordResetTokenTTL)

	payloadBytes, err := json.Marshal(passwordResetOutboxPayload{
		UserID:         user.ID,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		EncryptedToken: encryptedToken,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		return apperror.Internal(err)
	}

	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		lockedUser, err := s.repo.GetUserByIDForUpdate(txCtx, user.ID)
		if err != nil || lockedUser.Status != domain.UserStatusActive {
			return nil
		}
		if err := s.repo.InvalidateUnusedPasswordResetTokensByUser(txCtx, lockedUser.ID, now); err != nil {
			return err
		}
		tok, err := s.repo.InsertPasswordResetToken(txCtx, domain.OneTimeToken{
			ID:        s.ids.NewUUID(),
			UserID:    lockedUser.ID,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
			ID:            s.ids.NewUUID(),
			AggregateType: "user",
			AggregateID:   lockedUser.ID,
			EventType:     EventPasswordResetRequested,
			Payload:       payloadBytes,
			OccurredAt:    now,
			AvailableAt:   now,
		}); err != nil {
			return err
		}
		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &lockedUser.ID,
				Action:        "identity.password_reset_requested",
				ResourceType:  "password_reset_token",
				ResourceID:    &tok.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				IPHash:        auth.HashIdentifier(params.ClientIP),
				AfterData: map[string]any{
					"user_id":    lockedUser.ID.String(),
					"expires_at": expiresAt.UTC().Format(time.RFC3339),
				},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ConfirmPasswordReset locks the token by SHA-256 hash (FOR UPDATE), verifies it is unused and unexpired,
// updates the Argon2id password hash, marks the token used, revokes all sessions of the user, bumps
// authorization_version, and records an audit entry atomically in a single transaction.
func (s *Service) ConfirmPasswordReset(ctx context.Context, params PasswordResetConfirmParams) error {
	rawToken := strings.TrimSpace(params.RawToken)
	if rawToken == "" {
		return apperror.Validation("invalid reset token", apperror.FieldError{
			Field:   "token",
			Message: "token is required",
		})
	}
	if err := auth.ValidatePasswordPolicy(params.NewPassword); err != nil {
		return err
	}
	newHash, err := auth.HashPassword(params.NewPassword)
	if err != nil {
		return apperror.Internal(err)
	}

	tokenHash := auth.HashOpaqueToken(rawToken)
	now := s.clk.Now()

	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		tok, err := s.repo.GetPasswordResetTokenByHashForUpdate(txCtx, tokenHash)
		if err != nil {
			return apperror.Unauthenticated("invalid or expired password reset token")
		}
		if tok.UsedAt != nil || !tok.ExpiresAt.After(now) {
			return apperror.Unauthenticated("invalid or expired password reset token")
		}

		user, err := s.repo.GetUserByIDForUpdate(txCtx, tok.UserID)
		if err != nil || user.Status != domain.UserStatusActive {
			return apperror.Unauthenticated("invalid or expired password reset token")
		}

		if err := s.repo.UpsertUserCredential(txCtx, user.ID, newHash, now); err != nil {
			return err
		}
		if err := s.repo.MarkPasswordResetTokenUsed(txCtx, tok.ID, now); err != nil {
			return err
		}
		if err := s.repo.InvalidateUnusedPasswordResetTokensByUser(txCtx, user.ID, now); err != nil {
			return err
		}
		if err := s.repo.RevokeAllUserAuthSessions(txCtx, user.ID, now); err != nil {
			return err
		}
		if err := s.repo.BumpUserAuthorizationVersion(txCtx, user.ID, now); err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &user.ID,
				Action:        "identity.password_reset_confirmed",
				ResourceType:  "user",
				ResourceID:    &user.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				IPHash:        auth.HashIdentifier(params.ClientIP),
				Reason:        "password reset confirmed; all prior sessions revoked",
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// CreatePendingStaffParams defines inputs for provisioning a pending staff user and single-use activation token.
type CreatePendingStaffParams struct {
	Email       string
	DisplayName string
	Username    string
}

// PendingStaffProvision bundles the created pending staff user with the AES-256-GCM encrypted activation token.
type PendingStaffProvision struct {
	User           domain.User
	TokenID        uuid.UUID
	EncryptedToken string
	ExpiresAt      time.Time
}

// ActivateAccountParams defines inputs for activating a pending staff user account.
type ActivateAccountParams struct {
	RawToken  string
	Password  string
	ClientIP  string
	UserAgent string
}

// CreatePendingStaffUser creates a staff user in pending_activation status and issues a 24h single-use activation token
// (SHA-256 hash stored in activation_tokens, AES-256-GCM ciphertext returned for outbox delivery).
func (s *Service) CreatePendingStaffUser(ctx context.Context, params CreatePendingStaffParams) (PendingStaffProvision, error) {
	if s.cipher == nil {
		return PendingStaffProvision{}, apperror.Internal(nil)
	}
	email, err := normalizeEmail(params.Email)
	if err != nil {
		return PendingStaffProvision{}, err
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return PendingStaffProvision{}, apperror.Validation("invalid display name", apperror.FieldError{
			Field:   "fullName",
			Message: "full name is required",
		})
	}

	rawToken, tokenHash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return PendingStaffProvision{}, apperror.Internal(err)
	}
	encryptedToken, err := s.cipher.Encrypt(rawToken)
	if err != nil {
		return PendingStaffProvision{}, apperror.Internal(err)
	}

	now := s.clk.Now()
	expiresAt := now.Add(activationTokenTTL)
	var out PendingStaffProvision

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.CreateUser(txCtx, domain.User{
			ID:          s.ids.NewUUID(),
			Kind:        domain.UserKindStaff,
			Email:       email,
			Username:    strings.TrimSpace(params.Username),
			DisplayName: displayName,
			Status:      domain.UserStatusPendingActivation,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		tok, err := s.repo.InsertActivationToken(txCtx, domain.OneTimeToken{
			ID:        s.ids.NewUUID(),
			UserID:    user.ID,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			CreatedAt: now,
		})
		if err != nil {
			return err
		}
		out = PendingStaffProvision{
			User:           user,
			TokenID:        tok.ID,
			EncryptedToken: encryptedToken,
			ExpiresAt:      expiresAt,
		}
		return nil
	})
	return out, err
}

// ReissueActivationToken invalidates any unused activation tokens for a user still in pending_activation
// and mints a fresh 24h activation token.
func (s *Service) ReissueActivationToken(ctx context.Context, userID uuid.UUID) (PendingStaffProvision, error) {
	if s.cipher == nil {
		return PendingStaffProvision{}, apperror.Internal(nil)
	}
	rawToken, tokenHash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return PendingStaffProvision{}, apperror.Internal(err)
	}
	encryptedToken, err := s.cipher.Encrypt(rawToken)
	if err != nil {
		return PendingStaffProvision{}, apperror.Internal(err)
	}

	now := s.clk.Now()
	expiresAt := now.Add(activationTokenTTL)
	var out PendingStaffProvision

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		if user.Status != domain.UserStatusPendingActivation {
			return apperror.Conflict("user account is not in pending_activation status")
		}
		if err := s.repo.InvalidateUnusedActivationTokensByUser(txCtx, user.ID, now); err != nil {
			return err
		}
		tok, err := s.repo.InsertActivationToken(txCtx, domain.OneTimeToken{
			ID:        s.ids.NewUUID(),
			UserID:    user.ID,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			CreatedAt: now,
		})
		if err != nil {
			return err
		}
		out = PendingStaffProvision{
			User:           user,
			TokenID:        tok.ID,
			EncryptedToken: encryptedToken,
			ExpiresAt:      expiresAt,
		}
		return nil
	})
	return out, err
}

// ActivateAccount consumes a single-use activation token (locked FOR UPDATE), stores the Argon2id password hash,
// transitions the user from pending_activation to active, records an audit event, and issues a login session.
func (s *Service) ActivateAccount(ctx context.Context, params ActivateAccountParams) (AuthTokens, error) {
	if s.tokens == nil {
		return AuthTokens{}, apperror.Internal(nil)
	}
	rawToken := strings.TrimSpace(params.RawToken)
	if rawToken == "" {
		return AuthTokens{}, apperror.Validation("invalid activation token", apperror.FieldError{
			Field:   "token",
			Message: "token is required",
		})
	}
	if err := auth.ValidatePasswordPolicy(params.Password); err != nil {
		return AuthTokens{}, err
	}
	passwordHash, err := auth.HashPassword(params.Password)
	if err != nil {
		return AuthTokens{}, apperror.Internal(err)
	}

	tokenHash := auth.HashOpaqueToken(rawToken)
	now := s.clk.Now()
	var result AuthTokens

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		tok, err := s.repo.GetActivationTokenByHashForUpdate(txCtx, tokenHash)
		if err != nil {
			return apperror.Unauthenticated("invalid or expired activation token")
		}
		if tok.UsedAt != nil || !tok.ExpiresAt.After(now) {
			return apperror.Unauthenticated("invalid or expired activation token")
		}

		user, err := s.repo.GetUserByIDForUpdate(txCtx, tok.UserID)
		if err != nil {
			return apperror.Unauthenticated("invalid or expired activation token")
		}
		if user.Status != domain.UserStatusPendingActivation {
			return apperror.Unauthenticated("account is not eligible for activation")
		}

		if err := s.repo.UpsertUserCredential(txCtx, user.ID, passwordHash, now); err != nil {
			return err
		}
		updatedUser, err := s.repo.UpdateUserStatus(txCtx, user.ID, domain.UserStatusActive, now)
		if err != nil {
			return err
		}
		if err := s.repo.MarkActivationTokenUsed(txCtx, tok.ID, now); err != nil {
			return err
		}
		if err := s.repo.InvalidateUnusedActivationTokensByUser(txCtx, user.ID, now); err != nil {
			return err
		}
		if err := s.repo.TouchUserLastLogin(txCtx, user.ID, now); err != nil {
			return err
		}

		tokens, err := s.issueSessionInTx(txCtx, updatedUser, s.ids.NewUUID(), params.ClientIP, params.UserAgent)
		if err != nil {
			return err
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   &updatedUser.ID,
				Action:        "identity.account_activated",
				ResourceType:  "user",
				ResourceID:    &updatedUser.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				IPHash:        auth.HashIdentifier(params.ClientIP),
				BeforeData: map[string]any{
					"status": domain.UserStatusPendingActivation,
				},
				AfterData: map[string]any{
					"status": updatedUser.Status,
				},
			}); err != nil {
				return err
			}
		}

		result = tokens
		return nil
	})
	return result, err
}

// DeactivateUser transitions a user account to disabled, invalidates unused activation/reset tokens,
// revokes all active sessions, and increments authorization_version inside the active transaction.
func (s *Service) DeactivateUser(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	now := s.clk.Now()
	var updated domain.User
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := s.repo.GetUserByIDForUpdate(txCtx, userID); err != nil {
			return err
		}
		u, err := s.repo.UpdateUserStatus(txCtx, userID, domain.UserStatusDisabled, now)
		if err != nil {
			return err
		}
		if err := s.repo.InvalidateUnusedActivationTokensByUser(txCtx, userID, now); err != nil {
			return err
		}
		if err := s.repo.InvalidateUnusedPasswordResetTokensByUser(txCtx, userID, now); err != nil {
			return err
		}
		if err := s.repo.RevokeAllUserAuthSessions(txCtx, userID, now); err != nil {
			return err
		}
		updated = u
		return nil
	})
	return updated, err
}

// UpdateDisplayName updates the display_name of userID inside the active transaction.
func (s *Service) UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) (domain.User, error) {
	trimmed := strings.TrimSpace(displayName)
	if trimmed == "" {
		return domain.User{}, apperror.Validation("invalid display name", apperror.FieldError{
			Field:   "displayName",
			Message: "display name is required",
		})
	}
	return s.repo.UpdateUserDisplayName(ctx, userID, trimmed, s.clk.Now())
}

func (s *Service) issueSessionInTx(
	ctx context.Context,
	user domain.User,
	familyID uuid.UUID,
	clientIP string,
	userAgent string,
) (AuthTokens, error) {
	rawRefresh, refreshHash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return AuthTokens{}, apperror.Internal(err)
	}
	now := s.clk.Now()
	sessionID := s.ids.NewUUID()
	if familyID == uuid.Nil {
		familyID = s.ids.NewUUID()
	}

	sess, err := s.repo.InsertAuthSession(ctx, domain.AuthSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: refreshHash,
		FamilyID:         familyID,
		IssuedAt:         now,
		ExpiresAt:        now.Add(s.tokens.RefreshTTL()),
		IPHash:           auth.HashIdentifier(clientIP),
		UserAgentHash:    auth.HashIdentifier(userAgent),
	})
	if err != nil {
		return AuthTokens{}, err
	}

	accessToken, _, err := s.tokens.SignAccessToken(auth.Principal{
		UserID:               user.ID,
		Kind:                 user.Kind,
		SessionID:            sess.ID,
		AuthorizationVersion: user.AuthorizationVersion,
	})
	if err != nil {
		return AuthTokens{}, apperror.Internal(err)
	}

	return AuthTokens{
		User:         user,
		Session:      sess,
		AccessToken:  accessToken,
		ExpiresIn:    int(s.tokens.AccessTTL().Seconds()),
		RefreshToken: rawRefresh,
	}, nil
}

// CreateActiveAdminUser creates an active admin user with Argon2id credentials inside the active transaction.
func (s *Service) CreateActiveAdminUser(ctx context.Context, params CreateActiveAdminParams) (domain.User, error) {
	email, err := normalizeEmail(params.Email)
	if err != nil {
		return domain.User{}, err
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return domain.User{}, apperror.Validation("invalid display name", apperror.FieldError{
			Field:   "displayName",
			Message: "display name is required",
		})
	}
	if err := auth.ValidatePasswordPolicy(params.Password); err != nil {
		return domain.User{}, err
	}
	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		return domain.User{}, apperror.Internal(err)
	}

	now := s.clk.Now()
	userID := s.ids.NewUUID()
	var created domain.User

	err = s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		u, err := s.repo.CreateUser(txCtx, domain.User{
			ID:          userID,
			Kind:        domain.UserKindAdmin,
			Email:       email,
			Username:    strings.TrimSpace(params.Username),
			DisplayName: displayName,
			Status:      domain.UserStatusActive,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		if err := s.repo.UpsertUserCredential(txCtx, u.ID, hash, now); err != nil {
			return err
		}
		created = u
		return nil
	})
	return created, err
}

// GetUserByID retrieves a user account by UUID.
func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// GetUserByIDForUpdate locks and retrieves a user account by UUID inside a transaction.
func (s *Service) GetUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.repo.GetUserByIDForUpdate(ctx, userID)
}

// BumpAuthVersion increments users.authorization_version for the given user IDs inside the active transaction.
func (s *Service) BumpAuthVersion(ctx context.Context, userIDs ...uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}
	if len(userIDs) == 1 {
		return s.repo.BumpUserAuthorizationVersion(ctx, userIDs[0], s.clk.Now())
	}
	return s.repo.BumpUsersAuthorizationVersion(ctx, userIDs, s.clk.Now())
}

func normalizeEmail(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", apperror.Validation("invalid email", apperror.FieldError{
			Field:   "email",
			Message: "email is required",
		})
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Address != trimmed || !strings.Contains(trimmed, "@") {
		return "", apperror.Validation("invalid email", apperror.FieldError{
			Field:   "email",
			Message: "must be a valid email address",
		})
	}
	return trimmed, nil
}
