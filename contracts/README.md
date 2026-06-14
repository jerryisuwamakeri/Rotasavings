# ROTASAVINGS Smart Contracts

The truth layer. Three contracts mirror the off-chain `chain.TruthLayer`
interface:

| Contract              | Role                                                            |
|-----------------------|----------------------------------------------------------------|
| `IdentityRegistry`    | Anchors KYC identity commitments (hash only, never PII).       |
| `ReputationLedger`    | Append-only, event-based reputation; authorised writers only.  |
| `RotasavingsGroup`    | One ROSCA group: rules, lifecycle, contributions, settlement.  |

## Design notes

- **No custody here.** Real money moves off-chain (escrow + mobile money / bank
  rails). These contracts record *truth and reputation*, not fund transfers, so
  the system works with off-chain payment instruments common in ajo/esusu.
- **Commitments.** A contribution is bound to
  `keccak256(user, group, cycle, amount)` and revealed via `contribute`. A
  default is the deterministic failure to reveal before `cycleDeadline(cycle)`.
- **Lifecycle.** `CREATED -> ACTIVE -> SETTLEMENT -> CLOSED`, enforced on-chain.
- **Reputation.** `RotasavingsGroup` is authorised on the `ReputationLedger` and
  emits `ContributionMade` / `ContributionMissed` / `PayoutReceived`. The Go
  indexer subscribes to these events and projects them into the read cache.

## Build & test (Foundry)

```bash
cd contracts
forge build --extra-output-files abi bin
forge test
```

## Deploy (example, Anvil/local)

```bash
anvil &
forge create src/ReputationLedger.sol:ReputationLedger --rpc-url http://localhost:8545 --private-key $PK
# then deploy IdentityRegistry, and a RotasavingsGroup per group
```

The Go backend's EVM truth layer (`internal/chain/evm`) connects via
`ROTA_CHAIN_RPC_URL` + the deployed contract addresses and drives these methods
behind the same `chain.TruthLayer` interface used by the in-memory dev chain.
