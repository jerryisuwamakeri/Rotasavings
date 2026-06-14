package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// tokenBucket is a simple per-client token bucket: rate tokens/sec, burst depth.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter applies per-client (per-IP) token-bucket rate limiting. It is
// in-memory and adequate for a single instance; a distributed deployment would
// back this with Redis. Buckets idle out to bound memory.
type RateLimiter struct {
	rate    float64 // tokens per second
	burst   float64
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	now     func() time.Time
}

// NewRateLimiter builds a limiter allowing `rate` requests/sec with `burst`
// capacity per client.
func NewRateLimiter(ratePerSec, burst float64) *RateLimiter {
	rl := &RateLimiter{
		rate:    ratePerSec,
		burst:   burst,
		buckets: make(map[string]*tokenBucket),
		now:     time.Now,
	}
	go rl.gc()
	return rl
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &tokenBucket{tokens: rl.burst - 1, last: now}
		return true
	}
	// Refill based on elapsed time.
	b.tokens += now.Sub(b.last).Seconds() * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gc periodically drops buckets that have been idle, to bound memory.
func (rl *RateLimiter) gc() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := rl.now().Add(-5 * time.Minute)
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.last.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware enforces the limit, returning 429 with a Retry-After header.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, errorBody{"rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
