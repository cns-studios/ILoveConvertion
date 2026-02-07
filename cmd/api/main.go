package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("FileForge API starting...")

	port := getEnv("API_PORT", "3000")

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"api"}`))
	})

	r.Get("/api/formats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(formatsJSON))
	})

	r.Post("/api/jobs", stubHandler("POST /api/jobs"))
	r.Get("/api/jobs/{id}", stubHandler("GET /api/jobs/{id}"))
	r.Get("/api/jobs/{id}/download", stubHandler("GET /api/jobs/{id}/download"))
	r.Delete("/api/jobs/{id}", stubHandler("DELETE /api/jobs/{id}"))
	r.Get("/api/admin/stats", stubHandler("GET /api/admin/stats"))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       600 * time.Second,
		WriteTimeout:      600 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down API server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("API server stopped.")
}

func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, `{"error":"not implemented","endpoint":"%s"}`, name)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

const formatsJSON = `{
  "image_convert": {
    "input": ["jpeg","jpg","png","webp","tiff","tif","gif","avif","heif","heic","bmp"],
    "output": ["jpeg","png","webp","tiff","gif","avif","heif","bmp"]
  },
  "image_compress": {
    "input": ["jpeg","jpg","png","webp","tiff","tif","gif","avif","heif","heic","bmp"],
    "output": "same_as_input",
    "params": {"quality":{"type":"range","min":1,"max":100,"default":80},"lossless":{"type":"bool","default":false}}
  },
  "image_remove_bg": {
    "input": ["jpeg","jpg","png","webp","tiff","tif","bmp"],
    "output": ["png","webp"],
    "default_output": "png"
  },
  "pdf_compress": {
    "input": ["pdf"],
    "output": ["pdf"],
    "params": {"image_dpi":{"type":"select","options":[72,150,300,600],"default":150},"image_quality":{"type":"range","min":1,"max":100,"default":75}}
  },
  "audio_convert": {
    "input": ["mp3","wav","flac","ogg","opus","aac","m4a","aiff","wma"],
    "output": ["mp3","wav","flac","ogg","opus","aac","m4a","aiff"]
  },
  "audio_compress": {
    "input": ["mp3","wav","flac","ogg","opus","aac","m4a","aiff","wma"],
    "output": "same_as_input",
    "params": {"quality":{"type":"range","min":1,"max":100,"default":70},"lossless":{"type":"bool","default":false}}
  },
  "video_compress": {
    "input": ["mp4","mkv","webm","avi","mov"],
    "output": ["mp4","mkv","webm"],
    "params": {"quality":{"type":"range","min":1,"max":100,"default":65}}
  }
}`