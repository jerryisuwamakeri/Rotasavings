// Package app is the orchestration layer: it sequences truth-layer writes,
// payment execution, and cache updates. The ordering rule is absolute — the
// chain is authoritative; the cache is a follower; payments execute the chain's
// intent and are reconciled into the escrow ledger.
package app

import (
	"context"
	"time"

	"rotasavings/internal/auth"
	"rotasavings/internal/chain"
	"rotasavings/internal/domain"
	"rotasavings/internal/notify"
	"rotasavings/internal/payments"
	"rotasavings/internal/store"
	"rotasavings/internal/webhooks"
)

// Service wires every dependency the orchestration layer needs.
type Service struct {
	chain    chain.TruthLayer
	store    store.Store
	pay      payments.Provider
	notifier notify.Notifier
	issuer   *auth.Issuer
	hooks    *webhooks.Dispatcher
	now      func() time.Time
}

// Deps bundles Service dependencies for construction.
type Deps struct {
	Chain    chain.TruthLayer
	Store    store.Store
	Payments payments.Provider
	Notifier notify.Notifier
	Issuer   *auth.Issuer
	Webhooks *webhooks.Dispatcher
}

// NewService constructs the orchestration service.
func NewService(d Deps) *Service {
	return &Service{
		chain:    d.Chain,
		store:    d.Store,
		pay:      d.Payments,
		notifier: d.Notifier,
		issuer:   d.Issuer,
		hooks:    d.Webhooks,
		now:      time.Now,
	}
}

// SetClock overrides the time source (used in tests).
func (s *Service) SetClock(f func() time.Time) { s.now = f }

// Health reports whether the orchestration layer's datastore is reachable.
func (s *Service) Health(ctx context.Context) error { return s.store.Health(ctx) }

// --- shared helpers ---

// notifyUser persists and delivers a notification, swallowing delivery errors
// (notifications are best-effort and must never fail a core operation).
func (s *Service) notifyUser(ctx context.Context, userID string, kind domain.NotificationKind, title, body string) {
	n := domain.Notification{
		ID:        domain.NewID(),
		UserID:    userID,
		Kind:      kind,
		Title:     title,
		Body:      body,
		CreatedAt: s.now().UTC(),
	}
	_ = s.store.SaveNotification(ctx, &n)
	_ = s.notifier.Notify(ctx, n)
	if s.hooks != nil {
		s.hooks.Dispatch(ctx, domain.WebhookEvent{
			ID:        n.ID,
			Type:      string(kind),
			UserID:    userID,
			Data:      map[string]any{"title": title, "body": body},
			Timestamp: n.CreatedAt,
		})
	}
}

// audit appends an entry to the operator audit trail.
func (s *Service) audit(ctx context.Context, actorID, action, target, detail string) {
	_ = s.store.AppendAudit(ctx, &domain.AuditEntry{
		ID:        domain.NewID(),
		ActorID:   actorID,
		Action:    action,
		Target:    target,
		Detail:    detail,
		CreatedAt: s.now().UTC(),
	})
}
