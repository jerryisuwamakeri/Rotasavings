package app

import (
	"context"

	"rotasavings/internal/domain"
	"rotasavings/internal/intelligence"
)

// ScoreRisk blends a user's on-chain reputation with caller-supplied off-chain
// signals into a default-risk score.
func (s *Service) ScoreRisk(ctx context.Context, userID string, offchain intelligence.Features) (intelligence.RiskScore, error) {
	summary, err := s.Reputation(ctx, userID)
	if err != nil {
		return intelligence.RiskScore{}, err
	}
	f := intelligence.FeaturesFromReputation(summary)
	f.IncomeConsistency = offchain.IncomeConsistency
	f.SpendingVolatility = offchain.SpendingVolatility
	if offchain.PriorParticipations > f.PriorParticipations {
		f.PriorParticipations = offchain.PriorParticipations
	}
	return intelligence.ScoreDefaultRisk(f), nil
}

// memberRisk estimates a member's default risk from on-chain reputation alone
// (off-chain signals default to neutral). Used by the liquidity/monitoring
// engines where only the user id is known.
func (s *Service) memberRisk(ctx context.Context, userID string) float64 {
	summary, err := s.Reputation(ctx, userID)
	if err != nil {
		return 0.5
	}
	f := intelligence.FeaturesFromReputation(summary)
	f.IncomeConsistency = 0.5
	f.SpendingVolatility = 0.5
	return intelligence.ScoreDefaultRisk(f).Probability
}

// MonitorGroup runs the behavioral-monitoring engine over the current cycle of
// an active group, returning an early-warning flag per member.
func (s *Service) MonitorGroup(ctx context.Context, groupID string) ([]intelligence.BehaviorFlag, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	cycle, err := s.CurrentCycle(ctx, groupID)
	if err != nil {
		return nil, err
	}
	contribs, err := s.store.ListContributionsByCycle(ctx, groupID, cycle.Index)
	if err != nil {
		return nil, err
	}
	paid := make(map[string]bool, len(contribs))
	for _, c := range contribs {
		paid[c.UserID] = true
	}
	now := s.now().UTC()
	flags := make([]intelligence.BehaviorFlag, 0, len(g.Members))
	for _, uid := range g.Members {
		summary, _ := s.Reputation(ctx, uid)
		events, _ := s.store.ReputationEvents(ctx, uid)
		flags = append(flags, intelligence.MonitorMember(uid, summary, events, *cycle, paid[uid], now))
	}
	return flags, nil
}

// AssessGroupLiquidity runs the liquidity-stress predictor for a group's
// current cycle.
func (s *Service) AssessGroupLiquidity(ctx context.Context, groupID string) (intelligence.LiquidityAssessment, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return intelligence.LiquidityAssessment{}, err
	}
	cycleIdx := 0
	if cur, err := s.CurrentCycle(ctx, groupID); err == nil {
		cycleIdx = cur.Index
	}
	entries, err := s.store.ListEscrowByGroup(ctx, groupID)
	if err != nil {
		return intelligence.LiquidityAssessment{}, err
	}
	risks := make([]intelligence.MemberRisk, 0, len(g.Members))
	for _, uid := range g.Members {
		risks = append(risks, intelligence.MemberRisk{UserID: uid, Risk: s.memberRisk(ctx, uid)})
	}
	return intelligence.AssessLiquidity(groupID, cycleIdx, g.ContributionAmount, domain.EscrowBalance(entries), risks), nil
}
