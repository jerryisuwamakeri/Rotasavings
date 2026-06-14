package app

import (
	"context"
	"errors"
	"fmt"

	"rotasavings/internal/domain"
	"rotasavings/internal/payments"
)

// ActivateGroup fixes the payout order, materialises the cycle schedule, moves
// the group CREATED -> ACTIVE on-chain, and notifies members. If payoutOrder is
// empty, members are paid in join order.
func (s *Service) ActivateGroup(ctx context.Context, actorID, groupID string, payoutOrder []string) (*domain.Group, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.OrganizerID != actorID {
		return nil, fmt.Errorf("%w: only the organizer can activate the group", domain.ErrForbidden)
	}
	if !g.CanTransition(domain.GroupActive) {
		return nil, fmt.Errorf("%w: %s -> ACTIVE", domain.ErrInvalidTransition, g.State)
	}

	if len(payoutOrder) == 0 {
		payoutOrder = append([]string(nil), g.Members...)
	}
	g.PayoutOrder = payoutOrder
	g.TotalCycles = len(payoutOrder)
	if err := g.Validate(); err != nil {
		return nil, err
	}

	if _, err := s.chain.ActivateGroup(ctx, g.ContractAddress); err != nil {
		return nil, fmt.Errorf("activate group on-chain: %w", err)
	}
	now := s.now().UTC()
	g.ActivatedAt = &now
	if err := g.Transition(domain.GroupActive); err != nil {
		return nil, err
	}
	if err := s.store.SaveGroup(ctx, g); err != nil {
		return nil, err
	}

	// Materialise and persist the immutable cycle schedule.
	for _, c := range g.Cycles(now) {
		cc := c
		if err := s.store.SaveCycle(ctx, &cc); err != nil {
			return nil, err
		}
	}
	for _, uid := range g.Members {
		s.notifyUser(ctx, uid, domain.NotifyGroupActivated,
			"Group activated", fmt.Sprintf("%s is now active — contributions begin", g.Name))
	}
	return g, nil
}

// ListCycles returns the full cycle schedule for a group.
func (s *Service) ListCycles(ctx context.Context, groupID string) ([]*domain.Cycle, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.store.ListCyclesByGroup(ctx, groupID)
}

// CycleMemberStatus is one member's contribution standing within a cycle.
type CycleMemberStatus struct {
	UserID string `json:"user_id"`
	Paid   bool   `json:"paid"`
}

// CycleStatus is the detailed standing of a cycle: who has paid, who hasn't.
type CycleStatus struct {
	Cycle     domain.Cycle        `json:"cycle"`
	Members   []CycleMemberStatus `json:"members"`
	Collected domain.Money        `json:"collected"`
	Expected  domain.Money        `json:"expected"`
}

// CurrentCycle returns the first unsettled cycle of an active group.
func (s *Service) CurrentCycle(ctx context.Context, groupID string) (*domain.Cycle, error) {
	cycles, err := s.store.ListCyclesByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, c := range cycles {
		if !c.Settled {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}

// CycleStatus reports the per-member contribution standing for a cycle.
func (s *Service) CycleStatus(ctx context.Context, groupID string, index int) (*CycleStatus, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	cycle, err := s.store.GetCycle(ctx, groupID, index)
	if err != nil {
		return nil, err
	}
	contribs, err := s.store.ListContributionsByCycle(ctx, groupID, index)
	if err != nil {
		return nil, err
	}
	paid := make(map[string]bool, len(contribs))
	var collected domain.Money
	for _, c := range contribs {
		paid[c.UserID] = true
		collected += c.Amount
	}
	out := &CycleStatus{
		Cycle:     *cycle,
		Collected: collected,
		Expected:  g.ContributionAmount * domain.Money(len(g.Members)),
	}
	for _, uid := range g.Members {
		out.Members = append(out.Members, CycleMemberStatus{UserID: uid, Paid: paid[uid]})
	}
	return out, nil
}

// SubmitContribution charges the member via the payment provider, reveals the
// contribution commitment on-chain, and credits the group escrow. The
// ContributionMade reputation event flows back through the indexer.
func (s *Service) SubmitContribution(ctx context.Context, userID, groupID string, cycleIdx int, source string) (*domain.Contribution, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.State != domain.GroupActive {
		return nil, fmt.Errorf("%w: group is %s, not ACTIVE", domain.ErrConflict, g.State)
	}
	if !g.HasMember(userID) {
		return nil, fmt.Errorf("%w: not a member of this group", domain.ErrForbidden)
	}
	cycle, err := s.store.GetCycle(ctx, groupID, cycleIdx)
	if err != nil {
		return nil, err
	}
	if cycle.Settled {
		return nil, fmt.Errorf("%w: cycle %d already settled", domain.ErrConflict, cycleIdx)
	}
	if _, err := s.store.GetContribution(ctx, groupID, userID, cycleIdx); err == nil {
		return nil, fmt.Errorf("%w: already contributed to cycle %d", domain.ErrConflict, cycleIdx)
	}

	// 1. Execute the money movement (off-chain rail).
	res, err := s.pay.Charge(ctx, payments.ChargeRequest{
		GroupID: groupID, UserID: userID, CycleIndex: cycleIdx,
		Amount: g.ContributionAmount, Source: source,
	})
	if err != nil || !res.Success {
		return nil, fmt.Errorf("charge contribution: %w", errOr(err, res.Message))
	}

	// 2. Reveal the commitment on-chain (the truth).
	c := &domain.Contribution{
		ID: domain.NewID(), GroupID: groupID, UserID: userID, CycleIndex: cycleIdx,
		Amount:     g.ContributionAmount,
		Commitment: domain.ComputeContributionCommitment(userID, groupID, cycleIdx, g.ContributionAmount),
	}
	if _, err := s.chain.SubmitContribution(ctx, *c); err != nil {
		return nil, fmt.Errorf("submit contribution on-chain: %w", err)
	}
	now := s.now().UTC()
	c.Revealed = true
	c.PaidAt = &now
	if err := s.store.SaveContribution(ctx, c); err != nil {
		return nil, err
	}

	// 3. Credit the group escrow ledger.
	_ = s.store.AppendEscrowEntry(ctx, &domain.EscrowEntry{
		ID: domain.NewID(), GroupID: groupID, CycleIndex: cycleIdx, UserID: userID,
		Direction: domain.LedgerCredit, Amount: g.ContributionAmount,
		Reference: res.Reference, Memo: "contribution", CreatedAt: now,
	})
	s.notifyUser(ctx, userID, domain.NotifyContributionMade,
		"Contribution received", fmt.Sprintf("Your contribution to %s (cycle %d) was recorded", g.Name, cycleIdx))

	// 4. If everyone has now paid, settle early.
	if s.allContributed(ctx, g, cycleIdx) {
		if _, err := s.settleCycle(ctx, g, cycle); err != nil {
			// Settlement failure must not lose the contribution; log via notify.
			s.notifyUser(ctx, g.OrganizerID, domain.NotifyRiskWarning,
				"Settlement deferred", fmt.Sprintf("cycle %d could not auto-settle: %v", cycleIdx, err))
		}
	}
	return c, nil
}

func (s *Service) allContributed(ctx context.Context, g *domain.Group, cycleIdx int) bool {
	contribs, err := s.store.ListContributionsByCycle(ctx, g.ID, cycleIdx)
	if err != nil {
		return false
	}
	return len(contribs) >= len(g.Members)
}

func errOr(err error, msg string) error {
	if err != nil {
		return err
	}
	return errors.New(msg)
}
