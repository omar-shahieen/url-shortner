// Package middleware provides reusable HTTP middleware for the URL shortener.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// --------------------------------------------------------------------------
// Logging + panic recovery
// --------------------------------------------------------------------------

// responseRecorder captures the status code written by a handler so the
// logger can include it in the access-log line.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

// Logger returns middleware that logs every request (method, path, status,
// duration) using the structured logger l.
func Logger(l *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)
		l.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rr.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// Recoverer returns middleware that catches panics, logs a stack trace, and
// responds with 500 Internal Server Error so the server stays up.
func Recoverer(l *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				l.Error("panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --------------------------------------------------------------------------
// Token-bucket rate limiter (per-IP)
// --------------------------------------------------------------------------

// bucket holds the state for one IP address.
type bucket struct {
	mu     sync.Mutex
	tokens float64
	lastAt time.Time
}

// RateLimiter is a per-IP token-bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens added per second
	capacity float64 // max tokens (= burst size)
}

// NewRateLimiter returns a RateLimiter that allows rate requests/second per IP
// with a burst of burst.
func NewRateLimiter(rate, burst float64) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: burst,
	}
}

func (rl *RateLimiter) getBucket(ip string) *bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{tokens: rl.capacity, lastAt: time.Now()}
		rl.buckets[ip] = b
	}
	return b
}

// Allow returns true if the request from ip should be permitted.
func (rl *RateLimiter) Allow(ip string) bool {
	b := rl.getBucket(ip)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastAt).Seconds()
	b.lastAt = now
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Limit returns middleware that applies rl to every request, responding with
// 429 Too Many Requests when the bucket is empty. The IP is taken from
// r.RemoteAddr (suitable for a direct-connect server; put a proxy's X-Real-IP
// header here if needed).
func Limit(rl *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !rl.Allow(ip) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
