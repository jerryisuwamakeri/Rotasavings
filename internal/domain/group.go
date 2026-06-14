package domain

import (
	"fmt"
	"time"
)

// GroupState is the deployed contract's lifecycle. Transitions are one-way:
//
//	CREATED -> ACTIVE -> SETTLEMENT -> CLOSED
type GroupState string

const (
	GroupCreated    GroupState = "CREATED"
	GroupActive     GroupState = "ACTIVE"
	GroupSettlement GroupState = "SETTLEMENT"
	GroupClosed     GroupState = "CLOSED"
)

// allowedTransitions encodes the immutable lifecycle. A group can never move
// backwards; the contract enforces the same on-chain.
var allowedTransitions = map[GroupState]map[GroupState]bool{
	GroupCreated:    {GroupActive: true},
	GroupActive:     {GroupSettlement: true},
	GroupSettlement: {GroupClosed: true},
	GroupClosed:     {},
}

// Group is the off-chain projection of a deployed RotasavingsGroup contract.
// Once deployed, ContributionAmount / PayoutOrder / rules are immutable.
type Group struct {
	ID                 string     `json:"id"`
	ContractAddress    string     `json:"contract_address"`
	Name               string     `json:"name"`
	OrganizerID        string     `json:"organizer_id"`
	ContributionAmount Money      `json:"contribution_amount"`
	CycleLength        Duration   `json:"cycle_length"`
	MaxMembers         int        `json:"max_members"`
	TotalCycles        int        `json:"total_cycles"`
	Members            []string   `json:"members"`      // user IDs (kept in sync with memberships)
	PayoutOrder        []string   `json:"payout_order"` // user IDs, one per cycle; set at activation
	State              GroupState `json:"state"`
	CreatedAt          time.Time  `json:"created_at"`
	ActivatedAt        *time.Time `json:"activated_at,omitempty"`
}

// ValidateConfig checks the immutable parameters set at creation time, before
// any members have joined.
func (g *Group) ValidateConfig() error {
	switch {
	case g.Name == "":
		return fmt.Errorf("%w: group name is required", ErrValidation)
	case g.ContributionAmount <= 0:
		return fmt.Errorf("%w: contribution amount must be positive", ErrValidation)
	case time.Duration(g.CycleLength) <= 0:
		return fmt.Errorf("%w: cycle length must be positive", ErrValidation)
	case g.MaxMembers < 2:
		return fmt.Errorf("%w: max members must be at least 2", ErrValidation)
	}
	return nil
}

// HasMember reports whether userID is in the group's member list.
func (g *Group) HasMember(userID string) bool {
	for _, m := range g.Members {
		if m == userID {
			return true
		}
	}
	return false
}

// Duration is a time.Duration that marshals to/from a human string ("168h").
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("%w: invalid duration %q", ErrValidation, s)
	}
	*d = Duration(parsed)
	return nil
}

// Validate checks the structural rules a group must satisfy before deployment.
func (g *Group) Validate() error {
	switch {
	case g.Name == "":
		return fmt.Errorf("%w: group name is required", ErrValidation)
	case g.ContributionAmount <= 0:
		return fmt.Errorf("%w: contribution amount must be positive", ErrValidation)
	case len(g.Members) < 2:
		return fmt.Errorf("%w: a group needs at least 2 members", ErrValidation)
	case time.Duration(g.CycleLength) <= 0:
		return fmt.Errorf("%w: cycle length must be positive", ErrValidation)
	}
	// In a classic ROSCA every member receives exactly one payout, so the
	// payout order length must equal the membership and total cycle count.
	if len(g.PayoutOrder) != len(g.Members) {
		return fmt.Errorf("%w: payout order must cover every member exactly once", ErrValidation)
	}
	seen := make(map[string]bool, len(g.Members))
	for _, m := range g.Members {
		if seen[m] {
			return fmt.Errorf("%w: duplicate member %q", ErrValidation, m)
		}
		seen[m] = true
	}
	for _, p := range g.PayoutOrder {
		if !seen[p] {
			return fmt.Errorf("%w: payout user %q is not a member", ErrValidation, p)
		}
	}
	return nil
}

// CanTransition reports whether the group may move to next.
func (g *Group) CanTransition(next GroupState) bool {
	return allowedTransitions[g.State][next]
}

// Transition moves the group to next, enforcing the lifecycle.
func (g *Group) Transition(next GroupState) error {
	if !g.CanTransition(next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, g.State, next)
	}
	g.State = next
	return nil
}

// Cycles materialises the schedule of cycles from the immutable group rules.
// Cycle i pays out PayoutOrder[i]; its deadline is start + (i+1)*CycleLength.
func (g *Group) Cycles(start time.Time) []Cycle {
	out := make([]Cycle, 0, len(g.PayoutOrder))
	for i, payee := range g.PayoutOrder {
		out = append(out, Cycle{
			GroupID:    g.ID,
			Index:      i,
			Deadline:   start.Add(time.Duration(i+1) * time.Duration(g.CycleLength)),
			PayoutUser: payee,
		})
	}
	return out
}
