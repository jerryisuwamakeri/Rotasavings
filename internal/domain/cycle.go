package domain

import "time"

// Cycle is one round of a group: every member contributes, one member (PayoutUser)
// receives the pooled payout. A default is, deterministically, the failure to
// contribute the required amount before Deadline.
type Cycle struct {
	GroupID    string    `json:"group_id"`
	Index      int       `json:"index"`
	Deadline   time.Time `json:"deadline"`
	PayoutUser string    `json:"payout_user"`
	Settled    bool      `json:"settled"`
}

// IsOverdue reports whether the cycle deadline has passed at time now.
func (c Cycle) IsOverdue(now time.Time) bool {
	return now.After(c.Deadline)
}
