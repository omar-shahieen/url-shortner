package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/middleware"
)

// --------------------------------------------------------------------------
// Logger middleware
// --------------------------------------------------------------------------

func TestLoggerPassesThrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := middleware.Logger(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Logger status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

// --------------------------------------------------------------------------
// Recoverer middleware
// --------------------------------------------------------------------------

func TestRecovererCatchesPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := middleware.Recoverer(logger, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Recoverer status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestRecovererPassesThroughNormalHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := middleware.Recoverer(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Recoverer (no panic) status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// RateLimiter
// --------------------------------------------------------------------------

func TestRateLimiterAllowsBurst(t *testing.T) {
	const burst = 5
	rl := middleware.NewRateLimiter(1, burst)

	for i := 0; i < burst; i++ {
		if !rl.Allow("127.0.0.1:1234") {
			t.Fatalf("request %d should be allowed (within burst)", i+1)
		}
	}
}

func TestRateLimiterBlocksAfterBurst(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 2) // 1 req/s, burst=2

	rl.Allow("10.0.0.1:1") // token 1
	rl.Allow("10.0.0.1:1") // token 2
	if rl.Allow("10.0.0.1:1") {
		t.Error("third request should be blocked — bucket is empty")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	// rate=10/s, burst=1 → after 200 ms one new token should arrive
	rl := middleware.NewRateLimiter(10, 1)

	if !rl.Allow("1.2.3.4:0") {
		t.Fatal("first request should be allowed")
	}
	// bucket empty; wait for refill
	time.Sleep(150 * time.Millisecond)
	if !rl.Allow("1.2.3.4:0") {
		t.Error("should be allowed after refill delay")
	}
}

func TestRateLimiterIsolatesIPs(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 1)
	rl.Allow("192.168.1.1:0") // exhaust ip1

	// ip2 should still have its full bucket
	if !rl.Allow("192.168.1.2:0") {
		t.Error("different IP should not be affected by another IP's bucket")
	}
}

func TestLimitMiddleware429(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.Limit(rl, inner)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", nil)
	req.RemoteAddr = "5.5.5.5:9999"

	// First request: allowed
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Errorf("first request status = %d, want 200", rr1.Code)
	}

	// Second request: bucket empty → 429
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want 429", rr2.Code)
	}
}
