package httpapi

import (
	"context"
	"net/http"
	"time"

	"rotasavings/internal/domain"
)

type ctxKey int

const requestIDKey ctxKey = iota

// statusRecorder captures the response status code for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// requestID attaches a short correlation id to each request and echoes it back
// in the X-Request-ID header.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = domain.NewID()[:12]
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// accessLog logs one structured line per request with method, path, status, and
// latency.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.inFlight.Add(1)
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		s.metrics.inFlight.Add(-1)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		ms := time.Since(start).Milliseconds()
		s.metrics.observe(status, ms)

		id, _ := r.Context().Value(requestIDKey).(string)
		s.log.Info("request",
			"id", id,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", rec.bytes,
			"ms", ms,
		)
	})
}

// recover turns a panic in any handler into a 500 instead of crashing the
// connection, logging the panic with the request id.
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				id, _ := r.Context().Value(requestIDKey).(string)
				s.log.Error("panic recovered", "id", id, "path", r.URL.Path, "panic", rec)
				writeJSON(w, http.StatusInternalServerError, errorBody{"internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
