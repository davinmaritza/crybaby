package auth

import (
	"testing"
	"time"
)

func TestPasswordVerification(t *testing.T) {
	password := "superSecret123"
	auth := NewAuthenticator(password, "signingSecretKey")

	if !auth.VerifyPassword(password) {
		t.Errorf("expected valid password verification, got false")
	}

	if auth.VerifyPassword("wrongPassword") {
		t.Errorf("expected wrong password to fail verification, got true")
	}
}

func TestTokenLifecycle(t *testing.T) {
	auth := NewAuthenticator("password123", "cryptoSigningSecret32BytesLong!")

	token, exp, err := auth.IssueToken(2 * time.Second)
	if err != nil {
		t.Fatalf("failed to issue token: %v", err)
	}

	if exp <= time.Now().Unix() {
		t.Fatalf("token expiration should be in the future")
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("expected valid token validation, got error: %v", err)
	}

	if claims.Subject != "admin" {
		t.Errorf("expected claims subject 'admin', got '%s'", claims.Subject)
	}

	// Test invalid token signature
	tamperedToken := token + "tampered"
	_, err = auth.ValidateToken(tamperedToken)
	if err == nil {
		t.Errorf("expected tampered token to fail validation, got nil error")
	}
}
