package auth

import (
	"testing"
	"time"

	"rotasavings/internal/domain"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected rejection of short password")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	iss := NewIssuer("secret", time.Hour, 24*time.Hour)
	tok, err := iss.Issue("user-1", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := iss.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "user-1" || claims.Role != domain.RoleAdmin {
		t.Fatalf("bad claims: %+v", claims)
	}
}

func TestTokenRejectsTamperAndExpiry(t *testing.T) {
	iss := NewIssuer("secret", time.Hour, 24*time.Hour)
	tok, _ := iss.Issue("user-1", domain.RoleMember)
	if _, err := iss.Verify(tok + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	if _, err := NewIssuer("other-secret", time.Hour, time.Hour).Verify(tok); err == nil {
		t.Fatal("token verified under wrong secret")
	}

	expired := NewIssuer("secret", -time.Minute, time.Hour)
	tok2, _ := expired.Issue("user-1", domain.RoleMember)
	if _, err := iss.Verify(tok2); err == nil {
		t.Fatal("expired token accepted")
	}
}
