package domain

import "time"

// MembershipStatus tracks a user's relationship to a group.
type MembershipStatus string

const (
	MembershipActive  MembershipStatus = "active"
	MembershipLeft    MembershipStatus = "left"
	MembershipRemoved MembershipStatus = "removed"
)

// Membership is the join between a user and a group. The Organizer flag marks
// the member who created the group and may manage join requests.
type Membership struct {
	ID        string           `json:"id"`
	GroupID   string           `json:"group_id"`
	UserID    string           `json:"user_id"`
	Organizer bool             `json:"organizer"`
	Status    MembershipStatus `json:"status"`
	JoinedAt  time.Time        `json:"joined_at"`
}

// JoinRequestStatus is the lifecycle of a request to join a group.
type JoinRequestStatus string

const (
	JoinPending  JoinRequestStatus = "pending"
	JoinApproved JoinRequestStatus = "approved"
	JoinRejected JoinRequestStatus = "rejected"
)

// JoinRequest is a user's application to join a group, decided by the organizer.
type JoinRequest struct {
	ID        string            `json:"id"`
	GroupID   string            `json:"group_id"`
	UserID    string            `json:"user_id"`
	Status    JoinRequestStatus `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	DecidedAt *time.Time        `json:"decided_at,omitempty"`
	DecidedBy string            `json:"decided_by,omitempty"`
}

// InvitationStatus is the lifecycle of an organizer's invitation.
type InvitationStatus string

const (
	InvitePending  InvitationStatus = "pending"
	InviteAccepted InvitationStatus = "accepted"
	InviteDeclined InvitationStatus = "declined"
)

// Invitation is an organizer inviting a user into a group.
type Invitation struct {
	ID        string           `json:"id"`
	GroupID   string           `json:"group_id"`
	UserID    string           `json:"user_id"`
	InvitedBy string           `json:"invited_by"`
	Status    InvitationStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
}
