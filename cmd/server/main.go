// Command server is the ROTASAVINGS orchestration backend (the API gateway).
//
// It wires every layer behind its interface:
//
//   - truth layer  : chain.TruthLayer   (in-memory dev impl; EVM later)
//   - read cache   : store.Store        (in-memory dev impl; Postgres later)
//   - payments     : payments.Provider  (mock dev impl; mobile-money/bank later)
//   - notifications: notify.Notifier    (log dev impl; FCM/SMS/email later)
//   - intelligence : pure-Go scoring, optimization, monitoring, liquidity
//
// and runs the event indexer + the deadline scheduler alongside the HTTP server.
// Swapping in real implementations is a one-line change in this file.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"rotasavings/internal/app"
	"rotasavings/internal/auth"
	"rotasavings/internal/chain"
	"rotasavings/internal/chain/evm"
	"rotasavings/internal/config"
	"rotasavings/internal/httpapi"
	"rotasavings/internal/indexer"
	"rotasavings/internal/notify"
	"rotasavings/internal/payments"
	"rotasavings/internal/scheduler"
	"rotasavings/internal/store"
	"rotasavings/internal/store/pgstore"
	"rotasavings/internal/store/sqlitestore"
	"rotasavings/internal/webhooks"
)

// openCache selects the datastore. Precedence: Postgres (if ROTA_POSTGRES_DSN is
// set) > in-memory (ROTA_DB_PATH=memory) > on-disk SQLite (default). All three
// satisfy the same store.Store interface.
func openCache(ctx context.Context, cfg config.Config, log *slog.Logger) (store.Store, func(), error) {
	if cfg.PostgresDSN != "" {
		db, err := pgstore.Open(ctx, cfg.PostgresDSN)
		if err != nil {
			return nil, nil, err
		}
		log.Info("cache: postgres")
		return db, db.Close, nil
	}
	if cfg.DBPath == "memory" {
		log.Info("cache: in-memory (ephemeral)")
		return store.NewMemory(), func() {}, nil
	}
	db, err := sqlitestore.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}
	log.Info("cache: sqlite", "path", cfg.DBPath)
	return db, func() { _ = db.Close() }, nil
}

// openChain selects the truth layer. With ROTA_CHAIN_RPC_URL set it connects to
// an EVM node (production); otherwise it uses the in-process dev chain.
func openChain(ctx context.Context, cfg config.Config, log *slog.Logger) (chain.TruthLayer, error) {
	if cfg.ChainRPCURL == "" {
		log.Info("truth layer: in-memory dev chain")
		return chain.NewMemChain(), nil
	}
	log.Info("truth layer: evm", "rpc", cfg.ChainRPCURL)
	return evm.New(ctx, evm.Config{
		RPCURL:           cfg.ChainRPCURL,
		PrivateKeyHex:    cfg.ChainPrivateKey,
		IdentityRegistry: common.HexToAddress(cfg.IdentityRegistryAddress),
		ReputationLedger: common.HexToAddress(cfg.ReputationLedgerAddress),
	})
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- wire the truth layer: EVM if ROTA_CHAIN_RPC_URL is set, else dev chain ---
	truth, err := openChain(ctx, cfg, log)
	if err != nil {
		return err
	}

	// Cache: Postgres if configured, else on-disk SQLite (memory for ephemeral).
	cache, closeCache, err := openCache(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer closeCache()

	issuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTTTL, cfg.JWTRefreshTTL)
	svc := app.NewService(app.Deps{
		Chain:    truth,
		Store:    cache,
		Payments: payments.NewMockProvider(),
		Notifier: notify.NewLogNotifier(log),
		Issuer:   issuer,
		Webhooks: webhooks.New(cache, log),
	})

	// Bootstrap an admin account.
	if err := svc.SeedAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword, "Platform Admin"); err != nil {
		log.Error("seed admin", "err", err)
	} else {
		log.Info("admin account ready", "email", cfg.AdminEmail)
	}

	// Indexer projects chain events into the cache.
	ix := indexer.New(truth, cache, log)
	go func() {
		if err := ix.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("indexer exited", "err", err)
		}
	}()

	// Scheduler enforces cycle deadlines (records defaults + settles).
	sch := scheduler.New(svc, cfg.SchedulerInterval, log)
	go func() {
		if err := sch.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("scheduler exited", "err", err)
		}
	}()

	// --- HTTP server ---
	opts := httpapi.DefaultOptions()
	opts.CORSOrigins = cfg.CORSOrigins
	api := httpapi.NewServer(svc, auth.NewMiddleware(issuer), log, opts)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      api.Routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
