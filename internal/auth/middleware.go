package auth

import (
	"context"
	"net/http"
	"strings"

	"rotasavings/internal/domain"
)

type ctxKey int

const principalKey ctxKey = iota

// Principal is the authenticated caller attached to a request context.
type Principal struct {
	UserID string
	Role   domain.Role
}

// WithPrincipal stores a principal on a context (used in tests and handlers).
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom extracts the authenticated principal, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Middleware enforces auth using the Issuer.
type Middleware struct {
	issuer *Issuer
}

func NewMiddleware(issuer *Issuer) *Middleware { return &Middleware{issuer: issuer} }

// Authenticate parses the bearer token if present and attaches the principal.
// It does NOT reject anonymous requests; use RequireAuth for that. This lets a
// route be optionally authenticated.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(h, prefix) {
			if claims, err := m.issuer.Verify(strings.TrimPrefix(h, prefix)); err == nil {
				r = r.WithContext(WithPrincipal(r.Context(), Principal{
					UserID: claims.Sub,
					Role:   claims.Role,
				}))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth rejects requests without a valid principal.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFrom(r.Context()); !ok {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r)
	}
}

// RequireAdmin rejects requests whose principal is not an admin.
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if p.Role != domain.RoleAdmin {
			writeErr(w, http.StatusForbidden, "admin role required")
			return
		}
		next(w, r)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
