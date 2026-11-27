package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
)

// Principal represents the authenticated caller attached to request context.
type Principal struct {
	UserID               uuid.UUID `json:"userId"`
	Kind                 string    `json:"kind"`
	SessionID            uuid.UUID `json:"sessionId"`
	AuthorizationVersion int       `json:"authorizationVersion"`
}

type principalContextKey struct{}

// WithPrincipal attaches a Principal to ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

// PrincipalFromContext extracts the authenticated Principal from ctx, if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(principalContextKey{}).(Principal)
	return p, ok && p.UserID != uuid.Nil
}

// AccessClaims holds the signed JWT access token payload.
// Permissions are intentionally NEVER embedded in the JWT so dynamic RBAC changes take effect immediately.
type AccessClaims struct {
	Sub     uuid.UUID `json:"sub"`
	Kind    string    `json:"kind"`
	SID     uuid.UUID `json:"sid"`
	AuthVer int       `json:"auth_ver"`
	Iat     int64     `json:"iat"`
	Exp     int64     `json:"exp"`
}

// TokenManager issues and verifies HMAC-SHA256 JWT access tokens and opaque random tokens.
type TokenManager struct {
	accessSecret []byte
	accessTTL    time.Duration
	refreshTTL   time.Duration
	clk          clock.Clock
}

// NewTokenManager constructs a TokenManager with the provided access secret and TTLs.
func NewTokenManager(accessSecret string, accessTTL, refreshTTL time.Duration, clk clock.Clock) *TokenManager {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	return &TokenManager{
		accessSecret: []byte(accessSecret),
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		clk:          clk,
	}
}

// AccessTTL returns the configured access token lifetime.
func (m *TokenManager) AccessTTL() time.Duration {
	return m.accessTTL
}

// RefreshTTL returns the configured refresh token lifetime.
func (m *TokenManager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

var jwtHeaderB64 = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// SignAccessToken mints an HS256 JWT access token for p.
func (m *TokenManager) SignAccessToken(p Principal) (token string, expiresAt time.Time, err error) {
	if len(m.accessSecret) == 0 {
		return "", time.Time{}, fmt.Errorf("empty jwt access secret")
	}
	now := m.clk.Now().UTC()
	exp := now.Add(m.accessTTL)
	claims := AccessClaims{
		Sub:     p.UserID,
		Kind:    p.Kind,
		SID:     p.SessionID,
		AuthVer: p.AuthorizationVersion,
		Iat:     now.Unix(),
		Exp:     exp.Unix(),
	}
	rawClaims, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal jwt claims: %w", err)
	}
	claimsB64 := base64.RawURLEncoding.EncodeToString(rawClaims)
	signingInput := jwtHeaderB64 + "." + claimsB64
	sig := m.computeHMAC(signingInput)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), exp, nil
}

// VerifyAccessToken validates an HS256 JWT signature and expiry, returning its Principal and claims.
func (m *TokenManager) VerifyAccessToken(rawToken string) (Principal, AccessClaims, error) {
	parts := strings.Split(strings.TrimSpace(rawToken), ".")
	if len(parts) != 3 {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("invalid access token")
	}
	if parts[0] != jwtHeaderB64 {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("invalid access token header")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := m.computeHMAC(signingInput)
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(actualSig, expectedSig) != 1 {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("invalid access token signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("invalid access token payload")
	}

	var claims AccessClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("malformed access token claims")
	}

	nowUnix := m.clk.Now().UTC().Unix()
	if claims.Exp <= nowUnix {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("access token expired")
	}
	if claims.Sub == uuid.Nil || claims.SID == uuid.Nil || claims.Kind == "" || claims.AuthVer < 1 {
		return Principal{}, AccessClaims{}, apperror.Unauthenticated("incomplete access token claims")
	}

	return Principal{
		UserID:               claims.Sub,
		Kind:                 claims.Kind,
		SessionID:            claims.SID,
		AuthorizationVersion: claims.AuthVer,
	}, claims, nil
}

func (m *TokenManager) computeHMAC(input string) []byte {
	mac := hmac.New(sha256.New, m.accessSecret)
	_, _ = mac.Write([]byte(input))
	return mac.Sum(nil)
}

// GenerateOpaqueToken generates a cryptographically random 32-byte base64url token and its SHA-256 hash.
func GenerateOpaqueToken() (rawToken string, tokenHash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate random token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashOpaqueToken(raw), nil
}

// HashOpaqueToken computes the SHA-256 hash of a raw token string.
func HashOpaqueToken(rawToken string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawToken)))
	return sum[:]
}

// HashIdentifier computes a non-reversible SHA-256 digest of a client IP or User-Agent string (or nil if empty).
func HashIdentifier(value string) []byte {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(trimmed))
	return sum[:]
}
