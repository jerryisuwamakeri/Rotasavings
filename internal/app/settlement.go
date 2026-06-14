package app

import (
	"context"
	"fmt"

	"rotasavings/internal/chain"
	"rotasavings/internal/domain"
	"rotasavings/internal/payments"
)

// settleCycle disburses a cycle's collected pool to its payee, records the
// payout on-chain and in the escrow ledger, marks the cycle settled, and
// advances the group to SETTLEMENT/CLOSED if it was the last cycle.
//
// Defaults (missing contributions) are recorded by the scheduler BEFORE this is
// called on deadline; here we simply disburse whatever was collected, recording
// any shortfall.
func (s *Service) settleCycle(ctx context.Context, g *domain.Group, cycle *domain.Cycle) (*domain.Payout, error) {
	if cycle.Settled {
		return nil, fmt.Errorf("%w: cycle %d already settled", domain.ErrConflict, cycle.Index)
	}

	contribs, err := s.store.ListContributionsByCycle(ctx, g.ID, cycle.Index)
	if err != nil {
		return nil, err
	}
	var gross domain.Money
	for _, c := range contribs {
		gross += c.Amount
	}
	expected := g.ContributionAmount * domain.Money(len(g.Members))

	payee, err := s.store.GetUser(ctx, cycle.PayoutUser)
	if err != nil {
		return nil, err
	}

	// 1. Disburse via the payment rail.
	res, err := s.pay.Disburse(ctx, payments.DisburseRequest{
		GroupID: g.ID, CycleIndex: cycle.Index, PayeeUserID: payee.ID,
		Amount: gross, Destination: payee.WalletAddress,
	})
	if err != nil || !res.Success {
		return nil, fmt.Errorf("disburse payout: %w", errOr(err, res.Message))
	}

	// 2. Record the payout on-chain (emits PayoutReceived).
	if _, err := s.chain.RecordPayout(ctx, chain.PayoutEvent{
		GroupID: g.ID, PayeeUserID: payee.ID, CycleIndex: cycle.Index, Amount: gross,
	}); err != nil {
		return nil, fmt.Errorf("record payout on-chain: %w", err)
	}

	now := s.now().UTC()
	// 3. Debit escrow.
	_ = s.store.AppendEscrowEntry(ctx, &domain.EscrowEntry{
		ID: domain.NewID(), GroupID: g.ID, CycleIndex: cycle.Index, UserID: payee.ID,
		Direction: domain.LedgerDebit, Amount: gross,
		Reference: res.Reference, Memo: "payout", CreatedAt: now,
	})

	// 4. Persist the payout and mark the cycle settled.
	payout := &domain.Payout{
		ID: domain.NewID(), GroupID: g.ID, CycleIndex: cycle.Index, PayeeUserID: payee.ID,
		GrossAmount: gross, ExpectedAmount: expected, Shortfall: expected - gross,
		DisbursementRef: res.Reference, SettledAt: now,
	}
	if err := s.store.SavePayout(ctx, payout); err != nil {
		return nil, err
	}
	cycle.Settled = true
	if err := s.store.SaveCycle(ctx, cycle); err != nil {
		return nil, err
	}
	s.notifyUser(ctx, payee.ID, domain.NotifyPayoutSent,
		"Payout sent", fmt.Sprintf("You received the cycle %d payout for %s", cycle.Index, g.Name))

	// 5. Advance group lifecycle if this was the final cycle.
	if cycle.Index == g.TotalCycles-1 {
		_ = g.Transition(domain.GroupSettlement)
		_ = g.Transition(domain.GroupClosed)
		if err := s.store.SaveGroup(ctx, g); err != nil {
			return nil, err
		}
	}
	return payout, nil
}

// SettleCycle is the operator/organizer-triggered settlement of a specific cycle.
func (s *Service) SettleCycle(ctx context.Context, actorID, groupID string, index int) (*domain.Payout, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.OrganizerID != actorID {
		return nil, fmt.Errorf("%w: only the organizer can settle a cycle", domain.ErrForbidden)
	}
	cycle, err := s.store.GetCycle(ctx, groupID, index)
	if err != nil {
		return nil, err
	}
	return s.settleCycle(ctx, g, cycle)
}

// recordDefault records a member's missed contribution for a cycle on-chain
// (emits ContributionMissed via the indexer) and notifies them.
func (s *Service) recordDefault(ctx context.Context, g *domain.Group, cycle *domain.Cycle, userID string) error {
	proof := domain.Hash("default", g.ID, userID, cycle.GroupID)
	if _, err := s.chain.RecordDefault(ctx, chain.DefaultEvent{
		GroupID: g.ID, UserAddress: userID, CycleIndex: cycle.Index,
		MissedAmount: g.ContributionAmount, ProofHash: proof,
	}); err != nil {
		return err
	}
	s.notifyUser(ctx, userID, domain.NotifyDefaultRecorded,
		"Missed contribution", fmt.Sprintf("You missed cycle %d of %s", cycle.Index, g.Name))
	return nil
}
