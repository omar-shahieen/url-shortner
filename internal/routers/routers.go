// Package routers configures the application's HTTP routes.
package routers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/omar-shahieen/url-shortner/internal/handler"
)

// HealthChecker verifies whether a backing dependency is reachable.
type HealthChecker interface {
	PingContext(context.Context) error
}

// New returns the application's HTTP router.
func New(handler *handler.Handler, healthChecker HealthChecker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.Preview)
	mux.HandleFunc("POST /api/shorten", handler.Shorten)
	mux.HandleFunc("GET /api/stats/{code}", handler.Stats)
	mux.HandleFunc("GET /health", health(healthChecker))
	mux.HandleFunc("GET /{code}", handler.Redirect)
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
