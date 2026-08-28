// Package routers configures the application's HTTP routes.
package routers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/middleware"
)

// HealthChecker verifies whether a backing dependency is reachable.
type HealthChecker interface {
	PingContext(context.Context) error
}

// New returns the application's HTTP router with logging, panic recovery, and
// per-IP rate limiting on POST /api/shorten applied.
func New(h *handler.Handler, healthChecker HealthChecker, rl *middleware.RateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.Preview)
	// Rate-limit only the write endpoint.
	mux.Handle("POST /api/shorten", middleware.Limit(rl, http.HandlerFunc(h.Shorten)))
	mux.HandleFunc("GET /api/stats/{code}", h.Stats)
	mux.HandleFunc("GET /health", health(healthChecker))
	mux.HandleFunc("GET /{code}", h.Redirect)
	return mux
}

func health(healthChecker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := healthChecker.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
