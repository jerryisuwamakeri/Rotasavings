package domain

import "time"

// Webhook is an operator-registered HTTP endpoint that receives platform events
// (contributions, payouts, defaults, KYC decisions, ...). Delivery is
// best-effort and fire-and-forget.
type Webhook struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"` // optional HMAC secret
	Active    bool      `json:"active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookEvent is the payload POSTed to a registered webhook URL.
type WebhookEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	UserID    string         `json:"user_id,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}
