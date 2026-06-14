package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"rotasavings/internal/app"
	"rotasavings/internal/domain"
	"rotasavings/internal/intelligence"
)

// --- auth ---

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		DisplayName   string `json:"display_name"`
		WalletAddress string `json:"wallet_address"`
		KYCProvider   string `json:"kyc_provider"`
		KYCSignature  string `json:"kyc_signature"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	u, err := s.svc.Register(r.Context(), app.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
		WalletAddress: req.WalletAddress, KYCProvider: req.KYCProvider, KYCSignature: req.KYCSignature,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	pair, u, err := s.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token":         pair.AccessToken, // back-compat alias
		"user":          u,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	pair, err := s.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
	})
}

// --- me / users ---

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.GetUser(r.Context(), caller(r).UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	u, err := s.svc.UpdateProfile(r.Context(), caller(r).UserID, req.DisplayName)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleMyNotifications(w http.ResponseWriter, r *http.Request) {
	ns, err := s.svc.Notifications(r.Context(), caller(r).UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": ns})
}

func (s *Server) handleMyGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.svc.MyGroups(r.Context(), caller(r).UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (s *Server) handleMyTransactions(w http.ResponseWriter, r *http.Request) {
	txns, err := s.svc.Transactions(r.Context(), caller(r).UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txns})
}

func (s *Server) handleReputation(w http.ResponseWriter, r *http.Request) {
	summary, err := s.svc.Reputation(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleRiskScore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IncomeConsistency   float64 `json:"income_consistency"`
		SpendingVolatility  float64 `json:"spending_volatility"`
		PriorParticipations int     `json:"prior_participations"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	score, err := s.svc.ScoreRisk(r.Context(), r.PathValue("id"), intelligence.Features{
		IncomeConsistency:   req.IncomeConsistency,
		SpendingVolatility:  req.SpendingVolatility,
		PriorParticipations: req.PriorParticipations,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, score)
}

// --- groups ---

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name               string          `json:"name"`
		ContributionAmount domain.Money    `json:"contribution_amount"`
		CycleLength        domain.Duration `json:"cycle_length"`
		MaxMembers         int             `json:"max_members"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	g, err := s.svc.CreateGroup(r.Context(), caller(r).UserID, app.CreateGroupInput{
		Name: req.Name, ContributionAmount: req.ContributionAmount,
		CycleLength: req.CycleLength, MaxMembers: req.MaxMembers,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, total, err := s.svc.ListGroups(r.Context(), pageFrom(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "total": total})
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := s.svc.GetGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.svc.ListMembers(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleRequestJoin(w http.ResponseWriter, r *http.Request) {
	jr, err := s.svc.RequestToJoin(r.Context(), caller(r).UserID, r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, jr)
}

func (s *Server) handleListJoinRequests(w http.ResponseWriter, r *http.Request) {
	// Only the organizer should see these; service-level checks live in decide,
	// but listing is harmless metadata for members of the group.
	reqs, err := s.svc.ListJoinRequests(r.Context(), caller(r).UserID, r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"join_requests": reqs})
}

func (s *Server) handleDecideJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	jr, err := s.svc.DecideJoinRequest(r.Context(), caller(r).UserID, r.PathValue("id"), req.Approve)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jr)
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	inv, err := s.svc.Invite(r.Context(), caller(r).UserID, r.PathValue("id"), req.UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) handleMyInvitations(w http.ResponseWriter, r *http.Request) {
	invs, err := s.svc.MyInvitations(r.Context(), caller(r).UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": invs})
}

func (s *Server) handleRespondInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Accept bool `json:"accept"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	inv, err := s.svc.RespondInvitation(r.Context(), caller(r).UserID, r.PathValue("id"), req.Accept)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.LeaveGroup(r.Context(), caller(r).UserID, r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemoveMember(r.Context(), caller(r).UserID, r.PathValue("id"), r.PathValue("userID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PayoutOrder []string `json:"payout_order"`
	}
	// Body is optional; ignore decode error when empty.
	_ = decode(r, &req)
	g, err := s.svc.ActivateGroup(r.Context(), caller(r).UserID, r.PathValue("id"), req.PayoutOrder)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// --- cycles & contributions ---

func (s *Server) handleListCycles(w http.ResponseWriter, r *http.Request) {
	cycles, err := s.svc.ListCycles(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cycles": cycles})
}

func (s *Server) handleCurrentCycle(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.CurrentCycle(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCycleStatus(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		s.writeError(w, badBody())
		return
	}
	status, err := s.svc.CycleStatus(r.Context(), r.PathValue("id"), idx)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleContribute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CycleIndex int    `json:"cycle_index"`
		Source     string `json:"source"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	c, err := s.svc.SubmitContribution(r.Context(), caller(r).UserID, r.PathValue("id"), req.CycleIndex, req.Source)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleSettle(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		s.writeError(w, badBody())
		return
	}
	payout, err := s.svc.SettleCycle(r.Context(), caller(r).UserID, r.PathValue("id"), idx)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payout)
}

// --- intelligence ---

func (s *Server) handleMonitor(w http.ResponseWriter, r *http.Request) {
	flags, err := s.svc.MonitorGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"flags": flags})
}

func (s *Server) handleGroupLiquidity(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.AssessGroupLiquidity(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleOptimize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GroupSize  int                      `json:"group_size"`
		Candidates []intelligence.Candidate `json:"candidates"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, badBody())
		return
	}
	if req.GroupSize < 2 {
		s.writeError(w, fmt.Errorf("%w: group_size must be >= 2", domain.ErrValidation))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"groups": intelligence.OptimizeGroups(req.Candidates, req.GroupSize),
	})
}

func badBody() error {
	return fmt.Errorf("%w: malformed request body", domain.ErrValidation)
}
