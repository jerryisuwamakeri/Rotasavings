package domain

import (
	"errors"
	"testing"
	"time"
)

func validGroup() *Group {
	return &Group{
		Name:               "Block A",
		ContributionAmount: 50000,
		CycleLength:        Duration(168 * time.Hour),
		Members:            []string{"a", "b"},
		PayoutOrder:        []string{"a", "b"},
	}
}

func TestGroupValidate(t *testing.T) {
	if err := validGroup().Validate(); err != nil {
		t.Fatalf("valid group rejected: %v", err)
	}

	cases := map[string]func(*Group){
		"no name":           func(g *Group) { g.Name = "" },
		"zero amount":       func(g *Group) { g.ContributionAmount = 0 },
		"one member":        func(g *Group) { g.Members = []string{"a"}; g.PayoutOrder = []string{"a"} },
		"payout mismatch":   func(g *Group) { g.PayoutOrder = []string{"a"} },
		"payout non-member": func(g *Group) { g.PayoutOrder = []string{"a", "z"} },
		"dup member":        func(g *Group) { g.Members = []string{"a", "a"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			g := validGroup()
			mutate(g)
			if err := g.Validate(); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestGroupLifecycle(t *testing.T) {
	g := validGroup()
	g.State = GroupCreated

	if err := g.Transition(GroupSettlement); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("CREATED->SETTLEMENT should be invalid, got %v", err)
	}
	for _, next := range []GroupState{GroupActive, GroupSettlement, GroupClosed} {
		if err := g.Transition(next); err != nil {
			t.Fatalf("transition to %s failed: %v", next, err)
		}
	}
	if err := g.Transition(GroupActive); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("CLOSED->ACTIVE should be invalid, got %v", err)
	}
}

func TestCyclesSchedule(t *testing.T) {
	g := validGroup()
	g.ID = "grp"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cycles := g.Cycles(start)
	if len(cycles) != 2 {
		t.Fatalf("want 2 cycles, got %d", len(cycles))
	}
	if cycles[0].PayoutUser != "a" || cycles[1].PayoutUser != "b" {
		t.Fatalf("payout order not preserved: %+v", cycles)
	}
	if !cycles[0].Deadline.Equal(start.Add(168 * time.Hour)) {
		t.Fatalf("unexpected deadline: %v", cycles[0].Deadline)
	}
}

func TestCommitmentDeterminism(t *testing.T) {
	a := ComputeContributionCommitment("u", "g", 1, 100)
	b := ComputeContributionCommitment("u", "g", 1, 100)
	c := ComputeContributionCommitment("u", "g", 1, 101)
	if a != b {
		t.Fatal("same inputs must produce same commitment")
	}
	if a == c {
		t.Fatal("different amount must produce different commitment")
	}
}
