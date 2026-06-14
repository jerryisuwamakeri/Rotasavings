package domain

import (
	"fmt"
	"strconv"
	"time"
)

// Role is the platform-level authorization role.
type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// UserStatus controls whether a user may act on the platform.
type UserStatus string

const (
	UserActive    UserStatus = "active"
	UserSuspended UserStatus = "suspended"
)

// KYCStatus is the off-chain KYC review state. Only an approved user may join
// or create groups.
type KYCStatus string

const (
	KYCPending  KYCStatus = "pending"
	KYCApproved KYCStatus = "approved"
	KYCRejected KYCStatus = "rejected"
)

// User is the off-chain projection of an on-chain identity commitment, plus the
// off-chain account data (auth, KYC, role) the platform needs to operate.
//
// The chain only ever stores the IdentityCommitment hash. PasswordHash and PII
// never go on-chain.
type User struct {
	ID                 string     `json:"id"`
	Email              string     `json:"email"`
	DisplayName        string     `json:"display_name"`
	WalletAddress      string     `json:"wallet_address"`
	IdentityCommitment string     `json:"identity_commitment"`
	KYCProvider        string     `json:"kyc_provider"`
	KYCStatus          KYCStatus  `json:"kyc_status"`
	Role               Role       `json:"role"`
	Status             UserStatus `json:"status"`
	PasswordHash       string     `json:"-"` // never serialized
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CanParticipate reports whether the user may join or create groups.
func (u *User) CanParticipate() error {
	if u.Status != UserActive {
		return fmt.Errorf("%w: account is %s", ErrForbidden, u.Status)
	}
	if u.KYCStatus != KYCApproved {
		return fmt.Errorf("%w: KYC is %s, must be approved", ErrForbidden, u.KYCStatus)
	}
	return nil
}

// ComputeIdentityCommitment mirrors the on-chain IdentityRegistry commitment:
//
//	H(user_id + KYC_provider_signature + timestamp)
func ComputeIdentityCommitment(userID, kycProviderSig string, ts time.Time) string {
	return Hash(userID, kycProviderSig, strconv.FormatInt(ts.Unix(), 10))
}
