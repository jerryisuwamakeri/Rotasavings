// Package notify delivers user-facing notifications (push / SMS / email). The
// Notifier interface lets the orchestration layer fire-and-forget messages; the
// concrete delivery channel is swappable. A log-backed implementation ships for
// development.
package notify

import (
	"context"
	"log/slog"

	"rotasavings/internal/domain"
)

// Notifier delivers a notification across whatever channel it wraps.
type Notifier interface {
	Notify(ctx context.Context, n domain.Notification) error
}

// LogNotifier writes notifications to the structured log. Useful in dev; in
// production you would back this with FCM/APNs, an SMS gateway, or email.
type LogNotifier struct{ log *slog.Logger }

func NewLogNotifier(log *slog.Logger) *LogNotifier { return &LogNotifier{log: log} }

func (l *LogNotifier) Notify(_ context.Context, n domain.Notification) error {
	l.log.Info("notification",
		"user", n.UserID, "kind", n.Kind, "title", n.Title, "body", n.Body)
	return nil
}
