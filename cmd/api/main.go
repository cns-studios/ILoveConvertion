package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fileforge/internal/config"
	"fileforge/internal/database"
	"fileforge/internal/queue"
	"fileforge/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type app struct {
	cfg   *config.Config
	db    *database.DB
	queue *queue.Queue
	store *storage.Storage
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("FileForge API starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	log.Printf("Config loaded (max file size: %d MB, retention: %dh)",
		cfg.MaxFileSize/(1024*1024), cfg.FileRetentionHours)

	db, err := database.New(cfg.DSN())
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	q, err := queue.New(cfg.RedisAddr(), cfg.RedisPoolSize)
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	defer q.Close()

	store, err := storage.New(cfg.StoragePath)
	if err != nil {
		log.Fatalf("Storage error: %v", err)
	}
	log.Printf("Storage ready at %s", cfg.StoragePath)

	a := &app{
		cfg:   cfg,
		db:    db,
		queue: q,
		store: store,
	}

	r := a.buildRouter()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.startCleanup(ctx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Minute,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API server listening on :%d", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down API server...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("API server stopped.")
}

func (a *app) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", a.handleHealth)
		r.Get("/formats", a.handleFormats)
		r.Get("/admin/stats", a.handleAdminStats)

		r.Group(func(r chi.Router) {
			r.Use(a.sessionMiddleware)

			r.Post("/jobs", a.handleCreateJob)
			r.Get("/jobs/{id}", a.handleGetJob)
			r.Get("/jobs/{id}/download", a.handleDownload)
			r.Delete("/jobs/{id}", a.handleDeleteJob)
		})
	})

	return r
}

func (a *app) startCleanup(ctx context.Context) {
	interval := time.Duration(a.cfg.CleanupIntervalMin) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[cleanup] Running every %v", interval)

	a.runCleanup()

	for {
		select {
		case <-ctx.Done():
			log.Println("[cleanup] Stopped")
			return
		case <-ticker.C:
			a.runCleanup()
		}
	}
}

func (a *app) runCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ids, err := a.db.CleanupExpiredJobs(ctx)
	if err != nil {
		log.Printf("[cleanup] expired jobs error: %v", err)
	} else if len(ids) > 0 {
		for _, id := range ids {
			a.store.DeleteJobFiles(id)
		}
		log.Printf("[cleanup] Removed %d expired jobs + files", len(ids))
	}

	n, err := a.db.ResetHourlyCounts(ctx)
	if err != nil {
		log.Printf("[cleanup] reset hourly counts error: %v", err)
	} else if n > 0 {
		log.Printf("[cleanup] Reset hourly counts for %d sessions", n)
	}
}