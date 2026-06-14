// Package chain is the boundary to the blockchain truth layer.
//
// The TruthLayer interface is the ONLY source of truth in the system. The Go
// backend, Postgres, and payment rails are all downstream of it: they execute
// and cache, they never decide. Today we ship an in-memory implementation for
// local development; an EVM implementation (go-ethereum bound to the
// IdentityRegistry / RotasavingsGroup / ReputationLedger contracts) drops in
// behind the same interface without touching callers.
package chain

import (
	"context"

	"rotasavings/internal/domain"
)

// DefaultEvent is the deterministic on-chain record of a missed contribution.
// Mirrors the contract event of the same name.
type DefaultEvent struct {
	GroupID      string
	UserAddress  string
	CycleIndex   int
	MissedAmount domain.Money
	ProofHash    string
}

// PayoutEvent is the on-chain record of a settled cycle's disbursement.
type PayoutEvent struct {
	GroupID     string
	PayeeUserID string
	CycleIndex  int
	Amount      domain.Money
}

// TruthLayer is the on-chain system of truth. Every method that mutates state
// returns the transaction hash that committed it.
type TruthLayer interface {
	// RegisterIdentity anchors an identity commitment in the IdentityRegistry.
	RegisterIdentity(ctx context.Context, commitment string) (txHash string, err error)

	// DeployGroup deploys a RotasavingsGroup contract and returns its address.
	DeployGroup(ctx context.Context, g domain.Group) (contractAddress string, err error)

	// ActivateGroup moves a deployed group CREATED -> ACTIVE on-chain.
	ActivateGroup(ctx context.Context, contractAddress string) (txHash string, err error)

	// SubmitContribution reveals and records a contribution commitment.
	SubmitContribution(ctx context.Context, c domain.Contribution) (txHash string, err error)

	// RecordDefault emits a DefaultEvent for a missed contribution.
	RecordDefault(ctx context.Context, ev DefaultEvent) (txHash string, err error)

	// RecordPayout emits a PayoutReceived event when a cycle settles.
	RecordPayout(ctx context.Context, ev PayoutEvent) (txHash string, err error)

	// Subscribe streams reputation-affecting events for the indexer to project
	// into the read cache. The channel closes when ctx is cancelled.
	Subscribe(ctx context.Context) (<-chan domain.ReputationEvent, error)
}
