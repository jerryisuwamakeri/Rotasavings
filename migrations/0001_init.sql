-- ROTASAVINGS application database (PostgreSQL).
--
-- This database holds TWO kinds of data, and the distinction is load-bearing —
-- earlier revisions of this file wrongly claimed every row was a rebuildable
-- on-chain projection. That is not true. Be precise:
--
--   1. ON-CHAIN PROJECTIONS  (read cache; the chain is the source of truth)
--      Rows that mirror state whose authority is the blockchain. They are
--      rebuildable by replaying chain events from block height, and must never
--      encode a decision the chain does not also record. If lost, re-index them.
--          groups (the activated on-chain config: amount, cycle, payout order,
--                  state), cycles, contributions, payouts, reputation_events.
--
--   2. OFF-CHAIN APPLICATION STATE  (THIS database is the source of truth)
--      Data the chain does not — and must not — hold. It cannot be reconstructed
--      from the chain, so it must be backed up like any primary datastore.
--          users (auth credentials, KYC review outcome, role/status),
--          memberships, join_requests, invitations (the pre-activation
--          assembly workflow), escrow_entries (mirrors the PAYMENT PROVIDER,
--          not the chain), notifications, audit.
--
-- Conventions: money is BIGINT in the smallest currency unit (kobo/cents),
-- never floats; durations are BIGINT nanoseconds to match the Go domain model;
-- string arrays use native TEXT[]; timestamps are TIMESTAMPTZ.

BEGIN;

-- ───────────────────────── off-chain application state ─────────────────────────

-- Users: auth + KYC + role. The chain stores only identity_commitment; the
-- password hash and PII live here and nowhere else.
CREATE TABLE IF NOT EXISTS users (
    id                  TEXT PRIMARY KEY,
    email               TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    wallet_address      TEXT NOT NULL UNIQUE,
    identity_commitment TEXT NOT NULL UNIQUE,
    kyc_provider        TEXT NOT NULL DEFAULT '',
    kyc_status          TEXT NOT NULL DEFAULT 'pending'
                          CHECK (kyc_status IN ('pending','approved','rejected')),
    role                TEXT NOT NULL DEFAULT 'member'
                          CHECK (role IN ('member','admin')),
    status              TEXT NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active','suspended')),
    password_hash       TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Groups: the pre-activation assembly workflow is off-chain application state;
-- once ACTIVE the parameters (amount, cycle length, payout order) become the
-- immutable on-chain config this row mirrors.
CREATE TABLE IF NOT EXISTS groups (
    id                  TEXT PRIMARY KEY,
    contract_address    TEXT NOT NULL DEFAULT '',
    name                TEXT NOT NULL,
    organizer_id        TEXT NOT NULL REFERENCES users(id),
    contribution_amount BIGINT NOT NULL CHECK (contribution_amount > 0),
    cycle_length_ns     BIGINT NOT NULL CHECK (cycle_length_ns > 0),
    max_members         INTEGER NOT NULL CHECK (max_members >= 2),
    total_cycles        INTEGER NOT NULL DEFAULT 0 CHECK (total_cycles >= 0),
    members             TEXT[] NOT NULL DEFAULT '{}',
    payout_order        TEXT[] NOT NULL DEFAULT '{}',
    state               TEXT NOT NULL DEFAULT 'CREATED'
                          CHECK (state IN ('CREATED','ACTIVE','SETTLEMENT','CLOSED')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at        TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_groups_organizer ON groups(organizer_id);
CREATE INDEX IF NOT EXISTS idx_groups_state ON groups(state);

-- Memberships: who is in a group, and the organizer flag.
CREATE TABLE IF NOT EXISTS memberships (
    id        TEXT PRIMARY KEY,
    group_id  TEXT NOT NULL REFERENCES groups(id),
    user_id   TEXT NOT NULL REFERENCES users(id),
    organizer BOOLEAN NOT NULL DEFAULT FALSE,
    status    TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active','left','removed')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_memberships_user ON memberships(user_id);

-- Join requests: a user applying to a group, decided by the organizer.
CREATE TABLE IF NOT EXISTS join_requests (
    id         TEXT PRIMARY KEY,
    group_id   TEXT NOT NULL REFERENCES groups(id),
    user_id    TEXT NOT NULL REFERENCES users(id),
    status     TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','approved','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    decided_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_join_requests_group ON join_requests(group_id);

-- Invitations: an organizer inviting a user into a group.
CREATE TABLE IF NOT EXISTS invitations (
    id         TEXT PRIMARY KEY,
    group_id   TEXT NOT NULL REFERENCES groups(id),
    user_id    TEXT NOT NULL REFERENCES users(id),
    invited_by TEXT NOT NULL REFERENCES users(id),
    status     TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','accepted','declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invitations_user ON invitations(user_id);

-- Escrow ledger: append-only. Mirrors the PAYMENT PROVIDER (collected funds
-- held until settlement), not the chain. Never UPDATE/DELETE.
CREATE TABLE IF NOT EXISTS escrow_entries (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL REFERENCES groups(id),
    cycle_index INTEGER NOT NULL,
    user_id     TEXT NOT NULL DEFAULT '',
    direction   TEXT NOT NULL CHECK (direction IN ('credit','debit')),
    amount      BIGINT NOT NULL CHECK (amount >= 0),
    reference   TEXT NOT NULL DEFAULT '',
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_escrow_group ON escrow_entries(group_id);

-- Notifications: in-app feed; delivery happens out of band.
CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);

-- Audit trail: append-only record of privileged (admin) actions.
CREATE TABLE IF NOT EXISTS audit (
    id         TEXT PRIMARY KEY,
    actor_id   TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit(created_at DESC);

-- ───────────────────────── on-chain projections (cache) ─────────────────────────

-- Cycles: the materialised schedule of an activated group. Projection of the
-- on-chain group config.
CREATE TABLE IF NOT EXISTS cycles (
    group_id    TEXT NOT NULL REFERENCES groups(id),
    idx         INTEGER NOT NULL CHECK (idx >= 0),
    deadline    TIMESTAMPTZ NOT NULL,
    payout_user TEXT NOT NULL,
    settled     BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (group_id, idx)
);

-- Contributions: projection of on-chain contribution commitments / reveals.
CREATE TABLE IF NOT EXISTS contributions (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL REFERENCES groups(id),
    user_id     TEXT NOT NULL REFERENCES users(id),
    cycle_index INTEGER NOT NULL CHECK (cycle_index >= 0),
    amount      BIGINT NOT NULL CHECK (amount > 0),
    commitment  TEXT NOT NULL,
    revealed    BOOLEAN NOT NULL DEFAULT FALSE,
    paid_at     TIMESTAMPTZ,
    UNIQUE (group_id, user_id, cycle_index)
);
CREATE INDEX IF NOT EXISTS idx_contributions_group_cycle ON contributions(group_id, cycle_index);

-- Payouts: append-only projection of settled-cycle disbursements.
CREATE TABLE IF NOT EXISTS payouts (
    id               TEXT PRIMARY KEY,
    group_id         TEXT NOT NULL REFERENCES groups(id),
    cycle_index      INTEGER NOT NULL CHECK (cycle_index >= 0),
    payee_user_id    TEXT NOT NULL REFERENCES users(id),
    gross_amount     BIGINT NOT NULL CHECK (gross_amount >= 0),
    expected_amount  BIGINT NOT NULL CHECK (expected_amount >= 0),
    shortfall        BIGINT NOT NULL,
    disbursement_ref TEXT NOT NULL DEFAULT '',
    settled_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payouts_group ON payouts(group_id);

-- Reputation ledger: append-only projection of on-chain reputation events.
-- INSERT-only; never UPDATE/DELETE — it mirrors immutable on-chain history.
CREATE TABLE IF NOT EXISTS reputation_events (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    group_id    TEXT NOT NULL,
    cycle_index INTEGER NOT NULL,
    type        TEXT NOT NULL
                  CHECK (type IN ('ContributionMade','ContributionMissed',
                                  'PayoutReceived','GroupExit','GroupExpulsion')),
    amount      BIGINT NOT NULL DEFAULT 0,
    proof_hash  TEXT NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reputation_events_user ON reputation_events(user_id);


CREATE TABLE IF NOT EXISTS webhooks (
    id         TEXT PRIMARY KEY,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL DEFAULT '',
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
