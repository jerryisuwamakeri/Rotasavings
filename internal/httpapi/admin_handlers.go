package httpapi

import (
	"net/http"
	"strconv"

	"rotasavings/internal/domain"
)

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.svc.Overview(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, total, err := s.svc.ListUsers(r.Context(), pageFrom(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "total": total})
}

func (s *Server) handleAdminSuspend(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.SetUserStatus(r.Context(), caller(r).UserID, r.PathValue("id"), domain.UserSuspended)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleAdminActivate(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.SetUserStatus(r.Context(), caller(r).UserID, r.PathValue("id"), domain.UserActive)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleAdminKYCPending(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.PendingKYC(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending": users})
}

func (s *Server) handleAdminKYCDecision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	u, err := s.svc.DecideKYC(r.Context(), caller(r).UserID, r.PathValue("id"), req.Approve)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleAdminLiquidity(w http.ResponseWriter, r *http.Request) {
	board, err := s.svc.LiquidityBoard(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": board})
}

func (s *Server) handleAdminSetRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	u, err := s.svc.SetUserRole(r.Context(), caller(r).UserID, r.PathValue("id"), domain.Role(req.Role))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.UserDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleAdminGroups(w http.ResponseWriter, r *http.Request) {
	groups, total, err := s.svc.AllGroups(r.Context(), pageFrom(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "total": total})
}

func (s *Server) handleAdminTransactions(w http.ResponseWriter, r *http.Request) {
	txns, total, err := s.svc.AllTransactions(r.Context(), pageFrom(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txns, "total": total})
}

func (s *Server) handleAdminForceSettle(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		s.writeError(w, badBody())
		return
	}
	p, err := s.svc.AdminSettleCycle(r.Context(), caller(r).UserID, r.PathValue("id"), idx)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleAdminListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := s.svc.ListWebhooks(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": hooks})
}

func (s *Server) handleAdminCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	wh, err := s.svc.CreateWebhook(r.Context(), caller(r).UserID, req.URL, req.Secret)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) handleAdminDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteWebhook(r.Context(), caller(r).UserID, r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	entries, total, err := s.svc.AuditLog(r.Context(), pageFrom(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": entries, "total": total})
}
