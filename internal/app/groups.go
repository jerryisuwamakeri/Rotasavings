package app

import (
	"context"
	"fmt"
	"time"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

// CreateGroupInput is the payload to create a group.
type CreateGroupInput struct {
	Name               string
	ContributionAmount domain.Money
	CycleLength        domain.Duration
	MaxMembers         int
}

// CreateGroup deploys a group contract with the creator as organizer + first
// member. Members join over time; the payout order is fixed at activation.
func (s *Service) CreateGroup(ctx context.Context, creatorID string, in CreateGroupInput) (*domain.Group, error) {
	creator, err := s.store.GetUser(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	if err := creator.CanParticipate(); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	g := &domain.Group{
		ID:                 domain.NewID(),
		Name:               in.Name,
		OrganizerID:        creatorID,
		ContributionAmount: in.ContributionAmount,
		CycleLength:        in.CycleLength,
		MaxMembers:         in.MaxMembers,
		Members:            []string{creatorID},
		State:              domain.GroupCreated,
		CreatedAt:          now,
	}
	if err := g.ValidateConfig(); err != nil {
		return nil, err
	}

	addr, err := s.chain.DeployGroup(ctx, *g)
	if err != nil {
		return nil, fmt.Errorf("deploy group contract: %w", err)
	}
	g.ContractAddress = addr
	if err := s.store.SaveGroup(ctx, g); err != nil {
		return nil, fmt.Errorf("cache group: %w", err)
	}
	if err := s.store.SaveMembership(ctx, &domain.Membership{
		ID: domain.NewID(), GroupID: g.ID, UserID: creatorID,
		Organizer: true, Status: domain.MembershipActive, JoinedAt: now,
	}); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Service) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	return s.store.GetGroup(ctx, id)
}

// MyGroupMembership pairs a group with the caller's role/status in it.
type MyGroupMembership struct {
	Group     *domain.Group           `json:"group"`
	Organizer bool                    `json:"organizer"`
	Status    domain.MembershipStatus `json:"status"`
}

// MyGroups returns every group the user belongs to, with their membership role.
func (s *Service) MyGroups(ctx context.Context, userID string) ([]MyGroupMembership, error) {
	mems, err := s.store.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]MyGroupMembership, 0, len(mems))
	for _, m := range mems {
		g, err := s.store.GetGroup(ctx, m.GroupID)
		if err != nil {
			continue
		}
		out = append(out, MyGroupMembership{Group: g, Organizer: m.Organizer, Status: m.Status})
	}
	return out, nil
}

func (s *Service) ListGroups(ctx context.Context, p store.Page) ([]*domain.Group, int, error) {
	return s.store.ListGroups(ctx, p)
}

func (s *Service) ListMembers(ctx context.Context, groupID string) ([]*domain.Membership, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.store.ListMembershipsByGroup(ctx, groupID)
}

// ListJoinRequests returns a group's join requests (organizer only).
func (s *Service) ListJoinRequests(ctx context.Context, actorID, groupID string) ([]*domain.JoinRequest, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.OrganizerID != actorID {
		return nil, fmt.Errorf("%w: only the organizer can view join requests", domain.ErrForbidden)
	}
	return s.store.ListJoinRequestsByGroup(ctx, groupID)
}

// MyInvitations returns the invitations addressed to a user.
func (s *Service) MyInvitations(ctx context.Context, userID string) ([]*domain.Invitation, error) {
	return s.store.ListInvitationsByUser(ctx, userID)
}

// RequestToJoin records a pending join request for a CREATED group.
func (s *Service) RequestToJoin(ctx context.Context, userID, groupID string) (*domain.JoinRequest, error) {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := user.CanParticipate(); err != nil {
		return nil, err
	}
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.State != domain.GroupCreated {
		return nil, fmt.Errorf("%w: group is %s, can only join while CREATED", domain.ErrConflict, g.State)
	}
	if g.HasMember(userID) {
		return nil, fmt.Errorf("%w: already a member", domain.ErrConflict)
	}
	if len(g.Members) >= g.MaxMembers {
		return nil, fmt.Errorf("%w: group is full", domain.ErrConflict)
	}
	now := s.now().UTC()
	jr := &domain.JoinRequest{
		ID: domain.NewID(), GroupID: groupID, UserID: userID,
		Status: domain.JoinPending, CreatedAt: now,
	}
	if err := s.store.SaveJoinRequest(ctx, jr); err != nil {
		return nil, err
	}
	s.notifyUser(ctx, g.OrganizerID, domain.NotifyJoinRequest,
		"New join request", fmt.Sprintf("%s wants to join %s", user.DisplayName, g.Name))
	return jr, nil
}

// DecideJoinRequest lets the organizer approve or reject a request.
func (s *Service) DecideJoinRequest(ctx context.Context, actorID, requestID string, approve bool) (*domain.JoinRequest, error) {
	jr, err := s.store.GetJoinRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	g, err := s.store.GetGroup(ctx, jr.GroupID)
	if err != nil {
		return nil, err
	}
	if g.OrganizerID != actorID {
		return nil, fmt.Errorf("%w: only the organizer can decide join requests", domain.ErrForbidden)
	}
	if jr.Status != domain.JoinPending {
		return nil, fmt.Errorf("%w: request already %s", domain.ErrConflict, jr.Status)
	}
	now := s.now().UTC()
	jr.DecidedAt = &now
	jr.DecidedBy = actorID

	if approve {
		if g.State != domain.GroupCreated {
			return nil, fmt.Errorf("%w: group no longer accepting members", domain.ErrConflict)
		}
		if len(g.Members) >= g.MaxMembers {
			return nil, fmt.Errorf("%w: group is full", domain.ErrConflict)
		}
		jr.Status = domain.JoinApproved
		if err := s.addMember(ctx, g, jr.UserID, now); err != nil {
			return nil, err
		}
	} else {
		jr.Status = domain.JoinRejected
	}
	if err := s.store.SaveJoinRequest(ctx, jr); err != nil {
		return nil, err
	}
	s.notifyUser(ctx, jr.UserID, domain.NotifyJoinDecision,
		"Join request "+string(jr.Status), fmt.Sprintf("Your request to join %s was %s", g.Name, jr.Status))
	return jr, nil
}

// Invite lets the organizer invite a user directly.
func (s *Service) Invite(ctx context.Context, actorID, groupID, inviteeID string) (*domain.Invitation, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.OrganizerID != actorID {
		return nil, fmt.Errorf("%w: only the organizer can invite", domain.ErrForbidden)
	}
	if g.State != domain.GroupCreated {
		return nil, fmt.Errorf("%w: group is %s", domain.ErrConflict, g.State)
	}
	if _, err := s.store.GetUser(ctx, inviteeID); err != nil {
		return nil, err
	}
	if g.HasMember(inviteeID) {
		return nil, fmt.Errorf("%w: already a member", domain.ErrConflict)
	}
	now := s.now().UTC()
	inv := &domain.Invitation{
		ID: domain.NewID(), GroupID: groupID, UserID: inviteeID, InvitedBy: actorID,
		Status: domain.InvitePending, CreatedAt: now,
	}
	if err := s.store.SaveInvitation(ctx, inv); err != nil {
		return nil, err
	}
	s.notifyUser(ctx, inviteeID, domain.NotifyInvitation,
		"Group invitation", fmt.Sprintf("You were invited to join %s", g.Name))
	return inv, nil
}

// RespondInvitation lets an invitee accept or decline.
func (s *Service) RespondInvitation(ctx context.Context, userID, inviteID string, accept bool) (*domain.Invitation, error) {
	inv, err := s.store.GetInvitation(ctx, inviteID)
	if err != nil {
		return nil, err
	}
	if inv.UserID != userID {
		return nil, fmt.Errorf("%w: not your invitation", domain.ErrForbidden)
	}
	if inv.Status != domain.InvitePending {
		return nil, fmt.Errorf("%w: invitation already %s", domain.ErrConflict, inv.Status)
	}
	g, err := s.store.GetGroup(ctx, inv.GroupID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if accept {
		user, err := s.store.GetUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if err := user.CanParticipate(); err != nil {
			return nil, err
		}
		if g.State != domain.GroupCreated {
			return nil, fmt.Errorf("%w: group no longer accepting members", domain.ErrConflict)
		}
		if len(g.Members) >= g.MaxMembers {
			return nil, fmt.Errorf("%w: group is full", domain.ErrConflict)
		}
		inv.Status = domain.InviteAccepted
		if err := s.addMember(ctx, g, userID, now); err != nil {
			return nil, err
		}
	} else {
		inv.Status = domain.InviteDeclined
	}
	if err := s.store.SaveInvitation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// LeaveGroup removes a member from a CREATED group (cannot leave once ACTIVE).
func (s *Service) LeaveGroup(ctx context.Context, userID, groupID string) error {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return err
	}
	if g.State != domain.GroupCreated {
		return fmt.Errorf("%w: cannot leave a %s group mid-rotation", domain.ErrConflict, g.State)
	}
	if g.OrganizerID == userID {
		return fmt.Errorf("%w: the organizer cannot leave their own group", domain.ErrForbidden)
	}
	return s.dropMember(ctx, g, userID, domain.MembershipLeft)
}

// RemoveMember lets the organizer remove a member from a CREATED group.
func (s *Service) RemoveMember(ctx context.Context, actorID, groupID, userID string) error {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return err
	}
	if g.OrganizerID != actorID {
		return fmt.Errorf("%w: only the organizer can remove members", domain.ErrForbidden)
	}
	if g.State != domain.GroupCreated {
		return fmt.Errorf("%w: cannot remove from a %s group", domain.ErrConflict, g.State)
	}
	if userID == g.OrganizerID {
		return fmt.Errorf("%w: cannot remove the organizer", domain.ErrForbidden)
	}
	return s.dropMember(ctx, g, userID, domain.MembershipRemoved)
}

// addMember adds userID to the group's member list and creates a membership.
func (s *Service) addMember(ctx context.Context, g *domain.Group, userID string, now time.Time) error {
	g.Members = append(g.Members, userID)
	if err := s.store.SaveGroup(ctx, g); err != nil {
		return err
	}
	return s.store.SaveMembership(ctx, &domain.Membership{
		ID: domain.NewID(), GroupID: g.ID, UserID: userID,
		Organizer: false, Status: domain.MembershipActive, JoinedAt: now,
	})
}

// dropMember removes userID from the group and marks the membership.
func (s *Service) dropMember(ctx context.Context, g *domain.Group, userID string, status domain.MembershipStatus) error {
	mem, err := s.store.GetMembership(ctx, g.ID, userID)
	if err != nil {
		return err
	}
	out := g.Members[:0]
	for _, m := range g.Members {
		if m != userID {
			out = append(out, m)
		}
	}
	g.Members = out
	if err := s.store.SaveGroup(ctx, g); err != nil {
		return err
	}
	mem.Status = status
	return s.store.SaveMembership(ctx, mem)
}
