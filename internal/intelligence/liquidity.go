package intelligence

import "rotasavings/internal/domain"

// MemberRisk pairs a member with their estimated default probability for the
// liquidity model.
type MemberRisk struct {
	UserID string  `json:"user_id"`
	Risk   float64 `json:"risk"`
}

// LiquidityAssessment is the stress predictor's output for one group: the
// expected shortfall for the current cycle and a collapse probability.
type LiquidityAssessment struct {
	GroupID             string       `json:"group_id"`
	CycleIndex          int          `json:"cycle_index"`
	ExpectedInflow      domain.Money `json:"expected_inflow"`     // members * contribution
	PredictedShortfall  domain.Money `json:"predicted_shortfall"` // sum(risk_i * contribution)
	EscrowBalance       domain.Money `json:"escrow_balance"`
	CollapseProbability float64      `json:"collapse_probability"` // P(pool can't cover the payout)
	Level               string       `json:"level"`                // "ok" | "stressed" | "critical"
}

// AssessLiquidity estimates whether a group's current cycle can fund its payout.
//
// The payout owed is the full expected inflow (every member's contribution).
// Predicted inflow is reduced by each member's default risk. If predicted
// available funds (current escrow + predicted contributions) fall short of the
// payout, the cycle is at risk. CollapseProbability approximates P(at least one
// of the riskiest members defaults enough to break the pool).
func AssessLiquidity(
	groupID string,
	cycleIndex int,
	contribution domain.Money,
	escrow domain.Money,
	members []MemberRisk,
) LiquidityAssessment {
	expected := contribution * domain.Money(len(members))

	var predictedShortfall domain.Money
	noDefault := 1.0
	for _, m := range members {
		r := clamp01(m.Risk)
		predictedShortfall += domain.Money(float64(contribution) * r)
		noDefault *= (1 - r)
	}
	collapse := 1 - noDefault

	a := LiquidityAssessment{
		GroupID:             groupID,
		CycleIndex:          cycleIndex,
		ExpectedInflow:      expected,
		PredictedShortfall:  predictedShortfall,
		EscrowBalance:       escrow,
		CollapseProbability: collapse,
	}

	// A payout needs `expected` funds. Predicted available = escrow + (expected - shortfall).
	predictedAvailable := escrow + expected - predictedShortfall
	switch {
	case predictedAvailable >= expected && collapse < 0.25:
		a.Level = "ok"
	case predictedAvailable >= expected*8/10 && collapse < 0.5:
		a.Level = "stressed"
	default:
		a.Level = "critical"
	}
	return a
}
