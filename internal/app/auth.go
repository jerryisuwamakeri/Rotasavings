package app

import (
	"context"
	"fmt"
	"strings"

	"rotasavings/internal/auth"
	"rotasavings/internal/domain"
)

// RegisterInput is the payload to create a new account.
type RegisterInput struct {
	Email         string
	Password      string
	DisplayName   string
	WalletAddress string
	KYCProvider   string
	KYCSignature  string
}

// Register creates a member account (KYC pending), anchors the identity
// commitment on-chain, and caches the user. The user cannot join groups until
// an admin approves KYC.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || in.DisplayName == "" || in.WalletAddress == "" {
		return nil, fmt.Errorf("%w: email, display_name and wallet_address are required", domain.ErrValidation)
	}
	if _, err := s.store.GetUserByEmail(ctx, in.Email); err == nil {
		return nil, fmt.Errorf("%w: email already registered", domain.ErrConflict)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrValidation, err.Error())
	}

	now := s.now().UTC()
	id := domain.NewID()
	commitment := domain.ComputeIdentityCommitment(id, in.KYCSignature, now)
	if _, err := s.chain.RegisterIdentity(ctx, commitment); err != nil {
		return nil, fmt.Errorf("anchor identity on-chain: %w", err)
	}

	u := &domain.User{
		ID:                 id,
		Email:              in.Email,
		DisplayName:        in.DisplayName,
		WalletAddress:      in.WalletAddress,
		IdentityCommitment: commitment,
		KYCProvider:        in.KYCProvider,
		KYCStatus:          domain.KYCPending,
		Role:               domain.RoleMember,
		Status:             domain.UserActive,
		PasswordHash:       hash,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, fmt.Errorf("cache user: %w", err)
	}
	return u, nil
}

// TokenPair is an access token plus the refresh token used to renew it.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login verifies credentials and returns an access/refresh token pair plus user.
func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, *domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return TokenPair{}, nil, domain.ErrUnauthorized // do not leak which part failed
	}
	if !auth.VerifyPassword(u.PasswordHash, password) {
		return TokenPair{}, nil, domain.ErrUnauthorized
	}
	if u.Status != domain.UserActive {
		return TokenPair{}, nil, fmt.Errorf("%w: account is %s", domain.ErrForbidden, u.Status)
	}
	pair, err := s.issueTokens(u)
	if err != nil {
		return TokenPair{}, nil, err
	}
	return pair, u, nil
}

// Refresh exchanges a valid refresh token for a fresh token pair, re-checking
// that the account is still active and reflecting any role change.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := s.issuer.VerifyRefresh(refreshToken)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}
	u, err := s.store.GetUser(ctx, claims.Sub)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}
	if u.Status != domain.UserActive {
		return TokenPair{}, fmt.Errorf("%w: account is %s", domain.ErrForbidden, u.Status)
	}
	return s.issueTokens(u)
}

func (s *Service) issueTokens(u *domain.User) (TokenPair, error) {
	access, err := s.issuer.Issue(u.ID, u.Role)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.issuer.IssueRefresh(u.ID, u.Role)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// GetUser returns a user by id.
func (s *Service) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.store.GetUser(ctx, id)
}

// UpdateProfile updates mutable profile fields.
func (s *Service) UpdateProfile(ctx context.Context, userID, displayName string) (*domain.User, error) {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if displayName != "" {
		u.DisplayName = displayName
	}
	u.UpdatedAt = s.now().UTC()
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Reputation returns the deterministic summary folded from a user's event log.
func (s *Service) Reputation(ctx context.Context, userID string) (domain.ReputationSummary, error) {
	events, err := s.store.ReputationEvents(ctx, userID)
	if err != nil {
		return domain.ReputationSummary{}, err
	}
	return domain.SummariseReputation(userID, events), nil
}

// Notifications returns a user's notification feed.
func (s *Service) Notifications(ctx context.Context, userID string) ([]*domain.Notification, error) {
	return s.store.ListNotificationsByUser(ctx, userID)
}

// Transactions returns a user's on-chain reputation event ledger, newest first -
// their full contribution/payout/default history.
func (s *Service) Transactions(ctx context.Context, userID string) ([]domain.ReputationEvent, error) {
	events, err := s.store.ReputationEvents(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Reverse to newest-first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}
