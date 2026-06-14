// Package pgstore is the production PostgreSQL implementation of store.Store,
// backed by pgx/v5. It is the direct port of the SQLite dev store: same
// database/sql-shaped logic, Postgres dialect ($N placeholders, native TEXT[]
// and TIMESTAMPTZ, foreign keys). Select it by setting ROTA_POSTGRES_DSN.
//
// As with every store, this database is the cache for on-chain projections and
// the source of truth for off-chain application state (see schema.sql).
package pgstore

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

//go:embed schema.sql
var schema string

// Store satisfies the store.Store interface.
var _ store.Store = (*Store)(nil)

// Store is a Postgres-backed store.Store.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to Postgres, verifies the connection, and applies the schema
// (idempotently). dsn is a standard libpq/pgx connection string.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// Health pings the database.
func (s *Store) Health(ctx context.Context) error { return s.pool.Ping(ctx) }

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func nt(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// --- users ---

func (s *Store) SaveUser(ctx context.Context, u *domain.User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id,email,display_name,wallet_address,identity_commitment,
			kyc_provider,kyc_status,role,status,password_hash,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			email=EXCLUDED.email, display_name=EXCLUDED.display_name,
			wallet_address=EXCLUDED.wallet_address, identity_commitment=EXCLUDED.identity_commitment,
			kyc_provider=EXCLUDED.kyc_provider, kyc_status=EXCLUDED.kyc_status,
			role=EXCLUDED.role, status=EXCLUDED.status, password_hash=EXCLUDED.password_hash,
			updated_at=EXCLUDED.updated_at`,
		u.ID, u.Email, u.DisplayName, u.WalletAddress, u.IdentityCommitment,
		u.KYCProvider, string(u.KYCStatus), string(u.Role), string(u.Status),
		u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	return err
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var kyc, role, status string
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.WalletAddress, &u.IdentityCommitment,
		&u.KYCProvider, &kyc, &role, &status, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	u.KYCStatus, u.Role, u.Status = domain.KYCStatus(kyc), domain.Role(role), domain.UserStatus(status)
	return &u, nil
}

const userCols = `id,email,display_name,wallet_address,identity_commitment,kyc_provider,kyc_status,role,status,password_hash,created_at,updated_at`

func (s *Store) GetUser(ctx context.Context, id string) (*domain.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id=$1`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE email=$1`, email))
	if err != nil {
		return nil, noRows(err)
	}
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context, p store.Page) ([]*domain.User, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at LIMIT $1 OFFSET $2`, limit(p), p.Offset)
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
	rows, err := s.pool.Query(ctx, `SELECT `+userCols+` FROM users WHERE kyc_status=$1 ORDER BY created_at`, string(status))
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO groups (id,contract_address,name,organizer_id,contribution_amount,
			cycle_length_ns,max_members,total_cycles,members,payout_order,state,created_at,activated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			contract_address=EXCLUDED.contract_address, name=EXCLUDED.name,
			organizer_id=EXCLUDED.organizer_id, contribution_amount=EXCLUDED.contribution_amount,
			cycle_length_ns=EXCLUDED.cycle_length_ns, max_members=EXCLUDED.max_members,
			total_cycles=EXCLUDED.total_cycles, members=EXCLUDED.members,
			payout_order=EXCLUDED.payout_order, state=EXCLUDED.state, activated_at=EXCLUDED.activated_at`,
		g.ID, g.ContractAddress, g.Name, g.OrganizerID, int64(g.ContributionAmount),
		int64(g.CycleLength), g.MaxMembers, g.TotalCycles, g.Members, g.PayoutOrder,
		string(g.State), g.CreatedAt, nt(g.ActivatedAt))
	return err
}

func scanGroup(row pgx.Row) (*domain.Group, error) {
	var g domain.Group
	var amount, cycleNs int64
	var state string
	var activated *time.Time
	if err := row.Scan(&g.ID, &g.ContractAddress, &g.Name, &g.OrganizerID, &amount,
		&cycleNs, &g.MaxMembers, &g.TotalCycles, &g.Members, &g.PayoutOrder, &state, &g.CreatedAt, &activated); err != nil {
		return nil, err
	}
	g.ContributionAmount = domain.Money(amount)
	g.CycleLength = domain.Duration(cycleNs)
	g.State = domain.GroupState(state)
	g.ActivatedAt = activated
	return &g, nil
}

const groupCols = `id,contract_address,name,organizer_id,contribution_amount,cycle_length_ns,max_members,total_cycles,members,payout_order,state,created_at,activated_at`

func (s *Store) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	g, err := scanGroup(s.pool.QueryRow(ctx, `SELECT `+groupCols+` FROM groups WHERE id=$1`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return g, nil
}

func (s *Store) ListGroups(ctx context.Context, p store.Page) ([]*domain.Group, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM groups`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+groupCols+` FROM groups ORDER BY created_at LIMIT $1 OFFSET $2`, limit(p), p.Offset)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO memberships (id,group_id,user_id,organizer,status,joined_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (group_id,user_id) DO UPDATE SET
			id=EXCLUDED.id, organizer=EXCLUDED.organizer, status=EXCLUDED.status`,
		m.ID, m.GroupID, m.UserID, m.Organizer, string(m.Status), m.JoinedAt)
	return err
}

func scanMembership(row pgx.Row) (*domain.Membership, error) {
	var m domain.Membership
	var status string
	if err := row.Scan(&m.ID, &m.GroupID, &m.UserID, &m.Organizer, &status, &m.JoinedAt); err != nil {
		return nil, err
	}
	m.Status = domain.MembershipStatus(status)
	return &m, nil
}

const memberCols = `id,group_id,user_id,organizer,status,joined_at`

func (s *Store) GetMembership(ctx context.Context, groupID, userID string) (*domain.Membership, error) {
	m, err := scanMembership(s.pool.QueryRow(ctx, `SELECT `+memberCols+` FROM memberships WHERE group_id=$1 AND user_id=$2`, groupID, userID))
	if err != nil {
		return nil, noRows(err)
	}
	return m, nil
}

func (s *Store) ListMembershipsByGroup(ctx context.Context, groupID string) ([]*domain.Membership, error) {
	return s.queryMemberships(ctx, `SELECT `+memberCols+` FROM memberships WHERE group_id=$1 ORDER BY joined_at`, groupID)
}

func (s *Store) ListMembershipsByUser(ctx context.Context, userID string) ([]*domain.Membership, error) {
	return s.queryMemberships(ctx, `SELECT `+memberCols+` FROM memberships WHERE user_id=$1`, userID)
}

func (s *Store) queryMemberships(ctx context.Context, q string, args ...any) ([]*domain.Membership, error) {
	rows, err := s.pool.Query(ctx, q, args...)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO join_requests (id,group_id,user_id,status,created_at,decided_at,decided_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, decided_at=EXCLUDED.decided_at, decided_by=EXCLUDED.decided_by`,
		j.ID, j.GroupID, j.UserID, string(j.Status), j.CreatedAt, nt(j.DecidedAt), j.DecidedBy)
	return err
}

func scanJoinRequest(row pgx.Row) (*domain.JoinRequest, error) {
	var j domain.JoinRequest
	var status string
	var decidedAt *time.Time
	if err := row.Scan(&j.ID, &j.GroupID, &j.UserID, &status, &j.CreatedAt, &decidedAt, &j.DecidedBy); err != nil {
		return nil, err
	}
	j.Status = domain.JoinRequestStatus(status)
	j.DecidedAt = decidedAt
	return &j, nil
}

const jrCols = `id,group_id,user_id,status,created_at,decided_at,decided_by`

func (s *Store) GetJoinRequest(ctx context.Context, id string) (*domain.JoinRequest, error) {
	j, err := scanJoinRequest(s.pool.QueryRow(ctx, `SELECT `+jrCols+` FROM join_requests WHERE id=$1`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return j, nil
}

func (s *Store) ListJoinRequestsByGroup(ctx context.Context, groupID string) ([]*domain.JoinRequest, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+jrCols+` FROM join_requests WHERE group_id=$1 ORDER BY created_at`, groupID)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO invitations (id,group_id,user_id,invited_by,status,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status`,
		i.ID, i.GroupID, i.UserID, i.InvitedBy, string(i.Status), i.CreatedAt)
	return err
}

func scanInvitation(row pgx.Row) (*domain.Invitation, error) {
	var i domain.Invitation
	var status string
	if err := row.Scan(&i.ID, &i.GroupID, &i.UserID, &i.InvitedBy, &status, &i.CreatedAt); err != nil {
		return nil, err
	}
	i.Status = domain.InvitationStatus(status)
	return &i, nil
}

const invCols = `id,group_id,user_id,invited_by,status,created_at`

func (s *Store) GetInvitation(ctx context.Context, id string) (*domain.Invitation, error) {
	i, err := scanInvitation(s.pool.QueryRow(ctx, `SELECT `+invCols+` FROM invitations WHERE id=$1`, id))
	if err != nil {
		return nil, noRows(err)
	}
	return i, nil
}

func (s *Store) ListInvitationsByUser(ctx context.Context, userID string) ([]*domain.Invitation, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+invCols+` FROM invitations WHERE user_id=$1 ORDER BY created_at`, userID)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cycles (group_id,idx,deadline,payout_user,settled)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (group_id,idx) DO UPDATE SET deadline=EXCLUDED.deadline, payout_user=EXCLUDED.payout_user, settled=EXCLUDED.settled`,
		c.GroupID, c.Index, c.Deadline, c.PayoutUser, c.Settled)
	return err
}

func scanCycle(row pgx.Row) (*domain.Cycle, error) {
	var c domain.Cycle
	if err := row.Scan(&c.GroupID, &c.Index, &c.Deadline, &c.PayoutUser, &c.Settled); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) GetCycle(ctx context.Context, groupID string, index int) (*domain.Cycle, error) {
	c, err := scanCycle(s.pool.QueryRow(ctx, `SELECT group_id,idx,deadline,payout_user,settled FROM cycles WHERE group_id=$1 AND idx=$2`, groupID, index))
	if err != nil {
		return nil, noRows(err)
	}
	return c, nil
}

func (s *Store) ListCyclesByGroup(ctx context.Context, groupID string) ([]*domain.Cycle, error) {
	rows, err := s.pool.Query(ctx, `SELECT group_id,idx,deadline,payout_user,settled FROM cycles WHERE group_id=$1 ORDER BY idx`, groupID)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contributions (id,group_id,user_id,cycle_index,amount,commitment,revealed,paid_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (group_id,user_id,cycle_index) DO UPDATE SET
			id=EXCLUDED.id, amount=EXCLUDED.amount, commitment=EXCLUDED.commitment, revealed=EXCLUDED.revealed, paid_at=EXCLUDED.paid_at`,
		c.ID, c.GroupID, c.UserID, c.CycleIndex, int64(c.Amount), c.Commitment, c.Revealed, nt(c.PaidAt))
	return err
}

func scanContribution(row pgx.Row) (*domain.Contribution, error) {
	var c domain.Contribution
	var amount int64
	var paidAt *time.Time
	if err := row.Scan(&c.ID, &c.GroupID, &c.UserID, &c.CycleIndex, &amount, &c.Commitment, &c.Revealed, &paidAt); err != nil {
		return nil, err
	}
	c.Amount = domain.Money(amount)
	c.PaidAt = paidAt
	return &c, nil
}

const contribCols = `id,group_id,user_id,cycle_index,amount,commitment,revealed,paid_at`

func (s *Store) GetContribution(ctx context.Context, groupID, userID string, cycle int) (*domain.Contribution, error) {
	c, err := scanContribution(s.pool.QueryRow(ctx, `SELECT `+contribCols+` FROM contributions WHERE group_id=$1 AND user_id=$2 AND cycle_index=$3`, groupID, userID, cycle))
	if err != nil {
		return nil, noRows(err)
	}
	return c, nil
}

func (s *Store) ListContributionsByCycle(ctx context.Context, groupID string, cycle int) ([]*domain.Contribution, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+contribCols+` FROM contributions WHERE group_id=$1 AND cycle_index=$2`, groupID, cycle)
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

// --- payouts ---

func (s *Store) SavePayout(ctx context.Context, p *domain.Payout) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO payouts (id,group_id,cycle_index,payee_user_id,gross_amount,expected_amount,shortfall,disbursement_ref,settled_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		p.ID, p.GroupID, p.CycleIndex, p.PayeeUserID, int64(p.GrossAmount), int64(p.ExpectedAmount), int64(p.Shortfall), p.DisbursementRef, p.SettledAt)
	return err
}

func (s *Store) ListPayoutsByGroup(ctx context.Context, groupID string) ([]*domain.Payout, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,group_id,cycle_index,payee_user_id,gross_amount,expected_amount,shortfall,disbursement_ref,settled_at FROM payouts WHERE group_id=$1 ORDER BY cycle_index`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Payout, 0)
	for rows.Next() {
		var p domain.Payout
		var gross, expected, shortfall int64
		if err := rows.Scan(&p.ID, &p.GroupID, &p.CycleIndex, &p.PayeeUserID, &gross, &expected, &shortfall, &p.DisbursementRef, &p.SettledAt); err != nil {
			return nil, err
		}
		p.GrossAmount, p.ExpectedAmount, p.Shortfall = domain.Money(gross), domain.Money(expected), domain.Money(shortfall)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// --- escrow ---

func (s *Store) AppendEscrowEntry(ctx context.Context, e *domain.EscrowEntry) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO escrow_entries (id,group_id,cycle_index,user_id,direction,amount,reference,memo,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, e.GroupID, e.CycleIndex, e.UserID, string(e.Direction), int64(e.Amount), e.Reference, e.Memo, e.CreatedAt)
	return err
}

func (s *Store) ListEscrowByGroup(ctx context.Context, groupID string) ([]*domain.EscrowEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,group_id,cycle_index,user_id,direction,amount,reference,memo,created_at FROM escrow_entries WHERE group_id=$1 ORDER BY created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.EscrowEntry, 0)
	for rows.Next() {
		var e domain.EscrowEntry
		var dir string
		var amount int64
		if err := rows.Scan(&e.ID, &e.GroupID, &e.CycleIndex, &e.UserID, &dir, &amount, &e.Reference, &e.Memo, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Direction = domain.LedgerDirection(dir)
		e.Amount = domain.Money(amount)
		out = append(out, &e)
	}
	return out, rows.Err()
}

// --- reputation ---

func (s *Store) AppendReputationEvent(ctx context.Context, e *domain.ReputationEvent) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO reputation_events (id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.UserID, e.GroupID, e.CycleIndex, string(e.Type), int64(e.Amount), e.ProofHash, e.Timestamp)
	return err
}

func (s *Store) ReputationEvents(ctx context.Context, userID string) ([]domain.ReputationEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp FROM reputation_events WHERE user_id=$1 ORDER BY timestamp`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ReputationEvent, 0)
	for rows.Next() {
		var e domain.ReputationEvent
		var typ string
		var amount int64
		if err := rows.Scan(&e.ID, &e.UserID, &e.GroupID, &e.CycleIndex, &typ, &amount, &e.ProofHash, &e.Timestamp); err != nil {
			return nil, err
		}
		e.Type = domain.EventType(typ)
		e.Amount = domain.Money(amount)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) AllReputationEvents(ctx context.Context, p store.Page) ([]domain.ReputationEvent, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reputation_events`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,user_id,group_id,cycle_index,type,amount,proof_hash,timestamp FROM reputation_events ORDER BY timestamp DESC LIMIT $1 OFFSET $2`, limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.ReputationEvent, 0)
	for rows.Next() {
		var e domain.ReputationEvent
		var typ string
		var amount int64
		if err := rows.Scan(&e.ID, &e.UserID, &e.GroupID, &e.CycleIndex, &typ, &amount, &e.ProofHash, &e.Timestamp); err != nil {
			return nil, 0, err
		}
		e.Type = domain.EventType(typ)
		e.Amount = domain.Money(amount)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// --- webhooks ---

func (s *Store) SaveWebhook(ctx context.Context, w *domain.Webhook) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO webhooks (id,url,secret,active,created_by,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET url=EXCLUDED.url, secret=EXCLUDED.secret, active=EXCLUDED.active`,
		w.ID, w.URL, w.Secret, w.Active, w.CreatedBy, w.CreatedAt)
	return err
}

func (s *Store) GetWebhook(ctx context.Context, id string) (*domain.Webhook, error) {
	var w domain.Webhook
	err := s.pool.QueryRow(ctx, `SELECT id,url,secret,active,created_by,created_at FROM webhooks WHERE id=$1`, id).
		Scan(&w.ID, &w.URL, &w.Secret, &w.Active, &w.CreatedBy, &w.CreatedAt)
	if err != nil {
		return nil, noRows(err)
	}
	return &w, nil
}

func (s *Store) ListWebhooks(ctx context.Context) ([]*domain.Webhook, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,url,secret,active,created_by,created_at FROM webhooks ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Webhook, 0)
	for rows.Next() {
		var w domain.Webhook
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &w.Active, &w.CreatedBy, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &w)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM webhooks WHERE id=$1`, id)
	return err
}

// --- notifications ---

func (s *Store) SaveNotification(ctx context.Context, n *domain.Notification) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (id,user_id,kind,title,body,read,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET read=EXCLUDED.read`,
		n.ID, n.UserID, string(n.Kind), n.Title, n.Body, n.Read, n.CreatedAt)
	return err
}

func (s *Store) ListNotificationsByUser(ctx context.Context, userID string) ([]*domain.Notification, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,user_id,kind,title,body,read,created_at FROM notifications WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Notification, 0)
	for rows.Next() {
		var n domain.Notification
		var kind string
		if err := rows.Scan(&n.ID, &n.UserID, &kind, &n.Title, &n.Body, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Kind = domain.NotificationKind(kind)
		out = append(out, &n)
	}
	return out, rows.Err()
}

// --- audit ---

func (s *Store) AppendAudit(ctx context.Context, a *domain.AuditEntry) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit (id,actor_id,action,target,detail,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		a.ID, a.ActorID, a.Action, a.Target, a.Detail, a.CreatedAt)
	return err
}

func (s *Store) ListAudit(ctx context.Context, p store.Page) ([]*domain.AuditEntry, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,actor_id,action,target,detail,created_at FROM audit ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit(p), p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*domain.AuditEntry, 0)
	for rows.Next() {
		var a domain.AuditEntry
		if err := rows.Scan(&a.ID, &a.ActorID, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
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
