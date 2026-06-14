package httpapi

import (
	"net/http"
	"strings"
)

// cors applies CORS headers and short-circuits preflight (OPTIONS) requests so a
// browser app on a different origin (e.g. the Next.js dashboard on :3000) can
// call the API. Origins are an allowlist, or "*" for any (dev default).
//
// We authenticate with bearer tokens, not cookies, so credentialed CORS is not
// needed; "*" is therefore safe for local development. In production, set
// ROTA_CORS_ORIGINS to the exact dashboard origin(s).
func (s *Server) cors(allowed string) func(http.Handler) http.Handler {
	origins := map[string]bool{}
	any := strings.TrimSpace(allowed) == "*"
	if !any {
		for _, o := range strings.Split(allowed, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins[o] = true
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (any || origins[origin]) {
				if any {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
