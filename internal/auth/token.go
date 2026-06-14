package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"rotasavings/internal/domain"
)

// ErrInvalidToken is returned when a token is malformed, tampered, or expired.
var ErrInvalidToken = errors.New("invalid token")

// Token types distinguish short-lived access tokens from long-lived refresh
// tokens, so a refresh token can never be used to authorize a request.
const (
	TokenAccess  = "access"
	TokenRefresh = "refresh"
)

// Claims is the JWT payload we issue.
type Claims struct {
	Sub  string      `json:"sub"` // user id
	Role domain.Role `json:"role"`
	Typ  string      `json:"typ"` // access | refresh
	Exp  int64       `json:"exp"` // unix seconds
	Iat  int64       `json:"iat"`
}

// Issuer mints and verifies HS256 tokens with a shared secret.
type Issuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// NewIssuer returns an Issuer with separate access and refresh lifetimes.
func NewIssuer(secret string, accessTTL, refreshTTL time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Issue returns a signed short-lived access token for the given user.
func (i *Issuer) Issue(userID string, role domain.Role) (string, error) {
	return i.mint(userID, role, TokenAccess, i.accessTTL)
}

// IssueRefresh returns a signed long-lived refresh token.
func (i *Issuer) IssueRefresh(userID string, role domain.Role) (string, error) {
	return i.mint(userID, role, TokenRefresh, i.refreshTTL)
}

func (i *Issuer) mint(userID string, role domain.Role, typ string, ttl time.Duration) (string, error) {
	now := i.now()
	claims := Claims{Sub: userID, Role: role, Typ: typ, Iat: now.Unix(), Exp: now.Add(ttl).Unix()}
	header, _ := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	payload, _ := json.Marshal(claims)
	signingInput := b64(header) + "." + b64(payload)
	return signingInput + "." + i.sign(signingInput), nil
}

// Verify checks an access token's signature, type, and expiry.
func (i *Issuer) Verify(token string) (*Claims, error) { return i.verify(token, TokenAccess) }

// VerifyRefresh checks a refresh token's signature, type, and expiry.
func (i *Issuer) VerifyRefresh(token string) (*Claims, error) { return i.verify(token, TokenRefresh) }

func (i *Issuer) verify(token, wantType string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(i.sign(signingInput)), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Typ != wantType {
		return nil, ErrInvalidToken
	}
	if i.now().Unix() >= claims.Exp {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func (i *Issuer) sign(input string) string {
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
