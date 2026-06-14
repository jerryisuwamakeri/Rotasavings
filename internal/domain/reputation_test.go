package domain

import (
	"math"
	"testing"
	"time"
)

func TestSummariseReputation(t *testing.T) {
	now := time.Now()
	events := []ReputationEvent{
		{UserID: "u", Type: EventContributionMade, Amount: 100, Timestamp: now},
		{UserID: "u", Type: EventContributionMade, Amount: 100, Timestamp: now},
		{UserID: "u", Type: EventContributionMissed, Amount: 100, Timestamp: now},
		{UserID: "u", Type: EventPayoutReceived, Timestamp: now},
		{UserID: "u", Type: EventGroupExpulsion, Timestamp: now},
		{UserID: "other", Type: EventContributionMade, Amount: 999, Timestamp: now}, // ignored
	}
	s := SummariseReputation("u", events)

	if s.ContributionsMade != 2 || s.ContributionsMissed != 1 {
		t.Fatalf("bad counts: %+v", s)
	}
	if s.TotalContributed != 200 || s.TotalMissed != 100 {
		t.Fatalf("bad totals: %+v", s)
	}
	if s.PayoutsReceived != 1 || s.Expulsions != 1 {
		t.Fatalf("bad event tallies: %+v", s)
	}
	if math.Abs(s.ReliabilityRatio-2.0/3.0) > 1e-9 {
		t.Fatalf("reliability = %v, want 2/3", s.ReliabilityRatio)
	}
}

func TestSummariseReputationNoObligations(t *testing.T) {
	s := SummariseReputation("u", nil)
	if s.ReliabilityRatio != 1.0 {
		t.Fatalf("a user with no obligations should be perfectly reliable, got %v", s.ReliabilityRatio)
	}
}
