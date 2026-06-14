package chain

import (
	"context"
	"sync"
	"time"

	"rotasavings/internal/domain"
)

// MemChain is an in-memory TruthLayer for local development and tests. It fakes
// addresses and tx hashes with deterministic-looking digests and fans
// reputation events out to all live subscribers. It is NOT a blockchain — it
// makes no guarantees about persistence or consensus.
type MemChain struct {
	mu          sync.Mutex
	subscribers []chan domain.ReputationEvent
}

func NewMemChain() *MemChain { return &MemChain{} }

func (m *MemChain) RegisterIdentity(_ context.Context, commitment string) (string, error) {
	return txHash("identity", commitment), nil
}

func (m *MemChain) DeployGroup(_ context.Context, g domain.Group) (string, error) {
	// Fake a 20-byte EVM-style address from the group id.
	h := domain.Hash("group-contract", g.ID)
	return "0x" + h[2:42], nil
}

func (m *MemChain) ActivateGroup(_ context.Context, contractAddress string) (string, error) {
	return txHash("activate", contractAddress), nil
}

func (m *MemChain) SubmitContribution(_ context.Context, c domain.Contribution) (string, error) {
	tx := txHash("contribution", c.Commitment)
	m.emit(domain.ReputationEvent{
		ID:         domain.NewID(),
		UserID:     c.UserID,
		GroupID:    c.GroupID,
		CycleIndex: c.CycleIndex,
		Type:       domain.EventContributionMade,
		Amount:     c.Amount,
		ProofHash:  tx,
		Timestamp:  time.Now().UTC(),
	})
	return tx, nil
}

func (m *MemChain) RecordDefault(_ context.Context, ev DefaultEvent) (string, error) {
	tx := txHash("default", ev.ProofHash)
	m.emit(domain.ReputationEvent{
		ID:         domain.NewID(),
		UserID:     ev.UserAddress,
		GroupID:    ev.GroupID,
		CycleIndex: ev.CycleIndex,
		Type:       domain.EventContributionMissed,
		Amount:     ev.MissedAmount,
		ProofHash:  tx,
		Timestamp:  time.Now().UTC(),
	})
	return tx, nil
}

func (m *MemChain) Subscribe(ctx context.Context) (<-chan domain.ReputationEvent, error) {
	ch := make(chan domain.ReputationEvent, 64)
	m.mu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.mu.Unlock()

	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, sub := range m.subscribers {
			if sub == ch {
				m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}()
	return ch, nil
}

func (m *MemChain) RecordPayout(_ context.Context, ev PayoutEvent) (string, error) {
	tx := txHash("payout", ev.PayeeUserID)
	m.emit(domain.ReputationEvent{
		ID:         domain.NewID(),
		UserID:     ev.PayeeUserID,
		GroupID:    ev.GroupID,
		CycleIndex: ev.CycleIndex,
		Type:       domain.EventPayoutReceived,
		Amount:     ev.Amount,
		ProofHash:  tx,
		Timestamp:  time.Now().UTC(),
	})
	return tx, nil
}

func (m *MemChain) emit(ev domain.ReputationEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sub := range m.subscribers {
		select {
		case sub <- ev:
		default:
			// Drop on a full subscriber buffer rather than block the chain.
			// A real indexer would replay from block height on reconnect.
		}
	}
}

func txHash(kind, seed string) string {
	return domain.Hash("tx", kind, seed)
}
