package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// A file-backed DB in a temp dir, so we can also reopen it.
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserRoundTripAndUpsert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	u := &domain.User{
		ID: "u1", Email: "a@x.com", DisplayName: "Ada", WalletAddress: "0xA",
		IdentityCommitment: "0xc", KYCProvider: "k", KYCStatus: domain.KYCPending,
		Role: domain.RoleMember, Status: domain.UserActive, PasswordHash: "h",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond), UpdatedAt: time.Now().UTC(),
	}
	if err := s.SaveUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUser(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "a@x.com" || got.KYCStatus != domain.KYCPending {
		t.Fatalf("bad read: %+v", got)
	}
	if byEmail, err := s.GetUserByEmail(ctx, "a@x.com"); err != nil || byEmail.ID != "u1" {
		t.Fatalf("get by email failed: %v %+v", err, byEmail)
	}

	// Upsert: change KYC, ensure no duplicate row and the change persists.
	u.KYCStatus = domain.KYCApproved
	if err := s.SaveUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	users, total, err := s.ListUsers(ctx, store.Page{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(users) != 1 {
		t.Fatalf("expected exactly 1 user after upsert, got %d", total)
	}
	if users[0].KYCStatus != domain.KYCApproved {
		t.Fatalf("upsert did not persist KYC change: %+v", users[0])
	}
}

func TestGroupArraysAndMembershipUpsert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	g := &domain.Group{
		ID: "g1", ContractAddress: "0xG", Name: "Block", OrganizerID: "u1",
		ContributionAmount: 50000, CycleLength: domain.Duration(time.Hour), MaxMembers: 3,
		Members: []string{"u1", "u2"}, PayoutOrder: []string{"u2", "u1"},
		State: domain.GroupCreated, CreatedAt: time.Now().UTC(),
	}
	if err := s.SaveGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetGroup(ctx, "g1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 2 || got.Members[1] != "u2" || got.PayoutOrder[0] != "u2" {
		t.Fatalf("array round-trip failed: %+v", got)
	}

	// Membership upsert keyed on (group,user): saving twice must not duplicate.
	m := &domain.Membership{ID: "m1", GroupID: "g1", UserID: "u2", Status: domain.MembershipActive, JoinedAt: time.Now().UTC()}
	if err := s.SaveMembership(ctx, m); err != nil {
		t.Fatal(err)
	}
	m2 := &domain.Membership{ID: "m2", GroupID: "g1", UserID: "u2", Status: domain.MembershipLeft, JoinedAt: time.Now().UTC()}
	if err := s.SaveMembership(ctx, m2); err != nil {
		t.Fatal(err)
	}
	mems, err := s.ListMembershipsByGroup(ctx, "g1")
	if err != nil {
		t.Fatal(err)
	}
	if len(mems) != 1 || mems[0].Status != domain.MembershipLeft {
		t.Fatalf("membership upsert on (group,user) failed: %+v", mems)
	}
}

func TestReputationAppendAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for i := 0; i < 3; i++ {
		_ = s.AppendReputationEvent(ctx, &domain.ReputationEvent{
			ID: domain.NewID(), UserID: "u1", Type: domain.EventContributionMade,
			Amount: 100, Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Second),
		})
	}
	events, err := s.ReputationEvents(ctx, "u1")
	if err != nil || len(events) != 3 {
		t.Fatalf("expected 3 events, got %d (%v)", len(events), err)
	}
	summary := domain.SummariseReputation("u1", events)
	if summary.ContributionsMade != 3 {
		t.Fatalf("bad summary: %+v", summary)
	}

	if _, err := s.GetGroup(ctx, "missing"); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestPersistenceAcrossReopen proves the data survives closing and reopening the
// database file — the whole point of a "real" database.
func TestPersistenceAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.SaveUser(ctx, &domain.User{
		ID: "u1", Email: "p@x.com", DisplayName: "P", WalletAddress: "0xP",
		IdentityCommitment: "0xc", KYCStatus: domain.KYCApproved, Role: domain.RoleMember,
		Status: domain.UserActive, PasswordHash: "h", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	_ = s1.Close()

	// Reopen the same file in a fresh Store.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("data did not survive reopen: %v", err)
	}
	if got.Email != "p@x.com" {
		t.Fatalf("bad data after reopen: %+v", got)
	}
}
