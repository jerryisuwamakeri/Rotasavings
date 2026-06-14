package domain

import (
	"strconv"
	"time"
)

// Contribution is the off-chain projection of an on-chain contribution
// commitment. The commitment is computed before money moves; when payment is
// confirmed the commitment is "revealed" and verified against the chain.
type Contribution struct {
	ID         string     `json:"id"`
	GroupID    string     `json:"group_id"`
	UserID     string     `json:"user_id"`
	CycleIndex int        `json:"cycle_index"`
	Amount     Money      `json:"amount"`
	Commitment string     `json:"commitment"`
	Revealed   bool       `json:"revealed"`
	PaidAt     *time.Time `json:"paid_at,omitempty"`
}

// ComputeContributionCommitment mirrors the on-chain commitment:
//
//	H(user + group + cycle + amount)
//
// This binds a contribution to an exact (user, group, cycle, amount) tuple so a
// payment cannot be retroactively re-attributed or forged.
func ComputeContributionCommitment(userID, groupID string, cycle int, amount Money) string {
	return Hash(userID, groupID, strconv.Itoa(cycle), strconv.FormatInt(int64(amount), 10))
}
