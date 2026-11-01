package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Authenticator handles password validation and JWT token issuance.
type Authenticator struct {
	secretKey    []byte
	passwordHash string
	salt         string
}

// TokenClaims represents the payload embedded in a signed session token.
type TokenClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// NewAuthenticator initializes the authenticator with a master password and HMAC signing key.
func NewAuthenticator(masterPassword string, signingSecret string) *Authenticator {
	salt := generateSalt(16)
	hash := hashPassword(masterPassword, salt)
	return &Authenticator{
		secretKey:    []byte(signingSecret),
		passwordHash: hash,
		salt:         salt,
	}
}

// VerifyPassword checks if the input password matches the stored salted hash.
func (a *Authenticator) VerifyPassword(inputPassword string) bool {
	candidateHash := hashPassword(inputPassword, a.salt)
	return hmac.Equal([]byte(candidateHash), []byte(a.passwordHash))
}

// IssueToken creates a signed, base64-encoded JWT session token valid for duration.
func (a *Authenticator) IssueToken(duration time.Duration) (string, int64, error) {
	exp := time.Now().Add(duration).Unix()
	claims := TokenClaims{
		Subject:   "admin",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: exp,
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := header + "." + payload

	h := hmac.New(sha256.New, a.secretKey)
	h.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	token := signingInput + "." + signature
	return token, exp, nil
}

// ValidateToken verifies the cryptographic signature and expiration of a JWT session token.
func (a *Authenticator) ValidateToken(tokenStr string) (*TokenClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token structure")
	}

	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	h := hmac.New(sha256.New, a.secretKey)
	h.Write([]byte(signingInput))
	expectedSig := h.Sum(nil)

	if !hmac.Equal(sigBytes, expectedSig) {
		return nil, errors.New("invalid token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var claims TokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("session token expired")
	}

	return &claims, nil
}

func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + ":" + password))
	return hex.EncodeToString(h.Sum(nil))
}

func generateSalt(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
