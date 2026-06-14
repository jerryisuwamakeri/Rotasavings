// Package intelligence is the off-chain prediction/optimization engine. It is
// pure Go and uses transparent statistical heuristics rather than opaque ML, so
// every score is explainable. Critically, this layer ASSISTS decisions — it
// never controls truth. The chain enforces; this only advises.
package intelligence

import (
	"math"

	"rotasavings/internal/domain"
)

// Features are the inputs to the default-risk model. On-chain reputation is the
// backbone; the rest are off-chain signals (bank/mobile-money derived) that the
// caller supplies, already normalised to [0,1] where noted.
type Features struct {
	// Reliability is the user's deterministic on-chain reliability ratio [0,1].
	Reliability float64
	// PriorParticipations counts completed ROSCA cycles (more history = safer).
	PriorParticipations int
	// IncomeConsistency in [0,1]: 1 = perfectly regular income.
	IncomeConsistency float64
	// SpendingVolatility in [0,1]: 0 = stable, 1 = highly volatile.
	SpendingVolatility float64
	// Expulsions is the count of prior group expulsions (strong negative signal).
	Expulsions int
}

// FeaturesFromReputation seeds the on-chain-derived fields from a reputation
// summary. Off-chain fields are left at their zero values for the caller to set.
func FeaturesFromReputation(s domain.ReputationSummary) Features {
	return Features{
		Reliability:         s.ReliabilityRatio,
		PriorParticipations: s.PayoutsReceived,
		Expulsions:          s.Expulsions,
	}
}

// RiskScore is the model output: a default probability plus the band it falls in.
type RiskScore struct {
	// Probability of default in the next cycle, in [0,1].
	Probability float64 `json:"probability"`
	Band        string  `json:"band"` // "low" | "medium" | "high"
}

// weights for the logistic risk model. Positive weights increase risk.
// Kept explicit and in one place so the model stays auditable and tunable.
var w = struct {
	Bias               float64
	Reliability        float64
	IncomeConsistency  float64
	SpendingVolatility float64
	History            float64 // applied to a saturating transform of participations
	Expulsion          float64
}{
	Bias:               -0.4,
	Reliability:        -3.2, // high reliability strongly lowers risk
	IncomeConsistency:  -1.5,
	SpendingVolatility: 2.0,
	History:            -1.2,
	Expulsion:          1.8, // each expulsion sharply raises risk
}

// ScoreDefaultRisk estimates the probability a user defaults in the next cycle.
// It is a logistic regression over the features — deterministic and explainable.
func ScoreDefaultRisk(f Features) RiskScore {
	// Saturating history signal: diminishing returns after a handful of cycles.
	history := 1 - math.Exp(-float64(f.PriorParticipations)/3.0)

	z := w.Bias +
		w.Reliability*clamp01(f.Reliability) +
		w.IncomeConsistency*clamp01(f.IncomeConsistency) +
		w.SpendingVolatility*clamp01(f.SpendingVolatility) +
		w.History*history +
		w.Expulsion*float64(f.Expulsions)

	p := sigmoid(z)
	return RiskScore{Probability: p, Band: band(p)}
}

func sigmoid(z float64) float64 { return 1 / (1 + math.Exp(-z)) }

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func band(p float64) string {
	switch {
	case p < 0.20:
		return "low"
	case p < 0.50:
		return "medium"
	default:
		return "high"
	}
}
