package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"fileforge/internal/config"
	filecrypto "fileforge/internal/crypto"
	"fileforge/internal/database"
	"fileforge/internal/models"
	"fileforge/internal/processor"
	"fileforge/internal/queue"
	"fileforge/internal/storage"
)

type worker struct {
	cfg   *config.Config
	db    *database.DB
	queue *queue.Queue
	store *storage.Storage
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("FileForge Worker starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

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

	if err := os.MkdirAll(cfg.TmpDir, 0700); err != nil {
		log.Fatalf("tmpfs directory error: %v", err)
	}

	w := &worker{cfg: cfg, db: db, queue: q, store: store}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	for i := 0; i < cfg.WorkerConcurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.run(ctx, id)
		}(i)
	}

	log.Printf("Worker ready — %d goroutines listening on queue", cfg.WorkerConcurrency)

	<-done
	log.Println("Shutting down worker...")
	cancel()

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		log.Println("All worker goroutines stopped gracefully")
	case <-time.After(2 * time.Minute):
		log.Println("Shutdown timeout — some jobs may not have completed cleanly")
	}

	log.Println("Worker stopped.")
}

func (w *worker) run(ctx context.Context, id int) {
	log.Printf("[worker-%d] Started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker-%d] Context cancelled, stopping", id)
			return
		default:
		}

		jobID, err := w.queue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[worker-%d] Dequeue error: %v", id, err)
			time.Sleep(time.Second)
			continue
		}

		if jobID == "" {
			continue
		}

		w.processJob(ctx, id, jobID)
	}
}

func (w *worker) processJob(ctx context.Context, workerID int, jobID string) {
	startTime := time.Now()
	log.Printf("[worker-%d] ▶ Job %s", workerID, jobID)

	tmpDir := filepath.Join(w.cfg.TmpDir, jobID)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		log.Printf("[worker-%d] ✗ tmpdir error: %v", workerID, err)
		w.failJob(ctx, jobID, "Internal error: failed to create temp directory")
		return
	}
	defer os.RemoveAll(tmpDir)

	job, err := w.db.GetJob(ctx, jobID)
	if err != nil {
		log.Printf("[worker-%d] ✗ fetch job error: %v", workerID, err)
		return
	}

	if job.Status != models.StatusPending {
		log.Printf("[worker-%d] ⊘ Job %s status=%q, skipping", workerID, jobID, job.Status)
		return
	}

	if err := w.db.UpdateJobStarted(ctx, jobID); err != nil {
		log.Printf("[worker-%d] ✗ update started error: %v", workerID, err)
		return
	}

	key, err := filecrypto.DeriveKey(w.cfg.MasterKey, jobID)
	if err != nil {
		log.Printf("[worker-%d] ✗ key derivation error: %v", workerID, err)
		w.failJob(ctx, jobID, "Encryption key derivation failed")
		return
	}

	params, err := models.ParseParams(job.Params)
	if err != nil {
		log.Printf("[worker-%d] ✗ parse params error: %v", workerID, err)
		w.failJob(ctx, jobID, fmt.Sprintf("Invalid parameters: %v", err))
		return
	}

	inputExt := job.InputExt()
	if inputExt == "" {
		inputExt = "bin"
	}
	tmpInput := filepath.Join(tmpDir, "input."+inputExt)

	log.Printf("[worker-%d] Decrypting %s → %s", workerID, w.store.InputPath(jobID), tmpInput)

	if err := filecrypto.DecryptFile(key, w.store.InputPath(jobID), tmpInput); err != nil {
		log.Printf("[worker-%d] ✗ decrypt error: %v", workerID, err)
		w.failJob(ctx, jobID, fmt.Sprintf("Failed to decrypt input: %v", err))
		return
	}

	outExt := params.OutputFormat
	if outExt == "" {
		outExt = inputExt
	}
	tmpOutput := filepath.Join(tmpDir, "output."+outExt)

	log.Printf("[worker-%d] Processing %s: %s → .%s", workerID, job.Operation, job.OriginalName, outExt)

	timeout := w.cfg.TimeoutFor(job.Operation)
	processCtx, processCancel := context.WithTimeout(ctx, timeout)
	processErr := w.dispatch(processCtx, job.Operation, tmpInput, tmpOutput, tmpDir, params)
	processCancel()

	if processErr != nil {
		log.Printf("[worker-%d] ✗ process error: %v", workerID, processErr)
		w.handleProcessError(ctx, workerID, jobID, job.Operation, processErr)
		return
	}

	outputInfo, err := os.Stat(tmpOutput)
	if err != nil || outputInfo.Size() == 0 {
		log.Printf("[worker-%d] ✗ output missing or empty: err=%v", workerID, err)
		w.failJob(ctx, jobID, "Processing completed but output file is missing or empty")
		return
	}
	outputSize := outputInfo.Size()

	log.Printf("[worker-%d] Encrypting output (%s) → %s", workerID, formatBytes(outputSize), w.store.OutputPath(jobID))

	if err := filecrypto.EncryptFile(key, tmpOutput, w.store.OutputPath(jobID)); err != nil {
		log.Printf("[worker-%d] ✗ encrypt output error: %v", workerID, err)
		w.failJob(ctx, jobID, fmt.Sprintf("Failed to encrypt output: %v", err))
		return
	}

	outputFilename := models.OutputName(job.OriginalName, params.OutputFormat)

	if err := w.db.UpdateJobCompleted(ctx, jobID, outputFilename, outputSize); err != nil {
		log.Printf("[worker-%d] ✗ update completed error: %v", workerID, err)
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)
	log.Printf("[worker-%d] ✓ Job %s done in %v: %s → %s (%s → %s)",
		workerID, jobID, elapsed,
		job.OriginalName, outputFilename,
		formatBytes(job.InputSize), formatBytes(outputSize))
}

func (w *worker) dispatch(ctx context.Context, operation, inputPath, outputPath, tmpDir string, params models.JobParams) error {
	switch operation {
	case models.OpImageConvert:
		return processor.ImageConvert(ctx, inputPath, outputPath, params)
	case models.OpImageCompress:
		return processor.ImageCompress(ctx, inputPath, outputPath, params)
	case models.OpImageRemoveBG:
		return processor.ImageRemoveBG(ctx, inputPath, outputPath, w.cfg.RembgURL, params)
	case models.OpPDFCompress:
		return processor.PDFCompress(ctx, inputPath, outputPath, tmpDir, params)
	case models.OpAudioConvert:
		return processor.AudioConvert(ctx, inputPath, outputPath, params)
	case models.OpAudioCompress:
		return processor.AudioCompress(ctx, inputPath, outputPath, params)
	case models.OpVideoCompress:
		return processor.VideoCompress(ctx, inputPath, outputPath, params)
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

func (w *worker) handleProcessError(ctx context.Context, workerID int, jobID, operation string, processErr error) {
	retryCount, err := w.db.IncrementRetryCount(ctx, jobID)
	if err != nil {
		log.Printf("[worker-%d] Retry count increment failed for %s: %v", workerID, jobID, err)
		w.failJob(ctx, jobID, processErr.Error())
		return
	}

	maxRetries := w.cfg.MaxRetriesFor(operation)

	if retryCount <= maxRetries {
		log.Printf("[worker-%d] ↻ Job %s failed (attempt %d/%d): %v — requeuing",
			workerID, jobID, retryCount, maxRetries, processErr)
		if err := w.queue.Requeue(ctx, jobID); err != nil {
			log.Printf("[worker-%d] Requeue failed for %s: %v", workerID, jobID, err)
			w.failJob(ctx, jobID, processErr.Error())
		}
	} else {
		log.Printf("[worker-%d] ✗ Job %s permanently failed after %d attempts: %v",
			workerID, jobID, retryCount, processErr)
		w.failJob(ctx, jobID, processErr.Error())
	}
}

func (w *worker) failJob(ctx context.Context, jobID, msg string) {
	// Truncate very long error messages
	if len(msg) > 1000 {
		msg = msg[:1000] + "…"
	}
	if err := w.db.UpdateJobFailed(ctx, jobID, msg); err != nil {
		log.Printf("[worker] Failed to mark job %s as failed: %v", jobID, err)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}