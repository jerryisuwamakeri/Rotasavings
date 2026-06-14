// Package payments is the off-chain execution layer: it moves real money in and
// out via a provider (mobile money / bank / card). Per the architecture, this
// layer NEVER decides truth — it executes instructions and reports results that
// the orchestration layer reconciles against on-chain state.
package payments

import (
	"context"

	"rotasavings/internal/domain"
)

// ChargeRequest collects a contribution from a member.
type ChargeRequest struct {
	GroupID    string
	UserID     string
	CycleIndex int
	Amount     domain.Money
	// Source identifies the funding instrument (e.g. a mobile-money MSISDN). In
	// dev it is opaque; real providers validate it.
	Source string
}

// DisburseRequest pays the pooled funds out to a cycle's payee.
type DisburseRequest struct {
	GroupID     string
	CycleIndex  int
	PayeeUserID string
	Amount      domain.Money
	// Destination is the payee's payout instrument.
	Destination string
}

// Result is a provider response. Reference is the provider's transaction id used
// for reconciliation and stored on the escrow ledger.
type Result struct {
	Reference string
	Success   bool
	Message   string
}

// Provider is the payment rail abstraction. M-Pesa, MTN MoMo, Paystack,
// Flutterwave, or a bank adapter each implement this.
type Provider interface {
	Charge(ctx context.Context, req ChargeRequest) (Result, error)
	Disburse(ctx context.Context, req DisburseRequest) (Result, error)
	Name() string
}
