# ROTASAVINGS

Hybrid ROSCA infrastructure (ajo / esusu / stokvel). The blockchain is the
system of truth; a Go orchestration layer drives it; a pure-Go intelligence
engine advises (never enforces); and a Next.js dashboard operates it.

This repository now contains all four layers:

- Go backend (orchestration, auth, payments, scheduler, intelligence, admin API)
- Solidity contracts (the truth layer) + a go-ethereum client behind the same interface
- Real persistence (SQLite for dev, PostgreSQL for production, one interface)
- Next.js admin dashboard

## Architectural rules (enforced by package boundaries)

1. The chain decides truth. `internal/chain.TruthLayer` is the only source of truth.
2. The database is a cache for chain projections and the source of truth for
   off-chain application state (auth, KYC, memberships, escrow, audit) -
   see `migrations/0001_init.sql` for the exact split.
3. One projection path. Writes go to the chain first; the indexer folds emitted
   events into the cache. Reputation is never written directly by handlers.
4. Payments execute, they do not decide. `internal/payments` moves money on
   instruction and is reconciled into the escrow ledger.
5. Intelligence advises, never enforces.

## Repository layout

```
.
|- cmd/server/            Entrypoint; wires layers, runs HTTP + indexer + scheduler
|- internal/
|  |- domain/             Entities, lifecycle rules, commitments, reputation/escrow folds
|  |- auth/               PBKDF2 hashing, HS256 access + refresh tokens, RBAC middleware
|  |- chain/              TruthLayer interface + in-memory dev chain
|  |  \- evm/             go-ethereum implementation of TruthLayer
|  |- store/              Store interface + in-memory impl
|  |  |- sqlitestore/     On-disk SQLite store (database/sql, no CGO)
|  |  \- pgstore/         PostgreSQL store (pgx/v5)
|  |- payments/           Provider interface + mock (mobile money / bank later)
|  |- notify/             Notifier interface + log impl
|  |- indexer/            Projects chain events into the cache
|  |- scheduler/          Deadline heartbeat: records defaults + settles cycles
|  |- intelligence/       Risk scoring, group optimizer, behavioral monitor, liquidity stress
|  |- app/                Orchestration service
|  |- httpapi/            API gateway + admin API, middleware, OpenAPI spec
|  \- config/             Env-driven config
|- migrations/            PostgreSQL schema (source of truth for the schema)
|- contracts/             Solidity contracts (IdentityRegistry, ReputationLedger, RotasavingsGroup)
|- frontend/              Next.js 15 admin dashboard
|- Dockerfile, docker-compose.yml, .github/workflows/ci.yml
```

## Quick start (backend only, zero infrastructure)

```bash
go run ./cmd/server          # listens on :8080, cache persisted to ./rota.db
```

By default it uses the in-memory dev chain, an on-disk SQLite cache, and a mock
payment provider, and seeds an admin account. No external services required.

### With the full stack (Postgres + backend) via Docker

```bash
docker compose up --build     # backend on :8080, Postgres on :5432
```

### With the admin dashboard

```bash
cd frontend && npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev   # http://localhost:3000
```

Sign in with the seeded admin (`admin@rotasavings.local` / `changeme123`).

## Datastore selection

All three implementations satisfy `store.Store`; selection is by environment:

| Setting                       | Store used                          |
|-------------------------------|-------------------------------------|
| `ROTA_POSTGRES_DSN` set       | PostgreSQL (pgx)                    |
| `ROTA_DB_PATH=memory`         | In-memory (ephemeral)              |
| default (`ROTA_DB_PATH=rota.db`) | On-disk SQLite (persists)        |

The SQLite store is written against `database/sql`; the Postgres store is the
same logic in pgx with native `TIMESTAMPTZ`/`TEXT[]`. Both self-apply the schema
on boot. Inspect the SQLite file directly: `sqlite3 rota.db ".tables"`.

## Truth-layer selection

| Setting                 | Truth layer used                          |
|-------------------------|-------------------------------------------|
| `ROTA_CHAIN_RPC_URL` set| EVM (go-ethereum, `internal/chain/evm`)   |
| default                 | In-memory dev chain                       |

The EVM client connects to a node, anchors identity commitments as transactions,
and subscribes to `ReputationLedger` events, decoding them into the same
`domain.ReputationEvent` the indexer projects. See "Smart contracts" below for
the boundary on group-lifecycle writes.

## API

Authenticate with `Authorization: Bearer <access_token>` from `/v1/auth/login`;
renew via `/v1/auth/refresh`. Mutating requests accept an optional
`Idempotency-Key` header. The full machine-readable spec is served at
`GET /openapi.yaml`.

Operational endpoints:

| Endpoint        | Purpose                                     |
|-----------------|---------------------------------------------|
| `GET /healthz`  | Liveness (process up)                       |
| `GET /readyz`   | Readiness (datastore reachable)             |
| `GET /metrics`  | Prometheus metrics (text exposition)        |
| `GET /openapi.yaml` | OpenAPI 3 specification                  |

Member endpoints (selection): register/login/refresh, profile, groups CRUD,
join requests, invitations, leave/remove members, activate, cycles, contribute,
settle, reputation, risk score, behavioral monitor, group liquidity.

Admin endpoints (role `admin`): overview KPIs, paginated users, suspend/activate,
KYC review queue and decision, liquidity board, audit trail.

## Cross-cutting hardening

- Auth: PBKDF2 password hashing; HS256 access tokens (1h) + refresh tokens (30d);
  RBAC and organizer-ownership checks.
- HTTP middleware chain: panic recovery, request IDs (honored from
  `X-Request-ID`), access logs with status and latency, per-client token-bucket
  rate limiting (429 + `Retry-After`), idempotency-key replay, metrics.
- Observability: structured logs, request IDs, Prometheus `/metrics`,
  liveness/readiness probes.

## Smart contracts

`contracts/src` holds the three contracts that mirror `chain.TruthLayer`:

- `IdentityRegistry` - anchors KYC identity commitments (hash only).
- `ReputationLedger` - append-only, event-based reputation; authorized writers.
- `RotasavingsGroup` - one ROSCA group: rules, lifecycle, contributions, settlement.

Build/test with Foundry: `cd contracts && forge build && forge test`.

The Go EVM client fully implements identity registration and event subscription.
The group-lifecycle writes (deploy/activate/contribute/settle) are emitted
on-chain by `RotasavingsGroup` itself; reconciling the off-chain commitment
scheme (sha256 over string IDs) with the on-chain scheme
(keccak over addresses) is the documented remaining step - those methods return
a descriptive `ErrNotWired` rather than pretending. See `internal/chain/evm`.

## Intelligence engines (pure Go, explainable)

- Default-risk scoring (logistic over reputation + off-chain signals).
- Group-formation optimizer (snake-draft risk balancing).
- Behavioral monitor (early-warning flags during a cycle).
- Liquidity-stress predictor (per-group collapse probability).

These are transparent heuristics, not opaque ML; weights are explicit constants.
Model training/backtesting and an off-chain feature pipeline remain future work.

## Configuration (environment)

| Var                       | Default                         | Notes                               |
|---------------------------|---------------------------------|-------------------------------------|
| `ROTA_HTTP_ADDR`          | `:8080`                         | Listen address                      |
| `ROTA_JWT_SECRET`         | `dev-insecure-secret-change-me` | Set in production                   |
| `ROTA_JWT_TTL`            | `1h`                            | Access token lifetime               |
| `ROTA_JWT_REFRESH_TTL`    | `720h`                          | Refresh token lifetime              |
| `ROTA_ADMIN_EMAIL`        | `admin@rotasavings.local`       | Seeded admin                        |
| `ROTA_ADMIN_PASSWORD`     | `changeme123`                   | Change in production                |
| `ROTA_SCHEDULER_INTERVAL` | `30s`                           | Deadline heartbeat cadence          |
| `ROTA_DB_PATH`            | `rota.db`                       | SQLite path; `memory` = ephemeral   |
| `ROTA_POSTGRES_DSN`       | (unset)                         | If set, use Postgres                |
| `ROTA_CHAIN_RPC_URL`      | (unset)                         | If set, use the EVM truth layer     |
| `ROTA_CHAIN_PRIVATE_KEY`  | (unset)                         | Signer for on-chain writes          |
| `ROTA_IDENTITY_REGISTRY`  | (unset)                         | IdentityRegistry address            |
| `ROTA_REPUTATION_LEDGER`  | (unset)                         | ReputationLedger address            |

## Verification

```bash
go vet ./... && go test -race ./... && go build ./...   # backend
cd frontend && npm run build                            # frontend
cd contracts && forge build && forge test               # contracts (needs Foundry)
```

CI (`.github/workflows/ci.yml`) runs all three on push and pull request.

## Remaining work (honest)

- Reconcile the commitment scheme so the EVM group-lifecycle writes are driven
  on-chain end to end (contracts and identity/subscribe paths are done).
- Real payment adapters (M-Pesa, MTN MoMo, Paystack, Flutterwave, bank) behind
  `payments.Provider` (mock today).
- Real notification channels (FCM/APNs/SMS/email) behind `notify.Notifier`.
- ML model training/backtesting and an off-chain feature pipeline.
- A member-facing app (the admin dashboard is built; the member UI is not).
