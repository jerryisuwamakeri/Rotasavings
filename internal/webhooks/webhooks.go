// Package webhooks delivers platform events to operator-registered HTTP
// endpoints. Delivery is best-effort and fire-and-forget: each event is POSTed
// as JSON to every active webhook, optionally signed with an HMAC-SHA256
// signature in the X-Rota-Signature header.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"rotasavings/internal/domain"
	"rotasavings/internal/store"
)

// Dispatcher fans events out to registered webhooks.
type Dispatcher struct {
	store  store.Store
	client *http.Client
	log    *slog.Logger
}

// New builds a Dispatcher.
func New(s store.Store, log *slog.Logger) *Dispatcher {
	return &Dispatcher{store: s, client: &http.Client{Timeout: 5 * time.Second}, log: log}
}

// Dispatch sends an event to all active webhooks, asynchronously. It never
// blocks the caller and never returns an error - webhook delivery must not
// affect core operations.
func (d *Dispatcher) Dispatch(ctx context.Context, evt domain.WebhookEvent) {
	hooks, err := d.store.ListWebhooks(ctx)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return
	}
	for _, h := range hooks {
		if !h.Active {
			continue
		}
		go d.deliver(*h, body)
	}
}

func (d *Dispatcher) deliver(h domain.Webhook, body []byte) {
	req, err := http.NewRequest(http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Rotasavings-Webhook/1")
	if h.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.Secret))
		mac.Write(body)
		req.Header.Set("X-Rota-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		d.log.Warn("webhook delivery failed", "url", h.URL, "err", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		d.log.Warn("webhook non-2xx", "url", h.URL, "status", resp.StatusCode)
	}
}
