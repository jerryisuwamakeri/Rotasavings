-- ROTASAVINGS application database (SQLite dialect).
--
-- This holds TWO kinds of data and the distinction matters (see
-- migrations/0001_init.sql for the full rationale):
--
--   1. ON-CHAIN PROJECTIONS (read cache; rebuildable from chain events):
--      groups' on-chain config, cycles, contributions, payouts, reputation_events.
--   2. OFF-CHAIN APPLICATION STATE (this DB is the source of truth; back it up):
--      users (auth/KYC/role), memberships, join_requests, invitations,
--      escrow_entries (mirrors the payment provider), notifications, audit.
--
-- Storage encoding: times as RFC3339Nano TEXT, money/durations as INTEGER,
-- booleans as INTEGER (0/1), string arrays as JSON TEXT. The Postgres schema in
-- migrations/0001_init.sql is the equivalent with native TEXT[]/TIMESTAMPTZ and
-- foreign keys.

CREATE TABLE IF NOT EXISTS users (
    id                  TEXT PRIMARY KEY,
    email               TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    wallet_address      TEXT NOT NULL,
    identity_commitment TEXT NOT NULL,
    kyc_provider        TEXT NOT NULL DEFAULT '',
    kyc_status          TEXT NOT NULL,
    role                TEXT NOT NULL,
    status              TEXT NOT NULL,
    password_hash       TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS groups (
    id                  TEXT PRIMARY KEY,
    contract_address    TEXT NOT NULL,
    name                TEXT NOT NULL,
    organizer_id        TEXT NOT NULL,
    contribution_amount INTEGER NOT NULL,
    cycle_length_ns     INTEGER NOT NULL,
    max_members         INTEGER NOT NULL,
    total_cycles        INTEGER NOT NULL,
    members_json        TEXT NOT NULL DEFAULT '[]',
    payout_order_json   TEXT NOT NULL DEFAULT '[]',
    state               TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    activated_at        TEXT
);

CREATE TABLE IF NOT EXISTS memberships (
    id        TEXT PRIMARY KEY,
    group_id  TEXT NOT NULL,
    user_id   TEXT NOT NULL,
    organizer INTEGER NOT NULL DEFAULT 0,
    status    TEXT NOT NULL,
    joined_at TEXT NOT NULL,
    UNIQUE (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_memberships_group ON memberships(group_id);
CREATE INDEX IF NOT EXISTS idx_memberships_user  ON memberships(user_id);

CREATE TABLE IF NOT EXISTS join_requests (
    id         TEXT PRIMARY KEY,
    group_id   TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    status     TEXT NOT NULL,
    created_at TEXT NOT NULL,
    decided_at TEXT,
    decided_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_join_requests_group ON join_requests(group_id);

CREATE TABLE IF NOT EXISTS invitations (
    id         TEXT PRIMARY KEY,
    group_id   TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    invited_by TEXT NOT NULL,
    status     TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_invitations_user ON invitations(user_id);

CREATE TABLE IF NOT EXISTS cycles (
    group_id    TEXT NOT NULL,
    idx         INTEGER NOT NULL,
    deadline    TEXT NOT NULL,
    payout_user TEXT NOT NULL,
    settled     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, idx)
);

CREATE TABLE IF NOT EXISTS contributions (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    cycle_index INTEGER NOT NULL,
    amount      INTEGER NOT NULL,
    commitment  TEXT NOT NULL,
    revealed    INTEGER NOT NULL DEFAULT 0,
    paid_at     TEXT,
    UNIQUE (group_id, user_id, cycle_index)
);
CREATE INDEX IF NOT EXISTS idx_contribs_group_cycle ON contributions(group_id, cycle_index);

CREATE TABLE IF NOT EXISTS payouts (
    id               TEXT PRIMARY KEY,
    group_id         TEXT NOT NULL,
    cycle_index      INTEGER NOT NULL,
    payee_user_id    TEXT NOT NULL,
    gross_amount     INTEGER NOT NULL,
    expected_amount  INTEGER NOT NULL,
    shortfall        INTEGER NOT NULL,
    disbursement_ref TEXT NOT NULL,
    settled_at       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payouts_group ON payouts(group_id);

CREATE TABLE IF NOT EXISTS escrow_entries (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL,
    cycle_index INTEGER NOT NULL,
    user_id     TEXT NOT NULL DEFAULT '',
    direction   TEXT NOT NULL,
    amount      INTEGER NOT NULL,
    reference   TEXT NOT NULL DEFAULT '',
    memo        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_escrow_group ON escrow_entries(group_id);

CREATE TABLE IF NOT EXISTS reputation_events (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    group_id    TEXT NOT NULL,
    cycle_index INTEGER NOT NULL,
    type        TEXT NOT NULL,
    amount      INTEGER NOT NULL DEFAULT 0,
    proof_hash  TEXT NOT NULL,
    timestamp   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reputation_user ON reputation_events(user_id);

CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    read       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);

CREATE TABLE IF NOT EXISTS audit (
    id         TEXT PRIMARY KEY,
    actor_id   TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS webhooks (
    id         TEXT PRIMARY KEY,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL DEFAULT '',
    active     INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
