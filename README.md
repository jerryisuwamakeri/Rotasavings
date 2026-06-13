# ROTASAVINGS — Backend (Go)

Hybrid ROSCA infrastructure: the **blockchain is the system of truth**, the Go
backend is a **an orchestration + caching layer**, and a **pure-Go
intelligence engine** advises (but never enforces) decisions.

This repository contains a **substantial Go orchestration backend**: auth + RBAC,
the full group/membership lifecycle, cycles, contributions, escrow, payouts and
settlement, a default-detection scheduler, notifications, four intelligence
engines, and a complete admin API. The EVM smart-contract layer, the real
payment adapters, the Postgres driver, and the Next.js frontend are the
remaining workstreams — and they all plug in behind interfaces that already
exist here, with working in-memory/mock implementations for development.

## Architectural rules (enforced by the package boundaries)

1. **The chain decides truth.** `internal/chain.TruthLayer` is the only source
   of truth. Everything else is downstream.
2. **Postgres is cache only.** `internal/store.Store` holds projections that can
   be rebuilt by replaying chain events. Never the source of truth.
3. **One projection path.** Writes go to the chain first; the **indexer** folds
   emitted events into the cache. Reputation is *never* written directly by
   request handlers.
4. **Payments execute, they don't decide.** `internal/payments` moves money on
   instruction and is reconciled into the escrow ledger; it is downstream of the
   chain.
5. **Intelligence advises, never enforces.** `internal/intelligence` produces
   scores, flags, and suggestions; the contracts enforce the rules.

## Layout

| Path                     | Responsibility                                                       |
|--------------------------|---------------------------------------------------------------------|
| `cmd/server`             | Entrypoint; wires layers, runs HTTP + indexer + scheduler           |
| `internal/domain`        | Entities, lifecycle rules, commitments, reputation/escrow folds     |
| `internal/auth`          | PBKDF2 password hashing, HS256 JWTs, auth + RBAC middleware          |
| `internal/chain`         | `TruthLayer` interface + in-memory dev impl (**EVM later**)         |
| `internal/store`         | `Store` interface + in-memory impl + **on-disk SQLite impl**         |
| `internal/store/sqlitestore` | Real SQL cache (`database/sql`, pure-Go SQLite, survives restarts) |
| `internal/payments`      | `Provider` interface + mock impl (**mobile money / bank later**)    |
| `internal/notify`        | `Notifier` interface + log impl (**FCM / SMS / email later**)       |
| `internal/indexer`       | Projects chain events → cache (the one-way data flow)               |
| `internal/scheduler`     | Deadline heartbeat: records defaults + settles cycles               |
| `internal/intelligence`  | Risk scoring, group optimizer, behavioral monitor, liquidity stress |
| `internal/app`           | Orchestration service (chain-first; payments; cache)                |
| `internal/httpapi`       | API gateway + admin API (`net/http` 1.22 routing)                  |
| `internal/config`        | Env-driven config                                                   |
| `migrations/0001_init.sql` | The Postgres cache schema                                         |

## Run

```bash
go run ./cmd/server          # listens on :8080 (override with ROTA_HTTP_ADDR)
```

By default the cache is a **real on-disk SQLite database** (`rota.db`) that
persists across restarts — kill the server, start it again, your users, groups,
contributions, and ledgers are still there. The chain, payments, and notifier
are still in-process dev implementations. On boot it seeds an admin account
(`ROTA_ADMIN_EMAIL` / `ROTA_ADMIN_PASSWORD`).

```bash
go run ./cmd/server                    # cache persisted to ./rota.db
ROTA_DB_PATH=/tmp/rota.db go run ./cmd/server   # custom path
ROTA_DB_PATH=memory     go run ./cmd/server   # ephemeral in-memory cache
sqlite3 rota.db ".tables"              # inspect the live database with the CLI
```

The SQLite store (`internal/store/sqlitestore`) is written against
`database/sql` with a pure-Go driver (`modernc.org/sqlite` — no CGO, no server),
so the same code ports to Postgres/pgx with only dialect changes. The Postgres
production schema lives in `migrations/0001_init.sql`.

## How it fits together (request → truth → cache)

```
            ┌──────────── HTTP (auth + RBAC) ────────────┐
client ───▶ │  httpapi → app.Service                     │
            └───────┬──────────────┬─────────────┬───────┘
                    │ 1. charge    │ 2. write    │
                    ▼              ▼             │
              payments.Provider  chain.TruthLayer (TRUTH)
                    │              │ emits events │
                    │              ▼              │
                    │           indexer ──────────┘ 3. project
                    ▼              ▼
              escrow ledger ───▶ store.Store (CACHE)
                                   ▲
                scheduler ─────────┘  (deadlines → defaults + settlement)
```

## API

Auth is via `Authorization: Bearer <token>` from `/v1/auth/login`. Member routes
require a valid token; admin routes require the `admin` role.

**Public**
| Method & path             | Purpose                       |
|---------------------------|-------------------------------|
| `GET  /healthz`           | Liveness (process is up)      |
| `GET  /readyz`            | Readiness (datastore answers) |
| `POST /v1/auth/register`  | Create a member (KYC pending) |
| `POST /v1/auth/login`     | Exchange credentials for a JWT|

**Member**
| Method & path                                   | Purpose                          |
|-------------------------------------------------|----------------------------------|
| `GET  /v1/me`                                   | Current user                     |
| `PATCH /v1/me`                                  | Update profile                   |
| `GET  /v1/me/notifications`                     | Notification feed                |
| `GET  /v1/me/invitations`                       | My group invitations             |
| `GET  /v1/users/{id}/reputation`                | Deterministic reputation summary |
| `POST /v1/users/{id}/risk-score`                | Default-risk score               |
| `POST /v1/groups`                               | Create + deploy a group          |
| `GET  /v1/groups`                               | List groups (paginated)          |
| `GET  /v1/groups/{id}`                          | Group detail                     |
| `GET  /v1/groups/{id}/members`                  | List members                     |
| `POST /v1/groups/{id}/join-requests`            | Request to join                  |
| `GET  /v1/groups/{id}/join-requests`            | List requests (organizer)        |
| `POST /v1/join-requests/{id}/decision`          | Approve/reject (organizer)       |
| `POST /v1/groups/{id}/invitations`              | Invite a user (organizer)        |
| `POST /v1/invitations/{id}/response`            | Accept/decline an invite         |
| `POST /v1/groups/{id}/leave`                    | Leave (while CREATED)            |
| `DELETE /v1/groups/{id}/members/{userID}`       | Remove member (organizer)        |
| `POST /v1/groups/{id}/activate`                 | Fix payout order, go ACTIVE      |
| `GET  /v1/groups/{id}/cycles`                   | Cycle schedule                   |
| `GET  /v1/groups/{id}/cycles/current`           | Current open cycle               |
| `GET  /v1/groups/{id}/cycles/{index}/status`    | Per-member paid/unpaid           |
| `POST /v1/groups/{id}/contributions`            | Contribute (charges + reveals)   |
| `POST /v1/groups/{id}/cycles/{index}/settle`    | Settle a cycle (organizer)       |
| `GET  /v1/groups/{id}/monitor`                  | Behavioral early-warning flags   |
| `GET  /v1/groups/{id}/liquidity`                | Liquidity-stress assessment      |
| `POST /v1/intelligence/optimize-groups`         | Balanced group suggestions       |

**Admin** (role: admin)
| Method & path                          | Purpose                          |
|----------------------------------------|----------------------------------|
| `GET  /v1/admin/overview`              | Platform KPIs                    |
| `GET  /v1/admin/users`                 | List users (paginated)           |
| `POST /v1/admin/users/{id}/suspend`    | Suspend a user                   |
| `POST /v1/admin/users/{id}/activate`   | Reactivate a user                |
| `GET  /v1/admin/kyc/pending`           | KYC review queue                 |
| `POST /v1/admin/kyc/{id}/decision`     | Approve/reject KYC               |
| `GET  /v1/admin/liquidity`             | Liquidity board (all active)     |
| `GET  /v1/admin/audit`                 | Operator audit trail             |

### End-to-end (the ROSCA loop)

```bash
B=localhost:8080
# admin approves KYC; members log in; organizer creates a group;
# others join; organizer activates; everyone contributes each cycle;
# the pool auto-settles to the cycle's payee; missed contributions become
# on-chain defaults via the scheduler. See the smoke tests for the full script.
curl -s -X POST $B/v1/auth/login -d '{"email":"ada@x.com","password":"password123"}'
```

`contribution_amount` is in the smallest currency unit (kobo/cents);
`cycle_length` is a Go duration string (`"168h"` = 1 week).

---

## What is built vs. missing

Legend: ✅ done · 🟡 partial / stubbed · ❌ not started

### Core platform (Go) — now thick
| Area | Status | Notes |
|------|:--:|------|
| Auth: register/login, PBKDF2 + HS256 JWT | ✅ | `internal/auth` |
| Authorization: RBAC middleware, organizer checks | ✅ | member vs admin; per-group ownership |
| User: profile, KYC status, suspend/activate | ✅ | |
| Groups: create, list (paginated), detail | ✅ | |
| Membership: join requests, invitations, leave, remove | ✅ | capacity + lifecycle enforced |
| Activation: fix payout order, materialise cycles | ✅ | |
| Cycles: schedule, current, per-member status | ✅ | |
| Contributions: charge → reveal commitment → escrow | ✅ | three-step, reconciled |
| Payouts + settlement (auto on full pay, or on deadline) | ✅ | escrow debit + on-chain payout |
| Default-detection scheduler | ✅ | `internal/scheduler` heartbeat |
| Escrow ledger (append-only, per group) | ✅ | |
| Notifications (persisted + delivered) | ✅ | log channel in dev |
| Audit trail for admin actions | ✅ | |
| Intelligence: risk, optimizer, **behavioral monitor**, **liquidity stress** | ✅ | all pure Go, explainable |
| Admin API: overview, users, KYC queue, liquidity, audit | ✅ | |
| Pagination on list endpoints | ✅ | `?offset=&limit=` |
| **Real persistent database (SQLite, survives restarts)** | ✅ | `internal/store/sqlitestore`, `database/sql` |
| Unit + live smoke coverage | 🟡 | domain/auth/intelligence/store unit-tested; full lifecycle smoke-tested |

### Still missing (the remaining workstreams)
| Area | Status | Notes |
|------|:--:|------|
| **Solidity contracts** (Identity/Group/Reputation) + go-ethereum `TruthLayer` | ❌ | `MemChain` is a dev fake behind the interface |
| Chain event subscription: log polling, reorg handling, checkpointing | ❌ | indexer currently reads an in-process channel |
| **Real payment adapters** (M-Pesa, MTN MoMo, Paystack, Flutterwave, bank) | ❌ | `MockProvider` behind `payments.Provider` |
| Payment webhooks + idempotency + refunds/reversals | ❌ | |
| **pgx Postgres `Store`** + migration runner | 🟡 | real SQL persistence works today via SQLite (`database/sql`); the Postgres/pgx adapter + schema (`migrations/0001_init.sql`) is the production swap |
| Real notification channels (FCM/APNs/SMS/email) | ❌ | `LogNotifier` behind `notify.Notifier` |
| HTTP hardening: panic recovery, access logs (status+latency), request IDs, liveness+readiness | ✅ | `internal/httpapi/middleware.go`, `/healthz` + `/readyz` |
| Observability: metrics (Prometheus), tracing (OTel) | 🟡 | structured access logs + request IDs; metrics/tracing exporters still TODO |
| Rate limiting, idempotency keys, refresh tokens | ❌ | |
| OpenAPI/Swagger spec | ❌ | |
| Integration test suite | 🟡 | full-lifecycle orchestration test (`internal/app/integration_test.go`) + store/auth/domain/intelligence unit tests |
| Dockerfile, docker-compose, CI/CD, secrets management | ❌ | |
| **Frontend (Next.js)**: member app + admin dashboard UI | ❌ | this repo is backend only; admin **API** is done, admin **UI** is not |
| ML model training/backtesting + off-chain feature pipeline | ❌ | weights are hand-tuned, explainable constants |

> The admin **dashboard API** is fully built (overview, KYC queue, user
> suspension, liquidity board, audit). The admin **UI** that renders it is a
> Next.js workstream, not yet started.

## Configuration (env)

| Var                       | Default                        | Notes                              |
|---------------------------|--------------------------------|------------------------------------|
| `ROTA_HTTP_ADDR`          | `:8080`                        | HTTP listen address                |
| `ROTA_READ_TIMEOUT`       | `10s`                          |                                    |
| `ROTA_WRITE_TIMEOUT`      | `10s`                          |                                    |
| `ROTA_JWT_SECRET`         | `dev-insecure-secret-change-me`| **set in production**              |
| `ROTA_JWT_TTL`            | `24h`                          | token lifetime                     |
| `ROTA_ADMIN_EMAIL`        | `admin@rotasavings.local`      | seeded admin                       |
| `ROTA_ADMIN_PASSWORD`     | `changeme123`                  | **change in production**           |
| `ROTA_SCHEDULER_INTERVAL` | `30s`                          | deadline heartbeat cadence         |
| `ROTA_DB_PATH`            | `rota.db`                       | SQLite cache file; `memory` = ephemeral |
| `ROTA_CHAIN_RPC_URL`      | —                              | reserved for the EVM `TruthLayer`  |
| `ROTA_IDENTITY_REGISTRY`  | —                              | reserved (contract address)        |
| `ROTA_REPUTATION_LEDGER`  | —                              | reserved (contract address)        |
| `ROTA_POSTGRES_DSN`       | —                              | reserved for the pgx `Store`       |

## Suggested next steps

1. **Postgres/pgx `Store`** — port the SQLite `database/sql` store (already real
   and persistent) to Postgres for production scale + concurrency.
2. **Solidity contracts + go-ethereum `TruthLayer`** — replace `MemChain`; add
   real event subscription (reorg-safe, checkpointed).
3. **A real payment adapter** (start with one, e.g. Paystack) + webhooks.
4. **Next.js admin dashboard** against the existing admin API, then the member app.
5. **Observability + Docker/CI** to make it deployable.
