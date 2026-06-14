// Package auth provides password hashing, token issuance/verification, and the
// HTTP middleware that enforces authentication and role-based access control.
//
// It is stdlib-only: PBKDF2 for password hashing (crypto/pbkdf2, Go 1.24+) and
// a hand-rolled HS256 JWT. In production you would likely swap PBKDF2 for
// argon2id and the JWT for your IdP — both live behind small functions here.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	pbkdfIterations = 120_000
	pbkdfKeyLen     = 32
	saltLen         = 16
)

// HashPassword returns an encoded PBKDF2 hash of the form
// "pbkdf2$<iter>$<salt_b64>$<hash_b64>" suitable for storage.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, pbkdfIterations, pbkdfKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2$%d$%s$%s",
		pbkdfIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash, using a
// constant-time comparison.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false
	}
	var iter int
	if _, err := fmt.Sscanf(parts[1], "%d", &iter); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
