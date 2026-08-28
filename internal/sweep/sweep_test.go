package sweep_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/sweep"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}


// fakePurger counts how many times DeleteExpired is called.
type fakePurger struct {
	calls atomic.Int32
}

func (f *fakePurger) DeleteExpired(_ context.Context) (int64, error) {
	f.calls.Add(1)
	return 0, nil
}

func TestRunCallsPurgerOnTick(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	purger := &fakePurger{}
	go sweep.Run(ctx, purger, 50*time.Millisecond, noopLogger())

	// Wait for at least 3 ticks.
	time.Sleep(200 * time.Millisecond)
	if purger.calls.Load() < 3 {
		t.Errorf("expected ≥ 3 DeleteExpired calls in 200 ms (50 ms tick), got %d", purger.calls.Load())
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	purger := &fakePurger{}
	done := make(chan struct{})

	go func() {
		sweep.Run(ctx, purger, 10*time.Millisecond, noopLogger())
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// good — goroutine exited
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sweep goroutine did not stop within 500 ms after context cancel")
	}
}
