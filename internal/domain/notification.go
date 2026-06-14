package domain

import "time"

// NotificationKind classifies an outbound user notification.
type NotificationKind string

const (
	NotifyContributionDue  NotificationKind = "contribution_due"
	NotifyContributionMade NotificationKind = "contribution_made"
	NotifyPayoutSent       NotificationKind = "payout_sent"
	NotifyDefaultRecorded  NotificationKind = "default_recorded"
	NotifyJoinRequest      NotificationKind = "join_request"
	NotifyJoinDecision     NotificationKind = "join_decision"
	NotifyInvitation       NotificationKind = "invitation"
	NotifyGroupActivated   NotificationKind = "group_activated"
	NotifyRiskWarning      NotificationKind = "risk_warning"
)

// Notification is a message destined for a user across some channel (push/SMS/
// email). Stored so it can be listed in-app; delivery is the Notifier's job.
type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Kind      NotificationKind `json:"kind"`
	Title     string           `json:"title"`
	Body      string           `json:"body"`
	Read      bool             `json:"read"`
	CreatedAt time.Time        `json:"created_at"`
}
