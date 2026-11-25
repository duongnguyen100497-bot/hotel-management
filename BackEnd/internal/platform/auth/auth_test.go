package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
)

func TestPasswordHashAndPolicy(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"short1",
		"onlyletters",
		"12345678",
		"",
	}
	for _, pw := range invalid {
		if err := auth.ValidatePasswordPolicy(pw); err == nil {
			t.Errorf("expected ValidatePasswordPolicy(%q) to fail", pw)
		}
	}

	valid := "HotelPass2026!"
	if err := auth.ValidatePasswordPolicy(valid); err != nil {
		t.Fatalf("expected valid password to pass: %v", err)
	}

	hash, err := auth.HashPassword(valid)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := auth.VerifyPassword(valid, hash)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword valid: ok=%v err=%v", ok, err)
	}
	ok, err = auth.VerifyPassword("WrongPass2026!", hash)
	if err != nil || ok {
		t.Fatalf("VerifyPassword wrong password should return false: ok=%v err=%v", ok, err)
	}
}

func TestTokenManager_SignAndVerify(t *testing.T) {
	t.Parallel()

	baseNow := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(baseNow)
	tm := auth.NewTokenManager("test-secret-key-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour, clk)

	expected := auth.Principal{
		UserID:               uuid.New(),
		Kind:                 "customer",
		SessionID:            uuid.New(),
		AuthorizationVersion: 3,
	}

	token, exp, err := tm.SignAccessToken(expected)
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}
	if !exp.Equal(baseNow.Add(15 * time.Minute)) {
		t.Errorf("unexpected exp: %v", exp)
	}

	got, claims, err := tm.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if got != expected {
		t.Errorf("principal mismatch: got %+v want %+v", got, expected)
	}
	if claims.AuthVer != 3 {
		t.Errorf("expected AuthVer=3, got %d", claims.AuthVer)
	}

	// Tampered signature must fail.
	if _, _, err := tm.VerifyAccessToken(token + "tamper"); err == nil {
		t.Errorf("expected tampered token to fail")
	}

	// Expired token must fail.
	expiredClock := clock.NewFixed(baseNow.Add(16 * time.Minute))
	tmExpired := auth.NewTokenManager("test-secret-key-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour, expiredClock)
	if _, _, err := tmExpired.VerifyAccessToken(token); err == nil {
		t.Errorf("expected expired token to fail")
	}
}
