package httpapi

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// idempotentResponse is a captured response replayed for repeated keys.
type idempotentResponse struct {
	status  int
	body    []byte
	expires time.Time
}

// Idempotency replays the original response for repeated mutating requests that
// carry the same Idempotency-Key header, so a client retry never double-charges
// or double-creates. In-memory with a TTL; a multi-instance deployment would
// back this with a shared store.
type Idempotency struct {
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]idempotentResponse
	now     func() time.Time
}

func NewIdempotency(ttl time.Duration) *Idempotency {
	idem := &Idempotency{ttl: ttl, entries: make(map[string]idempotentResponse), now: time.Now}
	go idem.gc()
	return idem
}

// capturingWriter buffers the response so it can be both sent and stored.
type capturingWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (c *capturingWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *capturingWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

// Middleware enforces idempotency for unsafe methods carrying an Idempotency-Key.
func (idem *Idempotency) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" || r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		cacheKey := key + " " + r.Method + " " + r.URL.Path

		idem.mu.Lock()
		if e, ok := idem.entries[cacheKey]; ok && idem.now().Before(e.expires) {
			idem.mu.Unlock()
			w.Header().Set("Idempotency-Replayed", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(e.status)
			_, _ = w.Write(e.body)
			return
		}
		idem.mu.Unlock()

		cw := &capturingWriter{ResponseWriter: w}
		next.ServeHTTP(cw, r)

		// Only cache successful, deterministic outcomes.
		if cw.status < 500 {
			idem.mu.Lock()
			idem.entries[cacheKey] = idempotentResponse{
				status:  cw.status,
				body:    append([]byte(nil), cw.buf.Bytes()...),
				expires: idem.now().Add(idem.ttl),
			}
			idem.mu.Unlock()
		}
	})
}

func (idem *Idempotency) gc() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := idem.now()
		idem.mu.Lock()
		for k, e := range idem.entries {
			if now.After(e.expires) {
				delete(idem.entries, k)
			}
		}
		idem.mu.Unlock()
	}
}
