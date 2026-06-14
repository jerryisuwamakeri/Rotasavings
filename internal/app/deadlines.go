package app

import (
	"context"
	"time"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

// ProcessDeadlines is the enforcement heartbeat invoked by the scheduler. For
// every active group it advances any cycle whose deadline has passed: members
// who did not contribute are recorded as defaults on-chain, then the cycle is
// settled with whatever was collected. It returns the number of cycles settled.
//
// It is safe to call repeatedly; already-settled cycles are skipped.
func (s *Service) ProcessDeadlines(ctx context.Context, now time.Time) (int, error) {
	groups, _, err := s.store.ListGroups(ctx, store.Page{Limit: 100000})
	if err != nil {
		return 0, err
	}
	settled := 0
	for _, g := range groups {
		if g.State != domain.GroupActive {
			continue
		}
		// Drain all overdue cycles for this group in deadline order.
		for guard := 0; guard <= g.TotalCycles; guard++ {
			cycle, err := s.CurrentCycle(ctx, g.ID)
			if err != nil {
				break // no unsettled cycle left
			}
			if !cycle.IsOverdue(now) {
				break // current cycle still open
			}
			if err := s.settleOverdueCycle(ctx, g, cycle); err != nil {
				break // stop this group on error; retry next tick
			}
			settled++
			// Reload the group; lifecycle may have advanced to CLOSED.
			g, err = s.store.GetGroup(ctx, g.ID)
			if err != nil || g.State != domain.GroupActive {
				break
			}
		}
	}
	return settled, nil
}

// settleOverdueCycle records defaults for non-contributors then settles.
func (s *Service) settleOverdueCycle(ctx context.Context, g *domain.Group, cycle *domain.Cycle) error {
	contribs, err := s.store.ListContributionsByCycle(ctx, g.ID, cycle.Index)
	if err != nil {
		return err
	}
	paid := make(map[string]bool, len(contribs))
	for _, c := range contribs {
		paid[c.UserID] = true
	}
	for _, uid := range g.Members {
		if !paid[uid] {
			if err := s.recordDefault(ctx, g, cycle, uid); err != nil {
				return err
			}
		}
	}
	_, err = s.settleCycle(ctx, g, cycle)
	return err
}
