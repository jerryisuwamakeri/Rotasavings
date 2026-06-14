package domain

import "time"

// EventType enumerates the reputation-affecting events emitted by the chain.
// Reputation is NOT an ML score: it is a deterministic aggregation of these
// events, fully auditable and portable across every group.
type EventType string

const (
	EventContributionMade   EventType = "ContributionMade"
	EventContributionMissed EventType = "ContributionMissed"
	EventPayoutReceived     EventType = "PayoutReceived"
	EventGroupExit          EventType = "GroupExit"
	EventGroupExpulsion     EventType = "GroupExpulsion"
)

// ReputationEvent is one immutable entry in the reputation ledger. ProofHash
// links it back to the on-chain event that produced it.
type ReputationEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	GroupID    string    `json:"group_id"`
	CycleIndex int       `json:"cycle_index"`
	Type       EventType `json:"type"`
	Amount     Money     `json:"amount"`
	ProofHash  string    `json:"proof_hash"`
	Timestamp  time.Time `json:"timestamp"`
}

// ReputationSummary is a deterministic fold over a user's events. Given the
// same event log it always yields the same summary — no model, no randomness.
type ReputationSummary struct {
	UserID              string `json:"user_id"`
	ContributionsMade   int    `json:"contributions_made"`
	ContributionsMissed int    `json:"contributions_missed"`
	PayoutsReceived     int    `json:"payouts_received"`
	GroupExits          int    `json:"group_exits"`
	Expulsions          int    `json:"expulsions"`
	TotalContributed    Money  `json:"total_contributed"`
	TotalMissed         Money  `json:"total_missed"`
	// ReliabilityRatio = made / (made + missed), in [0,1]; 1.0 when no obligations yet.
	ReliabilityRatio float64 `json:"reliability_ratio"`
}

// SummariseReputation folds an event log into a summary. The input need not be
// sorted; the result is order-independent.
func SummariseReputation(userID string, events []ReputationEvent) ReputationSummary {
	s := ReputationSummary{UserID: userID, ReliabilityRatio: 1.0}
	for _, e := range events {
		if e.UserID != userID {
			continue
		}
		switch e.Type {
		case EventContributionMade:
			s.ContributionsMade++
			s.TotalContributed += e.Amount
		case EventContributionMissed:
			s.ContributionsMissed++
			s.TotalMissed += e.Amount
		case EventPayoutReceived:
			s.PayoutsReceived++
		case EventGroupExit:
			s.GroupExits++
		case EventGroupExpulsion:
			s.Expulsions++
		}
	}
	if obligations := s.ContributionsMade + s.ContributionsMissed; obligations > 0 {
		s.ReliabilityRatio = float64(s.ContributionsMade) / float64(obligations)
	}
	return s
}
