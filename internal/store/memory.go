package store

import (
	"context"
	"sort"
	"sync"

	"rotasavings/internal/domain"
)

// Memory is a thread-safe in-memory Store for development and tests. A Postgres
// implementation (pgx) satisfies the same interface for production; see
// migrations/0001_init.sql for the schema it caches into.
type Memory struct {
	mu sync.RWMutex

	users         map[string]domain.User
	groups        map[string]domain.Group
	memberships   map[string]domain.Membership   // key: groupID|userID
	joinRequests  map[string]domain.JoinRequest  // key: id
	invitations   map[string]domain.Invitation   // key: id
	cycles        map[string]domain.Cycle        // key: groupID|index
	contributions map[string]domain.Contribution // key: groupID|userID|cycle
	payouts       []domain.Payout
	escrow        []domain.EscrowEntry
	reputation    map[string][]domain.ReputationEvent // key: userID
	notifications map[string][]domain.Notification    // key: userID
	audit         []domain.AuditEntry
	webhooks      map[string]domain.Webhook // key: id
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		users:         make(map[string]domain.User),
		groups:        make(map[string]domain.Group),
		memberships:   make(map[string]domain.Membership),
		joinRequests:  make(map[string]domain.JoinRequest),
		invitations:   make(map[string]domain.Invitation),
		cycles:        make(map[string]domain.Cycle),
		contributions: make(map[string]domain.Contribution),
		reputation:    make(map[string][]domain.ReputationEvent),
		notifications: make(map[string][]domain.Notification),
		webhooks:      make(map[string]domain.Webhook),
	}
}

// Health always succeeds for the in-memory store.
func (m *Memory) Health(_ context.Context) error { return nil }

func memberKey(groupID, userID string) string { return groupID + "|" + userID }
func cycleKey(groupID string, index int) string {
	return groupID + "|" + itoa(index)
}
func contribKey(groupID, userID string, cycle int) string {
	return groupID + "|" + userID + "|" + itoa(cycle)
}
func itoa(i int) string {
	// small non-negative ints only; avoids strconv import churn
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

// --- users ---

func (m *Memory) SaveUser(_ context.Context, u *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = *u
	return nil
}

func (m *Memory) GetUser(_ context.Context, id string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := u
	return &out, nil
}

func (m *Memory) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.Email == email {
			out := u
			return &out, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *Memory) ListUsers(_ context.Context, p Page) ([]*domain.User, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		all = append(all, u)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	total := len(all)
	out := make([]*domain.User, 0)
	for i := range paginate(len(all), p) {
		u := all[i]
		out = append(out, &u)
	}
	return out, total, nil
}

func (m *Memory) ListUsersByKYC(_ context.Context, status domain.KYCStatus) ([]*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.User, 0)
	for _, u := range m.users {
		if u.KYCStatus == status {
			uu := u
			out = append(out, &uu)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// --- groups ---

func (m *Memory) SaveGroup(_ context.Context, g *domain.Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups[g.ID] = *g
	return nil
}

func (m *Memory) GetGroup(_ context.Context, id string) (*domain.Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.groups[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := g
	return &out, nil
}

func (m *Memory) ListGroups(_ context.Context, p Page) ([]*domain.Group, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.Group, 0, len(m.groups))
	for _, g := range m.groups {
		all = append(all, g)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	total := len(all)
	out := make([]*domain.Group, 0)
	for i := range paginate(len(all), p) {
		g := all[i]
		out = append(out, &g)
	}
	return out, total, nil
}

// --- memberships ---

func (m *Memory) SaveMembership(_ context.Context, mem *domain.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberships[memberKey(mem.GroupID, mem.UserID)] = *mem
	return nil
}

func (m *Memory) GetMembership(_ context.Context, groupID, userID string) (*domain.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mem, ok := m.memberships[memberKey(groupID, userID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := mem
	return &out, nil
}

func (m *Memory) ListMembershipsByGroup(_ context.Context, groupID string) ([]*domain.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Membership, 0)
	for _, mem := range m.memberships {
		if mem.GroupID == groupID {
			mm := mem
			out = append(out, &mm)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

func (m *Memory) ListMembershipsByUser(_ context.Context, userID string) ([]*domain.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Membership, 0)
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			mm := mem
			out = append(out, &mm)
		}
	}
	return out, nil
}

// --- join requests & invitations ---

func (m *Memory) SaveJoinRequest(_ context.Context, j *domain.JoinRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinRequests[j.ID] = *j
	return nil
}

func (m *Memory) GetJoinRequest(_ context.Context, id string) (*domain.JoinRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.joinRequests[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := j
	return &out, nil
}

func (m *Memory) ListJoinRequestsByGroup(_ context.Context, groupID string) ([]*domain.JoinRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.JoinRequest, 0)
	for _, j := range m.joinRequests {
		if j.GroupID == groupID {
			jj := j
			out = append(out, &jj)
		}
	}
	sort.Slice(out, func(i, j2 int) bool { return out[i].CreatedAt.Before(out[j2].CreatedAt) })
	return out, nil
}

func (m *Memory) SaveInvitation(_ context.Context, i *domain.Invitation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invitations[i.ID] = *i
	return nil
}

func (m *Memory) GetInvitation(_ context.Context, id string) (*domain.Invitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.invitations[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := i
	return &out, nil
}

func (m *Memory) ListInvitationsByUser(_ context.Context, userID string) ([]*domain.Invitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Invitation, 0)
	for _, i := range m.invitations {
		if i.UserID == userID {
			ii := i
			out = append(out, &ii)
		}
	}
	return out, nil
}

// --- cycles ---

func (m *Memory) SaveCycle(_ context.Context, c *domain.Cycle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cycles[cycleKey(c.GroupID, c.Index)] = *c
	return nil
}

func (m *Memory) GetCycle(_ context.Context, groupID string, index int) (*domain.Cycle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.cycles[cycleKey(groupID, index)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := c
	return &out, nil
}

func (m *Memory) ListCyclesByGroup(_ context.Context, groupID string) ([]*domain.Cycle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Cycle, 0)
	for _, c := range m.cycles {
		if c.GroupID == groupID {
			cc := c
			out = append(out, &cc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}

// --- contributions ---

func (m *Memory) SaveContribution(_ context.Context, c *domain.Contribution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contributions[contribKey(c.GroupID, c.UserID, c.CycleIndex)] = *c
	return nil
}

func (m *Memory) GetContribution(_ context.Context, groupID, userID string, cycle int) (*domain.Contribution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.contributions[contribKey(groupID, userID, cycle)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := c
	return &out, nil
}

func (m *Memory) ListContributionsByCycle(_ context.Context, groupID string, cycle int) ([]*domain.Contribution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Contribution, 0)
	for _, c := range m.contributions {
		if c.GroupID == groupID && c.CycleIndex == cycle {
			cc := c
			out = append(out, &cc)
		}
	}
	return out, nil
}

// --- payouts ---

func (m *Memory) SavePayout(_ context.Context, p *domain.Payout) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payouts = append(m.payouts, *p)
	return nil
}

func (m *Memory) ListPayoutsByGroup(_ context.Context, groupID string) ([]*domain.Payout, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Payout, 0)
	for _, p := range m.payouts {
		if p.GroupID == groupID {
			pp := p
			out = append(out, &pp)
		}
	}
	return out, nil
}

// --- escrow ---

func (m *Memory) AppendEscrowEntry(_ context.Context, e *domain.EscrowEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.escrow = append(m.escrow, *e)
	return nil
}

func (m *Memory) ListEscrowByGroup(_ context.Context, groupID string) ([]*domain.EscrowEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.EscrowEntry, 0)
	for _, e := range m.escrow {
		if e.GroupID == groupID {
			ee := e
			out = append(out, &ee)
		}
	}
	return out, nil
}

// --- reputation ---

func (m *Memory) AppendReputationEvent(_ context.Context, e *domain.ReputationEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reputation[e.UserID] = append(m.reputation[e.UserID], *e)
	return nil
}

func (m *Memory) ReputationEvents(_ context.Context, userID string) ([]domain.ReputationEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.reputation[userID]
	out := make([]domain.ReputationEvent, len(src))
	copy(out, src)
	return out, nil
}

func (m *Memory) AllReputationEvents(_ context.Context, p Page) ([]domain.ReputationEvent, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.ReputationEvent, 0)
	for _, evs := range m.reputation {
		all = append(all, evs...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp.After(all[j].Timestamp) })
	total := len(all)
	out := make([]domain.ReputationEvent, 0)
	for i := range paginate(len(all), p) {
		out = append(out, all[i])
	}
	return out, total, nil
}

func (m *Memory) SaveWebhook(_ context.Context, w *domain.Webhook) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhooks[w.ID] = *w
	return nil
}

func (m *Memory) GetWebhook(_ context.Context, id string) (*domain.Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.webhooks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := w
	return &out, nil
}

func (m *Memory) ListWebhooks(_ context.Context) ([]*domain.Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Webhook, 0, len(m.webhooks))
	for _, w := range m.webhooks {
		ww := w
		out = append(out, &ww)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *Memory) DeleteWebhook(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.webhooks, id)
	return nil
}

// --- notifications ---

func (m *Memory) SaveNotification(_ context.Context, n *domain.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications[n.UserID] = append(m.notifications[n.UserID], *n)
	return nil
}

func (m *Memory) ListNotificationsByUser(_ context.Context, userID string) ([]*domain.Notification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.notifications[userID]
	out := make([]*domain.Notification, 0, len(src))
	for i := range src {
		n := src[i]
		out = append(out, &n)
	}
	return out, nil
}

// --- audit ---

func (m *Memory) AppendAudit(_ context.Context, a *domain.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = append(m.audit, *a)
	return nil
}

func (m *Memory) ListAudit(_ context.Context, p Page) ([]*domain.AuditEntry, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.audit)
	// newest first
	all := make([]domain.AuditEntry, total)
	copy(all, m.audit)
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	out := make([]*domain.AuditEntry, 0)
	for i := range paginate(len(all), p) {
		a := all[i]
		out = append(out, &a)
	}
	return out, total, nil
}

// paginate returns the index range [start,end) to slice, given a total count.
func paginate(total int, p Page) []int {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	start := p.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + p.Limit
	if end > total {
		end = total
	}
	idx := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		idx = append(idx, i)
	}
	return idx
}
