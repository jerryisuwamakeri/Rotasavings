// Package httpapi exposes the orchestration layer over HTTP (the API gateway).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"rotasavings/internal/app"
	"rotasavings/internal/auth"
	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

// Server holds the dependencies the handlers need.
type Server struct {
	svc      *app.Service
	mw       *auth.Middleware
	log      *slog.Logger
	metrics  *Metrics
	limiter  *RateLimiter
	idem     *Idempotency
	corsOrig string
}

// Options configures cross-cutting middleware.
type Options struct {
	RatePerSec     float64       // per-client request rate
	RateBurst      float64       // per-client burst
	IdempotencyTTL time.Duration // how long to remember Idempotency-Key results
	CORSOrigins    string        // comma-separated allowlist, or "*" for any
}

// DefaultOptions returns sensible middleware defaults.
func DefaultOptions() Options {
	return Options{RatePerSec: 20, RateBurst: 40, IdempotencyTTL: 24 * time.Hour, CORSOrigins: "*"}
}

func NewServer(svc *app.Service, mw *auth.Middleware, log *slog.Logger, opts Options) *Server {
	return &Server{
		svc:      svc,
		mw:       mw,
		log:      log,
		metrics:  NewMetrics(),
		limiter:  NewRateLimiter(opts.RatePerSec, opts.RateBurst),
		idem:     NewIdempotency(opts.IdempotencyTTL),
		corsOrig: opts.CORSOrigins,
	}
}

// Routes builds the API gateway mux using Go 1.22+ method-aware patterns.
// Authentication is applied globally (parses the bearer token if present);
// individual handlers enforce RequireAuth / RequireAdmin.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public.
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)
	mux.HandleFunc("GET /metrics", s.metrics.Handler())
	mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPI)
	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)

	// Authenticated member endpoints.
	mux.HandleFunc("GET /v1/me", auth.RequireAuth(s.handleMe))
	mux.HandleFunc("PATCH /v1/me", auth.RequireAuth(s.handleUpdateMe))
	mux.HandleFunc("GET /v1/me/notifications", auth.RequireAuth(s.handleMyNotifications))
	mux.HandleFunc("GET /v1/me/invitations", auth.RequireAuth(s.handleMyInvitations))
	mux.HandleFunc("GET /v1/me/groups", auth.RequireAuth(s.handleMyGroups))
	mux.HandleFunc("GET /v1/me/transactions", auth.RequireAuth(s.handleMyTransactions))
	mux.HandleFunc("GET /v1/users/{id}/reputation", auth.RequireAuth(s.handleReputation))
	mux.HandleFunc("POST /v1/users/{id}/risk-score", auth.RequireAuth(s.handleRiskScore))

	mux.HandleFunc("POST /v1/groups", auth.RequireAuth(s.handleCreateGroup))
	mux.HandleFunc("GET /v1/groups", auth.RequireAuth(s.handleListGroups))
	mux.HandleFunc("GET /v1/groups/{id}", auth.RequireAuth(s.handleGetGroup))
	mux.HandleFunc("GET /v1/groups/{id}/members", auth.RequireAuth(s.handleListMembers))
	mux.HandleFunc("POST /v1/groups/{id}/join-requests", auth.RequireAuth(s.handleRequestJoin))
	mux.HandleFunc("GET /v1/groups/{id}/join-requests", auth.RequireAuth(s.handleListJoinRequests))
	mux.HandleFunc("POST /v1/join-requests/{id}/decision", auth.RequireAuth(s.handleDecideJoin))
	mux.HandleFunc("POST /v1/groups/{id}/invitations", auth.RequireAuth(s.handleInvite))
	mux.HandleFunc("POST /v1/invitations/{id}/response", auth.RequireAuth(s.handleRespondInvite))
	mux.HandleFunc("POST /v1/groups/{id}/leave", auth.RequireAuth(s.handleLeave))
	mux.HandleFunc("DELETE /v1/groups/{id}/members/{userID}", auth.RequireAuth(s.handleRemoveMember))
	mux.HandleFunc("POST /v1/groups/{id}/activate", auth.RequireAuth(s.handleActivate))

	mux.HandleFunc("GET /v1/groups/{id}/cycles", auth.RequireAuth(s.handleListCycles))
	mux.HandleFunc("GET /v1/groups/{id}/cycles/current", auth.RequireAuth(s.handleCurrentCycle))
	mux.HandleFunc("GET /v1/groups/{id}/cycles/{index}/status", auth.RequireAuth(s.handleCycleStatus))
	mux.HandleFunc("POST /v1/groups/{id}/contributions", auth.RequireAuth(s.handleContribute))
	mux.HandleFunc("POST /v1/groups/{id}/cycles/{index}/settle", auth.RequireAuth(s.handleSettle))

	mux.HandleFunc("GET /v1/groups/{id}/monitor", auth.RequireAuth(s.handleMonitor))
	mux.HandleFunc("GET /v1/groups/{id}/liquidity", auth.RequireAuth(s.handleGroupLiquidity))
	mux.HandleFunc("POST /v1/intelligence/optimize-groups", auth.RequireAuth(s.handleOptimize))

	// Admin endpoints.
	mux.HandleFunc("GET /v1/admin/overview", auth.RequireAdmin(s.handleAdminOverview))
	mux.HandleFunc("GET /v1/admin/users", auth.RequireAdmin(s.handleAdminUsers))
	mux.HandleFunc("POST /v1/admin/users/{id}/suspend", auth.RequireAdmin(s.handleAdminSuspend))
	mux.HandleFunc("POST /v1/admin/users/{id}/activate", auth.RequireAdmin(s.handleAdminActivate))
	mux.HandleFunc("GET /v1/admin/kyc/pending", auth.RequireAdmin(s.handleAdminKYCPending))
	mux.HandleFunc("POST /v1/admin/kyc/{id}/decision", auth.RequireAdmin(s.handleAdminKYCDecision))
	mux.HandleFunc("GET /v1/admin/users/{id}", auth.RequireAdmin(s.handleAdminUserDetail))
	mux.HandleFunc("POST /v1/admin/users/{id}/role", auth.RequireAdmin(s.handleAdminSetRole))
	mux.HandleFunc("GET /v1/admin/groups", auth.RequireAdmin(s.handleAdminGroups))
	mux.HandleFunc("POST /v1/admin/groups/{id}/cycles/{index}/settle", auth.RequireAdmin(s.handleAdminForceSettle))
	mux.HandleFunc("GET /v1/admin/transactions", auth.RequireAdmin(s.handleAdminTransactions))
	mux.HandleFunc("GET /v1/admin/webhooks", auth.RequireAdmin(s.handleAdminListWebhooks))
	mux.HandleFunc("POST /v1/admin/webhooks", auth.RequireAdmin(s.handleAdminCreateWebhook))
	mux.HandleFunc("DELETE /v1/admin/webhooks/{id}", auth.RequireAdmin(s.handleAdminDeleteWebhook))
	mux.HandleFunc("GET /v1/admin/liquidity", auth.RequireAdmin(s.handleAdminLiquidity))
	mux.HandleFunc("GET /v1/admin/audit", auth.RequireAdmin(s.handleAdminAudit))

	// Middleware chain (outermost first): cors -> recover -> request id ->
	// rate limit -> access log (+ metrics) -> idempotency -> authenticate -> routes.
	return s.cors(s.corsOrig)(
		s.recoverPanic(
			s.requestID(
				s.limiter.Middleware(
					s.accessLog(
						s.idem.Middleware(
							s.mw.Authenticate(mux)))))))
}

// handleHealth is a liveness probe: the process is up.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady is a readiness probe: the process is up AND its datastore answers.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Health(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorBody struct {
	Error string `json:"error"`
}

// writeError maps domain errors to HTTP status codes.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{err.Error()})
	case errors.Is(err, domain.ErrValidation):
		writeJSON(w, http.StatusBadRequest, errorBody{err.Error()})
	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, errorBody{err.Error()})
	case errors.Is(err, domain.ErrInvalidTransition):
		writeJSON(w, http.StatusConflict, errorBody{err.Error()})
	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, errorBody{err.Error()})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, errorBody{err.Error()})
	default:
		s.log.Error("internal error", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{"internal error"})
	}
}

func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.ErrValidation
	}
	return nil
}

// caller returns the authenticated principal (RequireAuth guarantees presence).
func caller(r *http.Request) auth.Principal {
	p, _ := auth.PrincipalFrom(r.Context())
	return p
}

// pageFrom parses ?offset=&limit= into a store.Page.
func pageFrom(r *http.Request) store.Page {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return store.Page{Offset: offset, Limit: limit}
}
