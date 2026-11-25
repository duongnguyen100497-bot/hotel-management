package application

import (
	"context"
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
)

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

// CreateActiveAdminParams defines parameters for bootstrapping an active admin account.
type CreateActiveAdminParams struct {
	Email       string
	Password    string
	DisplayName string
	Username    string
}

// Service provides identity account use cases and exported cross-module operations.
type Service struct {
	txRunner db.TxRunner
	repo     Repository
	audit    auditapp.Recorder
	outbox   auditapp.OutboxEnqueuer
	clk      clock.Clock
	ids      id.Generator
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
