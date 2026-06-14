package app

import (
	"context"
	"fmt"

	"rotasavings/internal/domain"
	"rotasavings/internal/intelligence"
	"rotasavings/internal/store"
)

// PlatformOverview is the admin dashboard's headline KPIs.
type PlatformOverview struct {
	Users         int                       `json:"users"`
	GroupsByState map[domain.GroupState]int `json:"groups_by_state"`
	TotalGroups   int                       `json:"total_groups"`
	PendingKYC    int                       `json:"pending_kyc"`
	TotalEscrow   domain.Money              `json:"total_escrow"`
	TotalPayouts  int                       `json:"total_payouts"`
}

// Overview computes platform-wide KPIs for the admin dashboard.
func (s *Service) Overview(ctx context.Context) (PlatformOverview, error) {
	_, userCount, err := s.store.ListUsers(ctx, store.Page{Limit: 1})
	if err != nil {
		return PlatformOverview{}, err
	}
	groups, groupCount, err := s.store.ListGroups(ctx, store.Page{Limit: 100000})
	if err != nil {
		return PlatformOverview{}, err
	}
	ov := PlatformOverview{
		Users:         userCount,
		TotalGroups:   groupCount,
		GroupsByState: map[domain.GroupState]int{},
	}
	for _, g := range groups {
		ov.GroupsByState[g.State]++
		entries, _ := s.store.ListEscrowByGroup(ctx, g.ID)
		ov.TotalEscrow += domain.EscrowBalance(entries)
		payouts, _ := s.store.ListPayoutsByGroup(ctx, g.ID)
		ov.TotalPayouts += len(payouts)
	}
	pending, _ := s.store.ListUsersByKYC(ctx, domain.KYCPending)
	ov.PendingKYC = len(pending)
	return ov, nil
}

// ListUsers returns a page of users (admin).
func (s *Service) ListUsers(ctx context.Context, p store.Page) ([]*domain.User, int, error) {
	return s.store.ListUsers(ctx, p)
}

// PendingKYC lists users awaiting KYC review.
func (s *Service) PendingKYC(ctx context.Context) ([]*domain.User, error) {
	return s.store.ListUsersByKYC(ctx, domain.KYCPending)
}

// DecideKYC approves or rejects a user's KYC (admin), auditing the action.
func (s *Service) DecideKYC(ctx context.Context, actorID, userID string, approve bool) (*domain.User, error) {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.KYCStatus != domain.KYCPending {
		return nil, fmt.Errorf("%w: KYC already %s", domain.ErrConflict, u.KYCStatus)
	}
	if approve {
		u.KYCStatus = domain.KYCApproved
	} else {
		u.KYCStatus = domain.KYCRejected
	}
	u.UpdatedAt = s.now().UTC()
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "kyc."+string(u.KYCStatus), userID, "")
	s.notifyUser(ctx, userID, domain.NotifyJoinDecision,
		"KYC "+string(u.KYCStatus), "Your identity verification was "+string(u.KYCStatus))
	return u, nil
}

// SetUserRole changes a user's platform role (admin), auditing the action.
func (s *Service) SetUserRole(ctx context.Context, actorID, userID string, role domain.Role) (*domain.User, error) {
	if role != domain.RoleMember && role != domain.RoleAdmin {
		return nil, fmt.Errorf("%w: invalid role", domain.ErrValidation)
	}
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	u.Role = role
	u.UpdatedAt = s.now().UTC()
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "user.role."+string(role), userID, "")
	return u, nil
}

// AdminUserDetail bundles a user with their groups and reputation for the
// operator's user-detail view.
type AdminUserDetail struct {
	User       *domain.User             `json:"user"`
	Groups     []MyGroupMembership      `json:"groups"`
	Reputation domain.ReputationSummary `json:"reputation"`
}

// UserDetail returns a full operator view of one user.
func (s *Service) UserDetail(ctx context.Context, userID string) (*AdminUserDetail, error) {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	groups, _ := s.MyGroups(ctx, userID)
	rep, _ := s.Reputation(ctx, userID)
	return &AdminUserDetail{User: u, Groups: groups, Reputation: rep}, nil
}

// AllGroups lists every group on the platform (admin), paginated.
func (s *Service) AllGroups(ctx context.Context, p store.Page) ([]*domain.Group, int, error) {
	return s.store.ListGroups(ctx, p)
}

// AllTransactions returns the platform-wide event ledger (admin), paginated.
func (s *Service) AllTransactions(ctx context.Context, p store.Page) ([]domain.ReputationEvent, int, error) {
	return s.store.AllReputationEvents(ctx, p)
}

// AdminSettleCycle force-settles a cycle as an operator, bypassing the
// organizer check (used for intervention). Audited.
func (s *Service) AdminSettleCycle(ctx context.Context, actorID, groupID string, index int) (*domain.Payout, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	cycle, err := s.store.GetCycle(ctx, groupID, index)
	if err != nil {
		return nil, err
	}
	p, err := s.settleCycle(ctx, g, cycle)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "group.force_settle", groupID, "cycle "+itoa(index))
	return p, nil
}

// --- webhooks (admin) ---

// CreateWebhook registers an operator webhook endpoint.
func (s *Service) CreateWebhook(ctx context.Context, actorID, url, secret string) (*domain.Webhook, error) {
	if url == "" {
		return nil, fmt.Errorf("%w: url is required", domain.ErrValidation)
	}
	w := &domain.Webhook{
		ID: domain.NewID(), URL: url, Secret: secret, Active: true,
		CreatedBy: actorID, CreatedAt: s.now().UTC(),
	}
	if err := s.store.SaveWebhook(ctx, w); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "webhook.create", w.ID, url)
	return w, nil
}

// ListWebhooks returns all registered webhooks (admin).
func (s *Service) ListWebhooks(ctx context.Context) ([]*domain.Webhook, error) {
	return s.store.ListWebhooks(ctx)
}

// DeleteWebhook removes a webhook (admin).
func (s *Service) DeleteWebhook(ctx context.Context, actorID, id string) error {
	if err := s.store.DeleteWebhook(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, actorID, "webhook.delete", id, "")
	return nil
}

// itoa is a tiny int->string helper (avoids strconv import churn here).
func itoa(i int) string { return fmt.Sprintf("%d", i) }

// SetUserStatus suspends or reactivates a user (admin), auditing the action.
func (s *Service) SetUserStatus(ctx context.Context, actorID, userID string, status domain.UserStatus) (*domain.User, error) {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	u.Status = status
	u.UpdatedAt = s.now().UTC()
	if err := s.store.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	s.audit(ctx, actorID, "user."+string(status), userID, "")
	return u, nil
}

// LiquidityBoard assesses liquidity across every active group for the operator
// dashboard.
func (s *Service) LiquidityBoard(ctx context.Context) ([]intelligence.LiquidityAssessment, error) {
	groups, _, err := s.store.ListGroups(ctx, store.Page{Limit: 100000})
	if err != nil {
		return nil, err
	}
	out := make([]intelligence.LiquidityAssessment, 0)
	for _, g := range groups {
		if g.State != domain.GroupActive {
			continue
		}
		a, err := s.AssessGroupLiquidity(ctx, g.ID)
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// AuditLog returns a page of the operator audit trail.
func (s *Service) AuditLog(ctx context.Context, p store.Page) ([]*domain.AuditEntry, int, error) {
	return s.store.ListAudit(ctx, p)
}

// SeedAdmin ensures an admin account exists (idempotent), used at boot.
func (s *Service) SeedAdmin(ctx context.Context, email, password, displayName string) error {
	if _, err := s.store.GetUserByEmail(ctx, email); err == nil {
		return nil // already exists
	}
	u, err := s.Register(ctx, RegisterInput{
		Email: email, Password: password, DisplayName: displayName,
		WalletAddress: "0xADMIN", KYCProvider: "platform", KYCSignature: "seed",
	})
	if err != nil {
		return err
	}
	u.Role = domain.RoleAdmin
	u.KYCStatus = domain.KYCApproved
	u.UpdatedAt = s.now().UTC()
	return s.store.SaveUser(ctx, u)
}
