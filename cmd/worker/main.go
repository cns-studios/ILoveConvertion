package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("FileForge Worker starting...")

	concurrency := getEnvInt("WORKER_CONCURRENCY", 4)
	log.Printf("Worker concurrency: %d", concurrency)

	for _, dir := range []string{"/app/storage/inputs", "/app/storage/outputs"} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Fatalf("Storage directory missing: %s", dir)
		}
	}

	tmpDir := "/tmp/processing"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		log.Printf("WARNING: tmpfs directory %s not found, creating...", tmpDir)
		if err := os.MkdirAll(tmpDir, 0700); err != nil {
			log.Fatalf("Failed to create tmpfs dir: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// Worker loop placeholder ──
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			log.Printf("Worker goroutine %d started (stub — waiting for Block 3)", workerID)
			<-ctx.Done()
			log.Printf("Worker goroutine %d stopped", workerID)
		}(i)
	}

	log.Printf("Worker ready — %d goroutines listening (stub mode)", concurrency)

	<-done
	log.Println("Shutting down worker...")
	cancel()
	time.Sleep(2 * time.Second)
	log.Println("Worker stopped.")
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