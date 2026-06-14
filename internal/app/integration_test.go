package app_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"rotasavings/internal/app"
	"rotasavings/internal/auth"
	"rotasavings/internal/chain"
	"rotasavings/internal/domain"
	"rotasavings/internal/indexer"
	"rotasavings/internal/notify"
	"rotasavings/internal/payments"
	"rotasavings/internal/store/sqlitestore"
)

// harness wires a full service against the real SQLite store with the indexer
// running, plus a controllable clock.
type harness struct {
	svc   *app.Service
	clock time.Time
	admin string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	truth := chain.NewMemChain()
	svc := app.NewService(app.Deps{
		Chain:    truth,
		Store:    db,
		Payments: payments.NewMockProvider(),
		Notifier: notify.NewLogNotifier(log),
		Issuer:   auth.NewIssuer("test-secret", time.Hour, 24*time.Hour),
	})

	h := &harness{svc: svc, clock: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	svc.SetClock(func() time.Time { return h.clock })

	// Run the indexer so reputation events project into the store.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = indexer.New(truth, db, log).Run(ctx) }()

	if err := svc.SeedAdmin(ctx, "admin@t.local", "password123", "Admin"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	adm, _ := db.GetUserByEmail(ctx, "admin@t.local")
	h.admin = adm.ID
	return h
}

// approvedMember registers a user and approves their KYC.
func (h *harness) approvedMember(t *testing.T, email string) string {
	t.Helper()
	ctx := context.Background()
	u, err := h.svc.Register(ctx, app.RegisterInput{
		Email: email, Password: "password123", DisplayName: email,
		WalletAddress: "0x" + email, KYCProvider: "k", KYCSignature: "s",
	})
	if err != nil {
		t.Fatalf("register %s: %v", email, err)
	}
	if _, err := h.svc.DecideKYC(ctx, h.admin, u.ID, true); err != nil {
		t.Fatalf("approve kyc %s: %v", email, err)
	}
	return u.ID
}

// waitReputation polls until the user's summary satisfies pred (indexer is async).
func (h *harness) waitReputation(t *testing.T, userID string, pred func(domain.ReputationSummary) bool) domain.ReputationSummary {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s, err := h.svc.Reputation(context.Background(), userID)
		if err == nil && pred(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("reputation never reached expected state for %s: last=%+v err=%v", userID, s, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestFullRoscaLifecycle exercises register → KYC → group → join → activate →
// contribute → auto-settle → default-on-deadline → close, against the real DB.
func TestFullRoscaLifecycle(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	ada := h.approvedMember(t, "ada@t.local")
	bob := h.approvedMember(t, "bob@t.local")

	g, err := h.svc.CreateGroup(ctx, ada, app.CreateGroupInput{
		Name: "Block", ContributionAmount: 50000, CycleLength: domain.Duration(time.Hour), MaxMembers: 2,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Bob joins via request → Ada approves.
	jr, err := h.svc.RequestToJoin(ctx, bob, g.ID)
	if err != nil {
		t.Fatalf("request join: %v", err)
	}
	if _, err := h.svc.DecideJoinRequest(ctx, ada, jr.ID, true); err != nil {
		t.Fatalf("approve join: %v", err)
	}

	// Activate with explicit payout order [ada, bob].
	if _, err := h.svc.ActivateGroup(ctx, ada, g.ID, []string{ada, bob}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Cycle 0: both contribute → auto-settles, payout to Ada.
	if _, err := h.svc.SubmitContribution(ctx, ada, g.ID, 0, "momo"); err != nil {
		t.Fatalf("ada c0: %v", err)
	}
	if _, err := h.svc.SubmitContribution(ctx, bob, g.ID, 0, "momo"); err != nil {
		t.Fatalf("bob c0: %v", err)
	}
	c0, err := h.svc.CycleStatus(ctx, g.ID, 0)
	if err != nil || !c0.Cycle.Settled {
		t.Fatalf("cycle 0 should be settled: %+v err=%v", c0, err)
	}

	// Cycle 1: only Ada contributes; advance past the deadline; scheduler logic
	// records Bob's default and settles → group CLOSED.
	if _, err := h.svc.SubmitContribution(ctx, ada, g.ID, 1, "momo"); err != nil {
		t.Fatalf("ada c1: %v", err)
	}
	h.clock = h.clock.Add(3 * time.Hour) // past both cycle deadlines
	settled, err := h.svc.ProcessDeadlines(ctx, h.clock)
	if err != nil {
		t.Fatalf("process deadlines: %v", err)
	}
	if settled != 1 {
		t.Fatalf("expected 1 cycle settled on deadline, got %d", settled)
	}

	final, err := h.svc.GetGroup(ctx, g.ID)
	if err != nil || final.State != domain.GroupClosed {
		t.Fatalf("group should be CLOSED, got %v (err %v)", final.State, err)
	}

	// Reputation (projected async via the indexer).
	adaRep := h.waitReputation(t, ada, func(s domain.ReputationSummary) bool {
		return s.ContributionsMade == 2 && s.PayoutsReceived == 1
	})
	if adaRep.ContributionsMissed != 0 || adaRep.ReliabilityRatio != 1.0 {
		t.Fatalf("ada reputation wrong: %+v", adaRep)
	}
	bobRep := h.waitReputation(t, bob, func(s domain.ReputationSummary) bool {
		return s.ContributionsMade == 1 && s.ContributionsMissed == 1
	})
	if bobRep.PayoutsReceived != 1 {
		t.Fatalf("bob should have received the cycle-1 payout: %+v", bobRep)
	}
	if bobRep.ReliabilityRatio != 0.5 {
		t.Fatalf("bob reliability should be 0.5, got %v", bobRep.ReliabilityRatio)
	}

	// Escrow nets to zero once both cycles have paid out.
	liq, err := h.svc.AssessGroupLiquidity(ctx, g.ID)
	if err != nil {
		t.Fatalf("liquidity: %v", err)
	}
	if liq.EscrowBalance != 0 {
		t.Fatalf("escrow should net to 0 after all payouts, got %d", liq.EscrowBalance)
	}
}

// TestAccessControlAndKYCGate verifies the participation gate and organizer-only
// actions at the service layer.
func TestAccessControlAndKYCGate(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	// A user with pending KYC cannot create a group.
	pending, err := h.svc.Register(ctx, app.RegisterInput{
		Email: "p@t.local", Password: "password123", DisplayName: "P",
		WalletAddress: "0xP", KYCProvider: "k", KYCSignature: "s",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := h.svc.CreateGroup(ctx, pending.ID, app.CreateGroupInput{
		Name: "X", ContributionAmount: 1000, CycleLength: domain.Duration(time.Hour), MaxMembers: 2,
	}); err == nil {
		t.Fatal("pending-KYC user should not be able to create a group")
	}

	ada := h.approvedMember(t, "ada@t.local")
	bob := h.approvedMember(t, "bob@t.local")
	g, err := h.svc.CreateGroup(ctx, ada, app.CreateGroupInput{
		Name: "Block", ContributionAmount: 1000, CycleLength: domain.Duration(time.Hour), MaxMembers: 2,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Non-organizer cannot activate.
	if _, err := h.svc.ActivateGroup(ctx, bob, g.ID, nil); err == nil {
		t.Fatal("non-organizer should not be able to activate")
	}
}
