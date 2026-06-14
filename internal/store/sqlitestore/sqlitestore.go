// Package sqlitestore is a real, on-disk SQL implementation of store.Store
// backed by a pure-Go SQLite driver (modernc.org/sqlite — no CGO, no server).
//
// It persists across restarts and is inspectable with the `sqlite3` CLI. The
// code is written against database/sql so it ports to Postgres/pgx with only
// dialect changes (the production schema lives in migrations/0001_init.sql).
//
// Remember the architecture rule: this database is a CACHE of on-chain truth,
// never the source of truth.
package sqlitestore

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

//go:embed schema.sql
var schema string

// Store satisfies the store.Store interface.
var _ store.Store = (*Store)(nil)

const tsFmt = time.RFC3339Nano

// Store is a SQLite-backed store.Store.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) a SQLite database at path and applies the
// schema. Use ":memory:" for an ephemeral DB. WAL + busy_timeout are enabled;
// the pool is capped at one connection so the indexer, scheduler, and HTTP
// handlers never collide on SQLite's single-writer lock.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Health pings the database, verifying the connection is alive.
func (s *Store) Health(ctx context.Context) error { return s.db.PingContext(ctx) }

// --- helpers ---

func tstr(t time.Time) string { return t.UTC().Format(tsFmt) }

func tparse(s string) time.Time {
	t, _ := time.Parse(tsFmt, s)
	return t
}

// nullTime renders an optional time as a NULL-able SQL argument.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return tstr(*t)
}

// optTime converts a nullable column back into a *time.Time.
func optTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := tparse(ns.String)
	return &t
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func parseStrSlice(s string) []string {
	var out []string
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// noRows maps sql.ErrNoRows to domain.ErrNotFound.
func noRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// --- users ---

func (s *Store) SaveUser(ctx context.Context, u *domain.User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id,email,display_name,wallet_address,identity_commitment,
			kyc_provider,kyc_status,role,status,password_hash,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			email=excluded.email, display_name=excluded.display_name,
			wallet_address=excluded.wallet_address, identity_commitment=excluded.identity_commitment,
			kyc_provider=excluded.kyc_provider, kyc_status=excluded.kyc_status,
			role=excluded.role, status=excluded.status, password_hash=excluded.password_hash,
			updated_at=excluded.updated_at`,
		u.ID, u.Email, u.DisplayName, u.WalletAddress, u.IdentityCommitment,
		u.KYCProvider, string(u.KYCStatus), string(u.Role), string(u.Status),
		u.PasswordHash, tstr(u.CreatedAt), tstr(u.UpdatedAt))
	return err
}

func scanUser(sc interface{ Scan(...any) error }) (*domain.User, error) {
	var u domain.User
	var kyc, role, status, created, updated string
	if err := sc.Scan(&u.ID, &u.Email, &u.DisplayName, &u.WalletAddress, &u.IdentityCommitment,
		&u.KYCProvider, &kyc, &role, &status, &u.PasswordHash, &created, &updated); err != nil {
		return nil, err
	}
	u.KYCStatus, u.Role, u.Status = domain.KYCStatus(kyc), domain.Role(role), domain.UserStatus(status)
	u.CreatedAt, u.UpdatedAt = tparse(created), tparse(updated)
	return &u, nil
}

const userCols = `id,email,display_name,wallet_address,identity_commitment,kyc_provider,kyc_status,role,status,password_hash,created_at,updated_at`

func (s *Store) GetUser(ctx context.Context, id string) (*domain.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id=?`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email=?`, email))
	if err != nil {
		return nil, noRows(err)
	}
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context, p store.Page) ([]*domain.User, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at LIMIT ? OFFSET ?`,
		limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*domain.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (s *Store) ListUsersByKYC(ctx context.Context, status domain.KYCStatus) ([]*domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE kyc_status=? ORDER BY created_at`, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- groups ---

func (s *Store) SaveGroup(ctx context.Context, g *domain.Group) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO groups (id,contract_address,name,organizer_id,contribution_amount,
			cycle_length_ns,max_members,total_cycles,members_json,payout_order_json,state,created_at,activated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			contract_address=excluded.contract_address, name=excluded.name,
			organizer_id=excluded.organizer_id, contribution_amount=excluded.contribution_amount,
			cycle_length_ns=excluded.cycle_length_ns, max_members=excluded.max_members,
			total_cycles=excluded.total_cycles, members_json=excluded.members_json,
			payout_order_json=excluded.payout_order_json, state=excluded.state,
			activated_at=excluded.activated_at`,
		g.ID, g.ContractAddress, g.Name, g.OrganizerID, int64(g.ContributionAmount),
		int64(g.CycleLength), g.MaxMembers, g.TotalCycles, mustJSON(g.Members), mustJSON(g.PayoutOrder),
		string(g.State), tstr(g.CreatedAt), nullTime(g.ActivatedAt))
	return err
}

func scanGroup(sc interface{ Scan(...any) error }) (*domain.Group, error) {
	var g domain.Group
	var amount, cycleNs int64
	var members, payout, state, created string
	var activated sql.NullString
	if err := sc.Scan(&g.ID, &g.ContractAddress, &g.Name, &g.OrganizerID, &amount,
		&cycleNs, &g.MaxMembers, &g.TotalCycles, &members, &payout, &state, &created, &activated); err != nil {
		return nil, err
	}
	g.ContributionAmount = domain.Money(amount)
	g.CycleLength = domain.Duration(cycleNs)
	g.Members = parseStrSlice(members)
	g.PayoutOrder = parseStrSlice(payout)
	g.State = domain.GroupState(state)
	g.CreatedAt = tparse(created)
	g.ActivatedAt = optTime(activated)
	return &g, nil
}

const groupCols = `id,contract_address,name,organizer_id,contribution_amount,cycle_length_ns,max_members,total_cycles,members_json,payout_order_json,state,created_at,activated_at`

func (s *Store) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	g, err := scanGroup(s.db.QueryRowContext(ctx, `SELECT `+groupCols+` FROM groups WHERE id=?`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return g, nil
}

func (s *Store) ListGroups(ctx context.Context, p store.Page) ([]*domain.Group, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+groupCols+` FROM groups ORDER BY created_at LIMIT ? OFFSET ?`,
		limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*domain.Group, 0)
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

// --- memberships ---

func (s *Store) SaveMembership(ctx context.Context, m *domain.Membership) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memberships (id,group_id,user_id,organizer,status,joined_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(group_id,user_id) DO UPDATE SET
			id=excluded.id, organizer=excluded.organizer, status=excluded.status`,
		m.ID, m.GroupID, m.UserID, boolToInt(m.Organizer), string(m.Status), tstr(m.JoinedAt))
	return err
}

func scanMembership(sc interface{ Scan(...any) error }) (*domain.Membership, error) {
	var m domain.Membership
	var organizer int
	var status, joined string
	if err := sc.Scan(&m.ID, &m.GroupID, &m.UserID, &organizer, &status, &joined); err != nil {
		return nil, err
	}
	m.Organizer = organizer == 1
	m.Status = domain.MembershipStatus(status)
	m.JoinedAt = tparse(joined)
	return &m, nil
}

const memberCols = `id,group_id,user_id,organizer,status,joined_at`

func (s *Store) GetMembership(ctx context.Context, groupID, userID string) (*domain.Membership, error) {
	m, err := scanMembership(s.db.QueryRowContext(ctx,
		`SELECT `+memberCols+` FROM memberships WHERE group_id=? AND user_id=?`, groupID, userID))
	if err != nil {
		return nil, noRows(err)
	}
	return m, nil
}

func (s *Store) ListMembershipsByGroup(ctx context.Context, groupID string) ([]*domain.Membership, error) {
	return s.queryMemberships(ctx, `SELECT `+memberCols+` FROM memberships WHERE group_id=? ORDER BY joined_at`, groupID)
}

func (s *Store) ListMembershipsByUser(ctx context.Context, userID string) ([]*domain.Membership, error) {
	return s.queryMemberships(ctx, `SELECT `+memberCols+` FROM memberships WHERE user_id=?`, userID)
}

func (s *Store) queryMemberships(ctx context.Context, q string, args ...any) ([]*domain.Membership, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Membership, 0)
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- join requests ---

func (s *Store) SaveJoinRequest(ctx context.Context, j *domain.JoinRequest) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO join_requests (id,group_id,user_id,status,created_at,decided_at,decided_by)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, decided_at=excluded.decided_at, decided_by=excluded.decided_by`,
		j.ID, j.GroupID, j.UserID, string(j.Status), tstr(j.CreatedAt), nullTime(j.DecidedAt), j.DecidedBy)
	return err
}

func scanJoinRequest(sc interface{ Scan(...any) error }) (*domain.JoinRequest, error) {
	var j domain.JoinRequest
	var status, created string
	var decidedAt sql.NullString
	if err := sc.Scan(&j.ID, &j.GroupID, &j.UserID, &status, &created, &decidedAt, &j.DecidedBy); err != nil {
		return nil, err
	}
	j.Status = domain.JoinRequestStatus(status)
	j.CreatedAt = tparse(created)
	j.DecidedAt = optTime(decidedAt)
	return &j, nil
}

func (s *Store) GetJoinRequest(ctx context.Context, id string) (*domain.JoinRequest, error) {
	j, err := scanJoinRequest(s.db.QueryRowContext(ctx,
		`SELECT id,group_id,user_id,status,created_at,decided_at,decided_by FROM join_requests WHERE id=?`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return j, nil
}

func (s *Store) ListJoinRequestsByGroup(ctx context.Context, groupID string) ([]*domain.JoinRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,group_id,user_id,status,created_at,decided_at,decided_by FROM join_requests WHERE group_id=? ORDER BY created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.JoinRequest, 0)
	for rows.Next() {
		j, err := scanJoinRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// --- invitations ---

func (s *Store) SaveInvitation(ctx context.Context, i *domain.Invitation) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO invitations (id,group_id,user_id,invited_by,status,created_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET status=excluded.status`,
		i.ID, i.GroupID, i.UserID, i.InvitedBy, string(i.Status), tstr(i.CreatedAt))
	return err
}

func scanInvitation(sc interface{ Scan(...any) error }) (*domain.Invitation, error) {
	var i domain.Invitation
	var status, created string
	if err := sc.Scan(&i.ID, &i.GroupID, &i.UserID, &i.InvitedBy, &status, &created); err != nil {
		return nil, err
	}
	i.Status = domain.InvitationStatus(status)
	i.CreatedAt = tparse(created)
	return &i, nil
}

func (s *Store) GetInvitation(ctx context.Context, id string) (*domain.Invitation, error) {
	i, err := scanInvitation(s.db.QueryRowContext(ctx,
		`SELECT id,group_id,user_id,invited_by,status,created_at FROM invitations WHERE id=?`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return i, nil
}

func (s *Store) ListInvitationsByUser(ctx context.Context, userID string) ([]*domain.Invitation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,group_id,user_id,invited_by,status,created_at FROM invitations WHERE user_id=? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Invitation, 0)
	for rows.Next() {
		i, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// --- cycles ---

func (s *Store) SaveCycle(ctx context.Context, c *domain.Cycle) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cycles (group_id,idx,deadline,payout_user,settled)
		VALUES (?,?,?,?,?)
		ON CONFLICT(group_id,idx) DO UPDATE SET
			deadline=excluded.deadline, payout_user=excluded.payout_user, settled=excluded.settled`,
		c.GroupID, c.Index, tstr(c.Deadline), c.PayoutUser, boolToInt(c.Settled))
	return err
}

func scanCycle(sc interface{ Scan(...any) error }) (*domain.Cycle, error) {
	var c domain.Cycle
	var deadline string
	var settled int
	if err := sc.Scan(&c.GroupID, &c.Index, &deadline, &c.PayoutUser, &settled); err != nil {
		return nil, err
	}
	c.Deadline = tparse(deadline)
	c.Settled = settled == 1
	return &c, nil
}

func (s *Store) GetCycle(ctx context.Context, groupID string, index int) (*domain.Cycle, error) {
	c, err := scanCycle(s.db.QueryRowContext(ctx,
		`SELECT group_id,idx,deadline,payout_user,settled FROM cycles WHERE group_id=? AND idx=?`, groupID, index))
	if err != nil {
		return nil, noRows(err)
	}
	return c, nil
}

func (s *Store) ListCyclesByGroup(ctx context.Context, groupID string) ([]*domain.Cycle, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id,idx,deadline,payout_user,settled FROM cycles WHERE group_id=? ORDER BY idx`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Cycle, 0)
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- contributions ---

func (s *Store) SaveContribution(ctx context.Context, c *domain.Contribution) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contributions (id,group_id,user_id,cycle_index,amount,commitment,revealed,paid_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(group_id,user_id,cycle_index) DO UPDATE SET
			id=excluded.id, amount=excluded.amount, commitment=excluded.commitment,
			revealed=excluded.revealed, paid_at=excluded.paid_at`,
		c.ID, c.GroupID, c.UserID, c.CycleIndex, int64(c.Amount), c.Commitment, boolToInt(c.Revealed), nullTime(c.PaidAt))
	return err
}

func scanContribution(sc interface{ Scan(...any) error }) (*domain.Contribution, error) {
	var c domain.Contribution
	var amount int64
	var revealed int
	var paidAt sql.NullString
	if err := sc.Scan(&c.ID, &c.GroupID, &c.UserID, &c.CycleIndex, &amount, &c.Commitment, &revealed, &paidAt); err != nil {
		return nil, err
	}
	c.Amount = domain.Money(amount)
	c.Revealed = revealed == 1
	c.PaidAt = optTime(paidAt)
	return &c, nil
}

const contribCols = `id,group_id,user_id,cycle_index,amount,commitment,revealed,paid_at`

func (s *Store) GetContribution(ctx context.Context, groupID, userID string, cycle int) (*domain.Contribution, error) {
	c, err := scanContribution(s.db.QueryRowContext(ctx,
		`SELECT `+contribCols+` FROM contributions WHERE group_id=? AND user_id=? AND cycle_index=?`, groupID, userID, cycle))
	if err != nil {
		return nil, noRows(err)
	}
	return c, nil
}

func (s *Store) ListContributionsByCycle(ctx context.Context, groupID string, cycle int) ([]*domain.Contribution, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+contribCols+` FROM contributions WHERE group_id=? AND cycle_index=?`, groupID, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Contribution, 0)
	for rows.Next() {
		c, err := scanContribution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- payouts (append-only) ---

func (s *Store) SavePayout(ctx context.Context, p *domain.Payout) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payouts (id,group_id,cycle_index,payee_user_id,gross_amount,expected_amount,shortfall,disbursement_ref,settled_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		p.ID, p.GroupID, p.CycleIndex, p.PayeeUserID, int64(p.GrossAmount),
		int64(p.ExpectedAmount), int64(p.Shortfall), p.DisbursementRef, tstr(p.SettledAt))
	return err
}

func (s *Store) ListPayoutsByGroup(ctx context.Context, groupID string) ([]*domain.Payout, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,group_id,cycle_index,payee_user_id,gross_amount,expected_amount,shortfall,disbursement_ref,settled_at
		 FROM payouts WHERE group_id=? ORDER BY cycle_index`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Payout, 0)
	for rows.Next() {
		var p domain.Payout
		var gross, expected, shortfall int64
		var settled string
		if err := rows.Scan(&p.ID, &p.GroupID, &p.CycleIndex, &p.PayeeUserID,
			&gross, &expected, &shortfall, &p.DisbursementRef, &settled); err != nil {
			return nil, err
		}
		p.GrossAmount, p.ExpectedAmount, p.Shortfall = domain.Money(gross), domain.Money(expected), domain.Money(shortfall)
		p.SettledAt = tparse(settled)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// --- escrow (append-only) ---

func (s *Store) AppendEscrowEntry(ctx context.Context, e *domain.EscrowEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO escrow_entries (id,group_id,cycle_index,user_id,direction,amount,reference,memo,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.GroupID, e.CycleIndex, e.UserID, string(e.Direction), int64(e.Amount), e.Reference, e.Memo, tstr(e.CreatedAt))
	return err
}

func (s *Store) ListEscrowByGroup(ctx context.Context, groupID string) ([]*domain.EscrowEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,group_id,cycle_index,user_id,direction,amount,reference,memo,created_at
		 FROM escrow_entries WHERE group_id=? ORDER BY created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.EscrowEntry, 0)
	for rows.Next() {
		var e domain.EscrowEntry
		var dir, created string
		var amount int64
		if err := rows.Scan(&e.ID, &e.GroupID, &e.CycleIndex, &e.UserID, &dir, &amount, &e.Reference, &e.Memo, &created); err != nil {
			return nil, err
		}
		e.Direction = domain.LedgerDirection(dir)
		e.Amount = domain.Money(amount)
		e.CreatedAt = tparse(created)
		out = append(out, &e)
	}
	return out, rows.Err()
}

// --- reputation (append-only) ---

func (s *Store) AppendReputationEvent(ctx context.Context, e *domain.ReputationEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reputation_events (id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp)
		VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.UserID, e.GroupID, e.CycleIndex, string(e.Type), int64(e.Amount), e.ProofHash, tstr(e.Timestamp))
	return err
}

func (s *Store) ReputationEvents(ctx context.Context, userID string) ([]domain.ReputationEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp
		 FROM reputation_events WHERE user_id=? ORDER BY timestamp`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ReputationEvent, 0)
	for rows.Next() {
		var e domain.ReputationEvent
		var typ, ts string
		var amount int64
		if err := rows.Scan(&e.ID, &e.UserID, &e.GroupID, &e.CycleIndex, &typ, &amount, &e.ProofHash, &ts); err != nil {
			return nil, err
		}
		e.Type = domain.EventType(typ)
		e.Amount = domain.Money(amount)
		e.Timestamp = tparse(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) AllReputationEvents(ctx context.Context, p store.Page) ([]domain.ReputationEvent, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reputation_events`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp
		 FROM reputation_events ORDER BY timestamp DESC LIMIT ? OFFSET ?`, limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.ReputationEvent, 0)
	for rows.Next() {
		var e domain.ReputationEvent
		var typ, ts string
		var amount int64
		if err := rows.Scan(&e.ID, &e.UserID, &e.GroupID, &e.CycleIndex, &typ, &amount, &e.ProofHash, &ts); err != nil {
			return nil, 0, err
		}
		e.Type = domain.EventType(typ)
		e.Amount = domain.Money(amount)
		e.Timestamp = tparse(ts)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// --- webhooks ---

func (s *Store) SaveWebhook(ctx context.Context, w *domain.Webhook) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhooks (id,url,secret,active,created_by,created_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET url=excluded.url, secret=excluded.secret, active=excluded.active`,
		w.ID, w.URL, w.Secret, boolToInt(w.Active), w.CreatedBy, tstr(w.CreatedAt))
	return err
}

func scanWebhook(sc interface{ Scan(...any) error }) (*domain.Webhook, error) {
	var w domain.Webhook
	var active int
	var created string
	if err := sc.Scan(&w.ID, &w.URL, &w.Secret, &active, &w.CreatedBy, &created); err != nil {
		return nil, err
	}
	w.Active = active == 1
	w.CreatedAt = tparse(created)
	return &w, nil
}

func (s *Store) GetWebhook(ctx context.Context, id string) (*domain.Webhook, error) {
	w, err := scanWebhook(s.db.QueryRowContext(ctx, `SELECT id,url,secret,active,created_by,created_at FROM webhooks WHERE id=?`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return w, nil
}

func (s *Store) ListWebhooks(ctx context.Context) ([]*domain.Webhook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,url,secret,active,created_by,created_at FROM webhooks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Webhook, 0)
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id=?`, id)
	return err
}

// --- notifications (append-only) ---

func (s *Store) SaveNotification(ctx context.Context, n *domain.Notification) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notifications (id,user_id,kind,title,body,read,created_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET read=excluded.read`,
		n.ID, n.UserID, string(n.Kind), n.Title, n.Body, boolToInt(n.Read), tstr(n.CreatedAt))
	return err
}

func (s *Store) ListNotificationsByUser(ctx context.Context, userID string) ([]*domain.Notification, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,user_id,kind,title,body,read,created_at FROM notifications WHERE user_id=? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Notification, 0)
	for rows.Next() {
		var n domain.Notification
		var kind, created string
		var read int
		if err := rows.Scan(&n.ID, &n.UserID, &kind, &n.Title, &n.Body, &read, &created); err != nil {
			return nil, err
		}
		n.Kind = domain.NotificationKind(kind)
		n.Read = read == 1
		n.CreatedAt = tparse(created)
		out = append(out, &n)
	}
	return out, rows.Err()
}

// --- audit (append-only) ---

func (s *Store) AppendAudit(ctx context.Context, a *domain.AuditEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit (id,actor_id,action,target,detail,created_at)
		VALUES (?,?,?,?,?,?)`,
		a.ID, a.ActorID, a.Action, a.Target, a.Detail, tstr(a.CreatedAt))
	return err
}

func (s *Store) ListAudit(ctx context.Context, p store.Page) ([]*domain.AuditEntry, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,actor_id,action,target,detail,created_at FROM audit ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*domain.AuditEntry, 0)
	for rows.Next() {
		var a domain.AuditEntry
		var created string
		if err := rows.Scan(&a.ID, &a.ActorID, &a.Action, &a.Target, &a.Detail, &created); err != nil {
			return nil, 0, err
		}
		a.CreatedAt = tparse(created)
		out = append(out, &a)
	}
	return out, total, rows.Err()
}

func limit(p store.Page) int {
	if p.Limit <= 0 {
		return 50
	}
	return p.Limit
}
