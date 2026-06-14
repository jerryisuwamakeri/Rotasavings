package intelligence

import "testing"

func TestScoreDefaultRiskOrdering(t *testing.T) {
	safe := ScoreDefaultRisk(Features{
		Reliability: 1.0, PriorParticipations: 10, IncomeConsistency: 0.95, SpendingVolatility: 0.05,
	})
	risky := ScoreDefaultRisk(Features{
		Reliability: 0.2, PriorParticipations: 0, IncomeConsistency: 0.1, SpendingVolatility: 0.9, Expulsions: 1,
	})
	if !(safe.Probability < risky.Probability) {
		t.Fatalf("safe (%v) should be < risky (%v)", safe.Probability, risky.Probability)
	}
	if safe.Band != "low" {
		t.Fatalf("expected low band for safe profile, got %q (p=%v)", safe.Band, safe.Probability)
	}
	if risky.Band != "high" {
		t.Fatalf("expected high band for risky profile, got %q (p=%v)", risky.Band, risky.Probability)
	}
}

func TestOptimizeGroupsBalances(t *testing.T) {
	candidates := []Candidate{
		{"a", 0.10}, {"b", 0.80}, {"c", 0.20}, {"d", 0.70},
	}
	groups := OptimizeGroups(candidates, 2)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	// Snake draft should keep the two groups' average risk close together,
	// rather than dumping all high-risk members into one group.
	diff := groups[0].AvgRisk - groups[1].AvgRisk
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.10 {
		t.Fatalf("groups not balanced: avg risks %v vs %v", groups[0].AvgRisk, groups[1].AvgRisk)
	}
}

func TestOptimizeGroupsTooFew(t *testing.T) {
	if OptimizeGroups([]Candidate{{"a", 0.1}}, 2) != nil {
		t.Fatal("expected nil when fewer candidates than group size")
	}
}
