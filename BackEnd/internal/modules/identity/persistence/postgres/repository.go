package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository persists users, credentials, sessions, and one-time tokens in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL identity repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func (r *Repository) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	var username pgtype.Text
	if trimmed := strings.TrimSpace(user.Username); trimmed != "" {
		username = pgtype.Text{String: trimmed, Valid: true}
	}
	row, err := r.queries(ctx).CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:          user.ID,
		Kind:        user.Kind,
		Email:       strings.TrimSpace(user.Email),
		Username:    username,
		DisplayName: strings.TrimSpace(user.DisplayName),
		Status:      user.Status,
		CreatedAt:   user.CreatedAt.UTC(),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.User{}, apperror.Conflict("email or username already exists").WithCause(err)
		}
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.queries(ctx).GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetUserByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.queries(ctx).GetUserByIDForUpdate(ctx, id)
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.queries(ctx).GetUserByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) GetUserByEmailForUpdate(ctx context.Context, email string) (domain.User, error) {
	row, err := r.queries(ctx).GetUserByEmailForUpdate(ctx, strings.TrimSpace(email))
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) UpdateUserStatus(ctx context.Context, id uuid.UUID, status string, now time.Time) (domain.User, error) {
	row, err := r.queries(ctx).UpdateUserStatus(ctx, sqlcgen.UpdateUserStatusParams{
		ID:        id,
		Status:    status,
		UpdatedAt: now.UTC(),
	})
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) UpdateUserDisplayName(ctx context.Context, id uuid.UUID, displayName string, now time.Time) (domain.User, error) {
	row, err := r.queries(ctx).UpdateUserDisplayName(ctx, sqlcgen.UpdateUserDisplayNameParams{
		ID:          id,
		DisplayName: strings.TrimSpace(displayName),
		UpdatedAt:   now.UTC(),
	})
	if err != nil {
		return domain.User{}, db.MapPgError(err)
	}
	return mapUserRow(row.ID, row.Kind, row.Email, row.Username, row.DisplayName, row.Status, row.AuthorizationVersion, row.LastLoginAt, row.CreatedAt, row.UpdatedAt, row.Version), nil
}

func (r *Repository) TouchUserLastLogin(ctx context.Context, id uuid.UUID, now time.Time) error {
	ts := now.UTC()
	return db.MapPgError(r.queries(ctx).TouchUserLastLogin(ctx, sqlcgen.TouchUserLastLoginParams{
		ID:          id,
		LastLoginAt: &ts,
	}))
}

func (r *Repository) BumpUserAuthorizationVersion(ctx context.Context, id uuid.UUID, now time.Time) error {
	return db.MapPgError(r.queries(ctx).BumpUserAuthorizationVersion(ctx, sqlcgen.BumpUserAuthorizationVersionParams{
		ID:        id,
		UpdatedAt: now.UTC(),
	}))
}

func (r *Repository) BumpUsersAuthorizationVersion(ctx context.Context, ids []uuid.UUID, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return db.MapPgError(r.queries(ctx).BumpUsersAuthorizationVersion(ctx, sqlcgen.BumpUsersAuthorizationVersionParams{
		Column1:   ids,
		UpdatedAt: now.UTC(),
	}))
}

func (r *Repository) UpsertUserCredential(ctx context.Context, userID uuid.UUID, passwordHash string, changedAt time.Time) error {
	return db.MapPgError(r.queries(ctx).UpsertUserCredential(ctx, sqlcgen.UpsertUserCredentialParams{
		UserID:            userID,
		PasswordHash:      passwordHash,
		PasswordChangedAt: changedAt.UTC(),
	}))
}

func (r *Repository) GetUserCredential(ctx context.Context, userID uuid.UUID) (domain.UserCredential, error) {
	row, err := r.queries(ctx).GetUserCredential(ctx, userID)
	if err != nil {
		return domain.UserCredential{}, db.MapPgError(err)
	}
	return domain.UserCredential{
		UserID:            row.UserID,
		PasswordHash:      row.PasswordHash,
		PasswordChangedAt: row.PasswordChangedAt,
		FailedLoginCount:  int(row.FailedLoginCount),
		LockedUntil:       row.LockedUntil,
	}, nil
}

func (r *Repository) GetUserCredentialForUpdate(ctx context.Context, userID uuid.UUID) (domain.UserCredential, error) {
	row, err := r.queries(ctx).GetUserCredentialForUpdate(ctx, userID)
	if err != nil {
		return domain.UserCredential{}, db.MapPgError(err)
	}
	return domain.UserCredential{
		UserID:            row.UserID,
		PasswordHash:      row.PasswordHash,
		PasswordChangedAt: row.PasswordChangedAt,
		FailedLoginCount:  int(row.FailedLoginCount),
		LockedUntil:       row.LockedUntil,
	}, nil
}

func (r *Repository) RecordFailedLogin(ctx context.Context, userID uuid.UUID, count int, lockedUntil *time.Time) error {
	var lu *time.Time
	if lockedUntil != nil {
		ts := lockedUntil.UTC()
		lu = &ts
	}
	return db.MapPgError(r.queries(ctx).RecordFailedLogin(ctx, sqlcgen.RecordFailedLoginParams{
		UserID:           userID,
		FailedLoginCount: int32(count),
		LockedUntil:      lu,
	}))
}

func (r *Repository) ResetFailedLogin(ctx context.Context, userID uuid.UUID) error {
	return db.MapPgError(r.queries(ctx).ResetFailedLogin(ctx, userID))
}

func (r *Repository) InsertAuthSession(ctx context.Context, sess domain.AuthSession) (domain.AuthSession, error) {
	row, err := r.queries(ctx).InsertAuthSession(ctx, sqlcgen.InsertAuthSessionParams{
		ID:               sess.ID,
		UserID:           sess.UserID,
		RefreshTokenHash: sess.RefreshTokenHash,
		FamilyID:         sess.FamilyID,
		IssuedAt:         sess.IssuedAt.UTC(),
		ExpiresAt:        sess.ExpiresAt.UTC(),
		IpHash:           sess.IPHash,
		UserAgentHash:    sess.UserAgentHash,
	})
	if err != nil {
		return domain.AuthSession{}, db.MapPgError(err)
	}
	return mapSessionRow(row), nil
}

func (r *Repository) GetAuthSessionByHashForUpdate(ctx context.Context, hash []byte) (domain.AuthSession, error) {
	row, err := r.queries(ctx).GetAuthSessionByHashForUpdate(ctx, hash)
	if err != nil {
		return domain.AuthSession{}, db.MapPgError(err)
	}
	return mapSessionRow(row), nil
}

func (r *Repository) GetAuthSessionByID(ctx context.Context, id uuid.UUID) (domain.AuthSession, error) {
	row, err := r.queries(ctx).GetAuthSessionByID(ctx, id)
	if err != nil {
		return domain.AuthSession{}, db.MapPgError(err)
	}
	return mapSessionRow(row), nil
}

func (r *Repository) ListActiveSessionsByUser(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.AuthSession, error) {
	rows, err := r.queries(ctx).ListActiveSessionsByUser(ctx, sqlcgen.ListActiveSessionsByUserParams{
		UserID:    userID,
		ExpiresAt: now.UTC(),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.AuthSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSessionRow(row))
	}
	return out, nil
}

func (r *Repository) RotateAuthSession(ctx context.Context, id uuid.UUID, revokedAt time.Time, replacedBy uuid.UUID) error {
	ts := revokedAt.UTC()
	return db.MapPgError(r.queries(ctx).RotateAuthSession(ctx, sqlcgen.RotateAuthSessionParams{
		ID:         id,
		RevokedAt:  &ts,
		ReplacedBy: &replacedBy,
	}))
}

func (r *Repository) RevokeAuthSessionByID(ctx context.Context, sessionID, userID uuid.UUID, revokedAt time.Time) (int64, error) {
	ts := revokedAt.UTC()
	n, err := r.queries(ctx).RevokeAuthSessionByID(ctx, sqlcgen.RevokeAuthSessionByIDParams{
		ID:        sessionID,
		UserID:    userID,
		RevokedAt: &ts,
	})
	return n, db.MapPgError(err)
}

func (r *Repository) RevokeAuthSessionFamily(ctx context.Context, familyID uuid.UUID, revokedAt time.Time) error {
	ts := revokedAt.UTC()
	return db.MapPgError(r.queries(ctx).RevokeAuthSessionFamily(ctx, sqlcgen.RevokeAuthSessionFamilyParams{
		FamilyID:  familyID,
		RevokedAt: &ts,
	}))
}

func (r *Repository) RevokeAllUserAuthSessions(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	ts := revokedAt.UTC()
	return db.MapPgError(r.queries(ctx).RevokeAllUserAuthSessions(ctx, sqlcgen.RevokeAllUserAuthSessionsParams{
		UserID:    userID,
		RevokedAt: &ts,
	}))
}

func (r *Repository) RevokeOtherUserAuthSessions(ctx context.Context, userID, keepSessionID uuid.UUID, revokedAt time.Time) error {
	ts := revokedAt.UTC()
	return db.MapPgError(r.queries(ctx).RevokeOtherUserAuthSessions(ctx, sqlcgen.RevokeOtherUserAuthSessionsParams{
		UserID:    userID,
		ID:        keepSessionID,
		RevokedAt: &ts,
	}))
}

func (r *Repository) InsertPasswordResetToken(ctx context.Context, tok domain.OneTimeToken) (domain.OneTimeToken, error) {
	row, err := r.queries(ctx).InsertPasswordResetToken(ctx, sqlcgen.InsertPasswordResetTokenParams{
		ID:        tok.ID,
		UserID:    tok.UserID,
		TokenHash: tok.TokenHash,
		ExpiresAt: tok.ExpiresAt.UTC(),
		CreatedAt: tok.CreatedAt.UTC(),
	})
	if err != nil {
		return domain.OneTimeToken{}, db.MapPgError(err)
	}
	return domain.OneTimeToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *Repository) GetPasswordResetTokenByHashForUpdate(ctx context.Context, hash []byte) (domain.OneTimeToken, error) {
	row, err := r.queries(ctx).GetPasswordResetTokenByHashForUpdate(ctx, hash)
	if err != nil {
		return domain.OneTimeToken{}, db.MapPgError(err)
	}
	return domain.OneTimeToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *Repository) MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	ts := usedAt.UTC()
	return db.MapPgError(r.queries(ctx).MarkPasswordResetTokenUsed(ctx, sqlcgen.MarkPasswordResetTokenUsedParams{
		ID:     id,
		UsedAt: &ts,
	}))
}

func (r *Repository) InvalidateUnusedPasswordResetTokensByUser(ctx context.Context, userID uuid.UUID, usedAt time.Time) error {
	ts := usedAt.UTC()
	return db.MapPgError(r.queries(ctx).InvalidateUnusedPasswordResetTokensByUser(ctx, sqlcgen.InvalidateUnusedPasswordResetTokensByUserParams{
		UserID: userID,
		UsedAt: &ts,
	}))
}

func (r *Repository) InsertActivationToken(ctx context.Context, tok domain.OneTimeToken) (domain.OneTimeToken, error) {
	row, err := r.queries(ctx).InsertActivationToken(ctx, sqlcgen.InsertActivationTokenParams{
		ID:        tok.ID,
		UserID:    tok.UserID,
		TokenHash: tok.TokenHash,
		ExpiresAt: tok.ExpiresAt.UTC(),
		CreatedAt: tok.CreatedAt.UTC(),
	})
	if err != nil {
		return domain.OneTimeToken{}, db.MapPgError(err)
	}
	return domain.OneTimeToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *Repository) GetActivationTokenByHashForUpdate(ctx context.Context, hash []byte) (domain.OneTimeToken, error) {
	row, err := r.queries(ctx).GetActivationTokenByHashForUpdate(ctx, hash)
	if err != nil {
		return domain.OneTimeToken{}, db.MapPgError(err)
	}
	return domain.OneTimeToken{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *Repository) MarkActivationTokenUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	ts := usedAt.UTC()
	return db.MapPgError(r.queries(ctx).MarkActivationTokenUsed(ctx, sqlcgen.MarkActivationTokenUsedParams{
		ID:     id,
		UsedAt: &ts,
	}))
}

func (r *Repository) InvalidateUnusedActivationTokensByUser(ctx context.Context, userID uuid.UUID, usedAt time.Time) error {
	ts := usedAt.UTC()
	return db.MapPgError(r.queries(ctx).InvalidateUnusedActivationTokensByUser(ctx, sqlcgen.InvalidateUnusedActivationTokensByUserParams{
		UserID: userID,
		UsedAt: &ts,
	}))
}

func mapUserRow(
	id uuid.UUID,
	kind string,
	email string,
	username string,
	displayName string,
	status string,
	authVer int32,
	lastLogin *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
	version int32,
) domain.User {
	return domain.User{
		ID:                   id,
		Kind:                 kind,
		Email:                email,
		Username:             username,
		DisplayName:          displayName,
		Status:               status,
		AuthorizationVersion: int(authVer),
		LastLoginAt:          lastLogin,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		Version:              int(version),
	}
}

func mapSessionRow(row sqlcgen.AuthSession) domain.AuthSession {
	return domain.AuthSession{
		ID:               row.ID,
		UserID:           row.UserID,
		RefreshTokenHash: row.RefreshTokenHash,
		FamilyID:         row.FamilyID,
		IssuedAt:         row.IssuedAt,
		ExpiresAt:        row.ExpiresAt,
		RevokedAt:        row.RevokedAt,
		ReplacedBy:       row.ReplacedBy,
		IPHash:           row.IpHash,
		UserAgentHash:    row.UserAgentHash,
	}
}
