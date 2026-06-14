package intelligence

import (
	"time"

	"rotasavings/internal/domain"
)

// BehaviorFlagLevel grades how concerning a behavioral signal is.
type BehaviorFlagLevel string

const (
	FlagNone   BehaviorFlagLevel = "none"
	FlagWatch  BehaviorFlagLevel = "watch"
	FlagAtRisk BehaviorFlagLevel = "at_risk"
)

// BehaviorFlag is the behavioral-monitoring engine's verdict on one member in
// one active cycle. It warns; it never enforces (the chain does that).
type BehaviorFlag struct {
	UserID  string            `json:"user_id"`
	Level   BehaviorFlagLevel `json:"level"`
	Reasons []string          `json:"reasons"`
}

// MonitorMember inspects a member's contribution behavior for the current cycle
// and recent history, returning an early-warning flag.
//
// Signals (transparent heuristics):
//   - Unpaid as the cycle deadline approaches.
//   - A recent run of missed contributions in the reputation log.
//   - Declining reliability.
func MonitorMember(
	userID string,
	summary domain.ReputationSummary,
	recent []domain.ReputationEvent,
	cycle domain.Cycle,
	paidThisCycle bool,
	now time.Time,
) BehaviorFlag {
	flag := BehaviorFlag{UserID: userID, Level: FlagNone}

	// 1. Approaching deadline while unpaid.
	if !paidThisCycle && !cycle.Settled {
		window := cycle.Deadline.Sub(now)
		switch {
		case window <= 0:
			flag.escalate(FlagAtRisk, "contribution past deadline and unpaid")
		case window <= 24*time.Hour:
			flag.escalate(FlagAtRisk, "unpaid with <24h to deadline")
		case window <= 72*time.Hour:
			flag.escalate(FlagWatch, "unpaid with <72h to deadline")
		}
	}

	// 2. Recent consecutive misses.
	if streak := trailingMissStreak(recent); streak >= 2 {
		flag.escalate(FlagAtRisk, "two or more consecutive missed contributions")
	} else if streak == 1 {
		flag.escalate(FlagWatch, "missed the previous contribution")
	}

	// 3. Low overall reliability with enough history to be meaningful.
	if summary.ContributionsMade+summary.ContributionsMissed >= 3 && summary.ReliabilityRatio < 0.6 {
		flag.escalate(FlagWatch, "reliability below 60%")
	}

	return flag
}

// trailingMissStreak counts consecutive ContributionMissed events at the end of
// the chronologically-ordered event slice.
func trailingMissStreak(events []domain.ReputationEvent) int {
	streak := 0
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Type {
		case domain.EventContributionMissed:
			streak++
		case domain.EventContributionMade:
			return streak
		}
	}
	return streak
}

func (f *BehaviorFlag) escalate(level BehaviorFlagLevel, reason string) {
	f.Reasons = append(f.Reasons, reason)
	if rank(level) > rank(f.Level) {
		f.Level = level
	}
}

func rank(l BehaviorFlagLevel) int {
	switch l {
	case FlagAtRisk:
		return 2
	case FlagWatch:
		return 1
	default:
		return 0
	}
}
