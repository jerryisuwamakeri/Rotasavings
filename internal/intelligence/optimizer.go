package intelligence

import "sort"

// Candidate is a prospective group member with their estimated default risk.
type Candidate struct {
	UserID string  `json:"user_id"`
	Risk   float64 `json:"risk"` // probability of default in [0,1]
}

// ProposedGroup is one optimizer-suggested group.
type ProposedGroup struct {
	Members []string `json:"members"`
	AvgRisk float64  `json:"avg_risk"`
	// SurvivalScore in [0,1]: rough estimate of the group completing all cycles.
	SurvivalScore float64 `json:"survival_score"`
}

// OptimizeGroups partitions candidates into balanced groups of size groupSize.
//
// Strategy: sort by risk, then snake-draft across groups so each group gets a
// spread of high- and low-risk members. This balances risk across groups
// (rather than concentrating defaults in one) and keeps within-group risk
// correlation low. It's a transparent heuristic, not a solver — a starting
// point the operator can iterate on.
func OptimizeGroups(candidates []Candidate, groupSize int) []ProposedGroup {
	if groupSize <= 0 || len(candidates) < groupSize {
		return nil
	}

	sorted := make([]Candidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Risk < sorted[j].Risk })

	numGroups := len(sorted) / groupSize
	groups := make([][]Candidate, numGroups)

	// Snake draft: 0,1,..,n-1, n-1,..,1,0, ... so adjacent risk ranks land in
	// different groups.
	dir := 1
	g := 0
	for _, c := range sorted[:numGroups*groupSize] {
		groups[g] = append(groups[g], c)
		g += dir
		if g == numGroups {
			g, dir = numGroups-1, -1
		} else if g < 0 {
			g, dir = 0, 1
		}
	}

	out := make([]ProposedGroup, 0, numGroups)
	for _, members := range groups {
		out = append(out, summariseGroup(members))
	}
	return out
}

func summariseGroup(members []Candidate) ProposedGroup {
	ids := make([]string, 0, len(members))
	var sum, survival float64
	survival = 1.0
	for _, m := range members {
		ids = append(ids, m.UserID)
		sum += m.Risk
		// Independence approximation: group survives if no member defaults.
		survival *= (1 - m.Risk)
	}
	avg := 0.0
	if len(members) > 0 {
		avg = sum / float64(len(members))
	}
	return ProposedGroup{Members: ids, AvgRisk: avg, SurvivalScore: survival}
}
