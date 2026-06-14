// Package indexer projects events from the truth layer into the read cache.
//
// This is the one-way data flow that keeps the system honest: the chain emits
// events, the indexer folds them into Postgres. If the cache is ever lost it is
// rebuilt by replaying from block height — the cache is disposable, the chain
// is not.
package indexer

import (
	"context"
	"log/slog"

	"rotasavings/internal/chain"
	"rotasavings/internal/store"
)

// Indexer subscribes to the truth layer and writes reputation projections.
type Indexer struct {
	chain chain.TruthLayer
	store store.Store
	log   *slog.Logger
}

func New(c chain.TruthLayer, s store.Store, log *slog.Logger) *Indexer {
	return &Indexer{chain: c, store: s, log: log}
}

// Run blocks, consuming chain events until ctx is cancelled. Intended to be
// launched in its own goroutine.
func (ix *Indexer) Run(ctx context.Context) error {
	events, err := ix.chain.Subscribe(ctx)
	if err != nil {
		return err
	}
	ix.log.Info("indexer started")
	for {
		select {
		case <-ctx.Done():
			ix.log.Info("indexer stopped")
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			e := ev
			if err := ix.store.AppendReputationEvent(ctx, &e); err != nil {
				// A real indexer tracks the last processed block so it can retry
				// without double-counting; here we just log.
				ix.log.Error("project reputation event", "err", err, "type", e.Type, "user", e.UserID)
				continue
			}
			ix.log.Debug("indexed event", "type", e.Type, "user", e.UserID, "group", e.GroupID)
		}
	}
}
