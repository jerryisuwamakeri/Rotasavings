// Package evm is the production go-ethereum implementation of chain.TruthLayer.
// It connects to an EVM node (ROTA_CHAIN_RPC_URL) and drives the deployed
// IdentityRegistry / ReputationLedger / RotasavingsGroup contracts
// (contracts/src/*.sol).
//
// Scope and honesty: two operations map cleanly through the existing TruthLayer
// interface and are fully implemented against the chain —
//
//	RegisterIdentity  -> IdentityRegistry.register(bytes32)   (a transaction)
//	Subscribe         -> decode ReputationLedger.ReputationEvent logs
//
// The group-lifecycle writes (DeployGroup / ActivateGroup / SubmitContribution /
// RecordDefault / RecordPayout) need data the current interface does not carry
// on-chain-addressably (compiled group bytecode for deploys; the per-group
// contract address and the keccak commitment scheme for writes — the dev chain
// uses sha256 over string IDs). Those are returned as ErrNotWired with a precise
// explanation rather than pretending to work. Reconciling them is the documented
// next step (see contracts/README.md and the package TODO).
package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"rotasavings/internal/chain"
	"rotasavings/internal/domain"
)

// ErrNotWired is returned by operations that require interface/commitment
// reconciliation before they can be driven on a real chain (see package doc).
var ErrNotWired = errors.New("evm: operation requires interface/commitment reconciliation (see package doc)")

// Config configures the EVM truth layer.
type Config struct {
	RPCURL           string
	PrivateKeyHex    string // optional; required only for write operations
	IdentityRegistry common.Address
	ReputationLedger common.Address
	PollInterval     time.Duration // event poll cadence (default 12s)
	StartBlock       uint64        // first block to index from
}

// Chain is a go-ethereum-backed chain.TruthLayer.
type Chain struct {
	cfg       Config
	client    *ethclient.Client
	identity  *bind.BoundContract
	ledgerABI abi.ABI

	mu   sync.Mutex // serialises nonce use on the shared transactor
	auth *bind.TransactOpts
}

// Chain satisfies the chain.TruthLayer interface.
var _ chain.TruthLayer = (*Chain)(nil)

// New dials the node and prepares bound contracts. A private key is optional;
// without it, write operations fail but Subscribe still works.
func New(ctx context.Context, cfg Config) (*Chain, error) {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 12 * time.Second
	}
	client, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.RPCURL, err)
	}

	idABI, err := abi.JSON(strings.NewReader(identityRegistryABI))
	if err != nil {
		return nil, err
	}
	ledgerABI, err := abi.JSON(strings.NewReader(reputationLedgerABI))
	if err != nil {
		return nil, err
	}

	c := &Chain{
		cfg:       cfg,
		client:    client,
		ledgerABI: ledgerABI,
		identity:  bind.NewBoundContract(cfg.IdentityRegistry, idABI, client, client, client),
	}

	if cfg.PrivateKeyHex != "" {
		chainID, err := client.ChainID(ctx)
		if err != nil {
			return nil, fmt.Errorf("chain id: %w", err)
		}
		key, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKeyHex, "0x"))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
		if err != nil {
			return nil, err
		}
		c.auth = auth
	}
	return c, nil
}

// RegisterIdentity anchors a commitment in the IdentityRegistry. The off-chain
// commitment is a 32-byte digest; it maps directly to the contract's bytes32.
func (c *Chain) RegisterIdentity(ctx context.Context, commitment string) (string, error) {
	if c.auth == nil {
		return "", errors.New("evm: no signer configured (set ROTA_CHAIN_PRIVATE_KEY)")
	}
	var h [32]byte
	copy(h[:], common.FromHex(commitment))

	c.mu.Lock()
	defer c.mu.Unlock()
	opts := *c.auth
	opts.Context = ctx
	tx, err := c.identity.Transact(&opts, "register", h)
	if err != nil {
		return "", fmt.Errorf("register identity: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// DeployGroup requires the compiled RotasavingsGroup bytecode (run `forge build`)
// and a deploy transactor; not wired through the current interface.
func (c *Chain) DeployGroup(_ context.Context, _ domain.Group) (string, error) {
	return "", fmt.Errorf("%w: DeployGroup needs compiled group bytecode", ErrNotWired)
}

// ActivateGroup needs the on-chain payout-order addresses, which the interface
// does not carry; not wired.
func (c *Chain) ActivateGroup(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("%w: ActivateGroup needs payout-order addresses", ErrNotWired)
}

// SubmitContribution needs the group contract address and the keccak commitment
// scheme (the dev chain uses sha256 over string IDs); not wired.
func (c *Chain) SubmitContribution(_ context.Context, _ domain.Contribution) (string, error) {
	return "", fmt.Errorf("%w: SubmitContribution needs group address + keccak commitment", ErrNotWired)
}

// RecordDefault is emitted on-chain by RotasavingsGroup.settle(); the backend
// does not push it directly in the EVM model.
func (c *Chain) RecordDefault(_ context.Context, _ chain.DefaultEvent) (string, error) {
	return "", fmt.Errorf("%w: defaults are emitted by RotasavingsGroup.settle()", ErrNotWired)
}

// RecordPayout is emitted on-chain by RotasavingsGroup.settle(); not pushed.
func (c *Chain) RecordPayout(_ context.Context, _ chain.PayoutEvent) (string, error) {
	return "", fmt.Errorf("%w: payouts are emitted by RotasavingsGroup.settle()", ErrNotWired)
}

// repEventData is the non-indexed payload of ReputationLedger.ReputationEvent.
type repEventData struct {
	CycleIndex *big.Int
	EventType  uint8
	Amount     *big.Int
	Timestamp  uint64
}

// Subscribe polls the ReputationLedger for ReputationEvent logs and projects
// them into domain.ReputationEvent. User and group are addresses in the EVM
// model, so they populate the string ID fields as hex addresses.
func (c *Chain) Subscribe(ctx context.Context) (<-chan domain.ReputationEvent, error) {
	out := make(chan domain.ReputationEvent, 64)
	topic := c.ledgerABI.Events["ReputationEvent"].ID

	go func() {
		defer close(out)
		from := c.cfg.StartBlock
		ticker := time.NewTicker(c.cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			head, err := c.client.BlockNumber(ctx)
			if err != nil || head < from {
				continue
			}
			logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(from),
				ToBlock:   new(big.Int).SetUint64(head),
				Addresses: []common.Address{c.cfg.ReputationLedger},
				Topics:    [][]common.Hash{{topic}},
			})
			if err != nil {
				continue
			}
			for _, lg := range logs {
				ev, ok := c.decodeReputation(lg)
				if !ok {
					continue
				}
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
			from = head + 1
		}
	}()
	return out, nil
}

func (c *Chain) decodeReputation(lg types.Log) (domain.ReputationEvent, bool) {
	if len(lg.Topics) < 3 {
		return domain.ReputationEvent{}, false
	}
	var d repEventData
	if err := c.ledgerABI.UnpackIntoInterface(&d, "ReputationEvent", lg.Data); err != nil {
		return domain.ReputationEvent{}, false
	}
	if int(d.EventType) >= len(reputationEventOrdinal) {
		return domain.ReputationEvent{}, false
	}
	user := common.BytesToAddress(lg.Topics[1].Bytes())
	group := common.BytesToAddress(lg.Topics[2].Bytes())
	return domain.ReputationEvent{
		ID:         fmt.Sprintf("%s:%d", lg.TxHash.Hex(), lg.Index),
		UserID:     user.Hex(),
		GroupID:    group.Hex(),
		CycleIndex: int(d.CycleIndex.Int64()),
		Type:       domain.EventType(reputationEventOrdinal[d.EventType]),
		Amount:     domain.Money(d.Amount.Int64()),
		ProofHash:  lg.TxHash.Hex(),
		Timestamp:  time.Unix(int64(d.Timestamp), 0).UTC(),
	}, true
}
