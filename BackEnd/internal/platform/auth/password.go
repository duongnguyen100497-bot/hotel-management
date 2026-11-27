package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"golang.org/x/crypto/argon2"
)

// PasswordParams defines Argon2id tuning parameters.
type PasswordParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultPasswordParams provides OWASP-recommended Argon2id defaults for interactive logins.
func DefaultPasswordParams() PasswordParams {
	return PasswordParams{
		Memory:      64 * 1024, // 64 MiB
		Iterations:  1,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// ValidatePasswordPolicy enforces the canonical password strength policy:
// at least 8 characters and at least 1 letter and 1 digit.
func ValidatePasswordPolicy(password string) error {
	if len([]rune(password)) < 8 {
		return apperror.Validation("invalid password", apperror.FieldError{
			Field:   "password",
			Message: "must be at least 8 characters long and include both letters and digits",
		})
	}
	if len(password) > 256 {
		return apperror.Validation("invalid password", apperror.FieldError{
			Field:   "password",
			Message: "must not exceed 256 bytes",
		})
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperror.Validation("invalid password", apperror.FieldError{
			Field:   "password",
			Message: "must include at least one letter and one digit",
		})
	}
	return nil
}

// HashPassword hashes a plaintext password with Argon2id and a fresh random salt,
// returning a PHC-formatted string: $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>.
func HashPassword(password string) (string, error) {
	return HashPasswordWithParams(password, DefaultPasswordParams())
}

// HashPasswordWithParams hashes a password with explicit Argon2id parameters.
func HashPasswordWithParams(password string, p PasswordParams) (string, error) {
	if p.SaltLength == 0 {
		p.SaltLength = 16
	}
	if p.KeyLength == 0 {
		p.KeyLength = 32
	}
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate argon2id salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword compares a plaintext password against a PHC-encoded Argon2id hash in constant time.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse argon2id version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("incompatible argon2id version %d", version)
	}

	var p PasswordParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return false, fmt.Errorf("parse argon2id params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode argon2id salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode argon2id hash: %w", err)
	}
	if len(expectedHash) == 0 {
		return false, fmt.Errorf("empty argon2id hash")
	}
	p.KeyLength = uint32(len(expectedHash))

	computed := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	if subtle.ConstantTimeCompare(computed, expectedHash) == 1 {
		return true, nil
	}
	return false, nil
}
