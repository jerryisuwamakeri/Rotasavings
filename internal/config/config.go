// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds process configuration. The chain RPC / contract addresses and
// Postgres DSN live here for when the real implementations replace the in-memory
// dev defaults.
type Config struct {
	HTTPAddr     string // e.g. ":8080"
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Auth.
	JWTSecret     string
	JWTTTL        time.Duration // access token lifetime
	JWTRefreshTTL time.Duration // refresh token lifetime

	// Bootstrap admin (created at boot if absent).
	AdminEmail    string
	AdminPassword string

	// Scheduler.
	SchedulerInterval time.Duration

	// CORS: comma-separated allowed origins, or "*" for any (dev default).
	CORSOrigins string

	// Truth layer. When ChainRPCURL is set, the EVM truth layer is used instead
	// of the in-memory dev chain.
	ChainRPCURL             string
	ChainPrivateKey         string
	IdentityRegistryAddress string
	ReputationLedgerAddress string

	// Cache. DBPath selects the on-disk SQLite cache; "memory" uses the
	// ephemeral in-memory store. PostgresDSN is reserved for the pgx store.
	DBPath      string
	PostgresDSN string
}

// Load reads configuration from the environment, applying sane defaults.
func Load() Config {
	return Config{
		HTTPAddr:                getEnv("ROTA_HTTP_ADDR", ":8080"),
		ReadTimeout:             getDuration("ROTA_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:            getDuration("ROTA_WRITE_TIMEOUT", 10*time.Second),
		JWTSecret:               getEnv("ROTA_JWT_SECRET", "dev-insecure-secret-change-me"),
		JWTTTL:                  getDuration("ROTA_JWT_TTL", 1*time.Hour),
		JWTRefreshTTL:           getDuration("ROTA_JWT_REFRESH_TTL", 720*time.Hour),
		AdminEmail:              getEnv("ROTA_ADMIN_EMAIL", "admin@rotasavings.local"),
		AdminPassword:           getEnv("ROTA_ADMIN_PASSWORD", "changeme123"),
		SchedulerInterval:       getDuration("ROTA_SCHEDULER_INTERVAL", 30*time.Second),
		CORSOrigins:             getEnv("ROTA_CORS_ORIGINS", "*"),
		DBPath:                  getEnv("ROTA_DB_PATH", "rota.db"),
		ChainRPCURL:             getEnv("ROTA_CHAIN_RPC_URL", ""),
		ChainPrivateKey:         getEnv("ROTA_CHAIN_PRIVATE_KEY", ""),
		IdentityRegistryAddress: getEnv("ROTA_IDENTITY_REGISTRY", ""),
		ReputationLedgerAddress: getEnv("ROTA_REPUTATION_LEDGER", ""),
		PostgresDSN:             getEnv("ROTA_POSTGRES_DSN", ""),
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return def
}
