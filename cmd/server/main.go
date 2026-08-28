package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/handler"
	"github.com/omar-shahieen/url-shortner/internal/middleware"
	"github.com/omar-shahieen/url-shortner/internal/repository/cached"
	"github.com/omar-shahieen/url-shortner/internal/repository/sqlite"
	"github.com/omar-shahieen/url-shortner/internal/routers"
	"github.com/omar-shahieen/url-shortner/internal/service"
	"github.com/omar-shahieen/url-shortner/internal/sweep"
	_ "modernc.org/sqlite"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	database, err := sql.Open("sqlite", "url-shortener.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	sqliteRepo := sqlite.New(database)
	if err := sqliteRepo.Initialize(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Wrap the SQLite repository with the LRU + bloom filter caching layer.
	cachedRepo := cached.New(sqliteRepo, 0, 0)
	if err := cachedRepo.Build(context.Background()); err != nil {
		log.Fatal(err)
	}

	shortener := service.New(cachedRepo)

	// 10 requests/second, burst of 20 per IP on POST /api/shorten.
	rl := middleware.NewRateLimiter(10, 20)

	router := routers.New(handler.New(shortener), database, rl)
	// Wrap the entire mux with structured logging and panic recovery.
	httpHandler := middleware.Logger(logger, middleware.Recoverer(logger, router))

	// --- Graceful shutdown ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start TTL expiry sweep: purge expired rows every 5 minutes.
	go sweep.Run(ctx, sqliteRepo, 5*time.Minute, logger)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: httpHandler,
	}

	go func() {
		logger.Info("URL shortener listening", slog.String("addr", "http://localhost:8080"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Block until a signal is received, then shut down gracefully.
	<-ctx.Done()
	logger.Info("shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("server shutdown:", err)
	}
	logger.Info("shutdown complete")
}
