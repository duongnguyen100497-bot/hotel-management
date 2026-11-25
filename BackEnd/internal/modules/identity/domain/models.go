package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	UserKindCustomer = "customer"
	UserKindStaff    = "staff"
	UserKindAdmin    = "admin"

	UserStatusPendingActivation = "pending_activation"
	UserStatusActive            = "active"
	UserStatusDisabled          = "disabled"
)

// User represents a canonical identity account across customers, staff, and admins.
type User struct {
	ID                   uuid.UUID  `json:"id"`
	Kind                 string     `json:"kind"`
	Email                string     `json:"email"`
	Username             string     `json:"username,omitempty"`
	DisplayName          string     `json:"displayName"`
	Status               string     `json:"status"`
	AuthorizationVersion int        `json:"authorizationVersion"`
	LastLoginAt          *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	Version              int        `json:"version"`
}

// UserCredential stores the password hash and lockout state of a user.
type UserCredential struct {
	UserID            uuid.UUID
	PasswordHash      string
	PasswordChangedAt time.Time
	FailedLoginCount  int
	LockedUntil       *time.Time
}

// AuthSession represents a rotating refresh-token session within a token family.
type AuthSession struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"userId"`
	RefreshTokenHash []byte     `json:"-"`
	FamilyID         uuid.UUID  `json:"familyId"`
	IssuedAt         time.Time  `json:"issuedAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	ReplacedBy       *uuid.UUID `json:"replacedBy,omitempty"`
	IPHash           []byte     `json:"-"`
	UserAgentHash    []byte     `json:"-"`
}

// OneTimeToken represents an activation or password-reset token record stored as a SHA-256 hash.
type OneTimeToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
