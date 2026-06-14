// Package store is the read cache. Per the architecture, Postgres (and this
// in-memory stand-in) is application state and caching ONLY — never the source
// of truth. Anything here can be rebuilt by replaying chain events.
package store

import (
	"context"

	"rotasavings/internal/domain"
)

// Page describes a slice of a list result.
type Page struct {
	Offset int
	Limit  int
}

// Store is the cache the API reads from and the indexer writes projections into.
type Store interface {
	// Health verifies the datastore is reachable (used by readiness probes).
	Health(ctx context.Context) error

	// --- users ---
	SaveUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	ListUsers(ctx context.Context, p Page) ([]*domain.User, int, error)
	ListUsersByKYC(ctx context.Context, status domain.KYCStatus) ([]*domain.User, error)

	// --- groups ---
	SaveGroup(ctx context.Context, g *domain.Group) error
	GetGroup(ctx context.Context, id string) (*domain.Group, error)
	ListGroups(ctx context.Context, p Page) ([]*domain.Group, int, error)

	// --- memberships ---
	SaveMembership(ctx context.Context, m *domain.Membership) error
	GetMembership(ctx context.Context, groupID, userID string) (*domain.Membership, error)
	ListMembershipsByGroup(ctx context.Context, groupID string) ([]*domain.Membership, error)
	ListMembershipsByUser(ctx context.Context, userID string) ([]*domain.Membership, error)

	// --- join requests & invitations ---
	SaveJoinRequest(ctx context.Context, j *domain.JoinRequest) error
	GetJoinRequest(ctx context.Context, id string) (*domain.JoinRequest, error)
	ListJoinRequestsByGroup(ctx context.Context, groupID string) ([]*domain.JoinRequest, error)
	SaveInvitation(ctx context.Context, i *domain.Invitation) error
	GetInvitation(ctx context.Context, id string) (*domain.Invitation, error)
	ListInvitationsByUser(ctx context.Context, userID string) ([]*domain.Invitation, error)

	// --- cycles ---
	SaveCycle(ctx context.Context, c *domain.Cycle) error
	GetCycle(ctx context.Context, groupID string, index int) (*domain.Cycle, error)
	ListCyclesByGroup(ctx context.Context, groupID string) ([]*domain.Cycle, error)

	// --- contributions ---
	SaveContribution(ctx context.Context, c *domain.Contribution) error
	GetContribution(ctx context.Context, groupID, userID string, cycle int) (*domain.Contribution, error)
	ListContributionsByCycle(ctx context.Context, groupID string, cycle int) ([]*domain.Contribution, error)

	// --- payouts ---
	SavePayout(ctx context.Context, p *domain.Payout) error
	ListPayoutsByGroup(ctx context.Context, groupID string) ([]*domain.Payout, error)

	// --- escrow ledger (append-only) ---
	AppendEscrowEntry(ctx context.Context, e *domain.EscrowEntry) error
	ListEscrowByGroup(ctx context.Context, groupID string) ([]*domain.EscrowEntry, error)

	// --- reputation ledger (append-only, projected from chain) ---
	AppendReputationEvent(ctx context.Context, e *domain.ReputationEvent) error
	ReputationEvents(ctx context.Context, userID string) ([]domain.ReputationEvent, error)
	// AllReputationEvents returns a page of every user's events, newest first
	// (the platform-wide transaction feed for operators).
	AllReputationEvents(ctx context.Context, p Page) ([]domain.ReputationEvent, int, error)

	// --- webhooks ---
	SaveWebhook(ctx context.Context, w *domain.Webhook) error
	GetWebhook(ctx context.Context, id string) (*domain.Webhook, error)
	ListWebhooks(ctx context.Context) ([]*domain.Webhook, error)
	DeleteWebhook(ctx context.Context, id string) error

	// --- notifications ---
	SaveNotification(ctx context.Context, n *domain.Notification) error
	ListNotificationsByUser(ctx context.Context, userID string) ([]*domain.Notification, error)

	// --- audit trail (append-only) ---
	AppendAudit(ctx context.Context, a *domain.AuditEntry) error
	ListAudit(ctx context.Context, p Page) ([]*domain.AuditEntry, int, error)
}
