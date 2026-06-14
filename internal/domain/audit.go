package domain

import "time"

// AuditEntry records a privileged action for the operator audit trail. Every
// admin mutation appends one; the trail is append-only and never deleted.
type AuditEntry struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"` // admin user id
	Action    string    `json:"action"`   // e.g. "user.suspend"
	Target    string    `json:"target"`   // affected entity id
	Detail    string    `json:"detail"`   // free-form context
	CreatedAt time.Time `json:"created_at"`
}
