package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	filecrypto "fileforge/internal/crypto"
	"fileforge/internal/database"
	"fileforge/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbErr := a.db.Ping(r.Context())
	qErr := a.queue.Ping(r.Context())

	if dbErr != nil || qErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":   "degraded",
			"database": errStr(dbErr),
			"redis":    errStr(qErr),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "api",
	})
}

func (a *app) handleFormats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(formatsJSON))
}

func (a *app) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxFileSize+10<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("File too large. Maximum: %s", formatBytes(a.cfg.MaxFileSize)))
			return
		}
		writeError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()

	operation := strings.TrimSpace(r.FormValue("operation"))
	if !models.ValidOperations[operation] {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid operation: %q", operation))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No file provided. Use field name 'file'.")
		return
	}
	defer file.Close()

	if header.Size > a.cfg.MaxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("File too large (%s). Maximum: %s",
				formatBytes(header.Size), formatBytes(a.cfg.MaxFileSize)))
		return
	}

	if header.Size == 0 {
		writeError(w, http.StatusBadRequest, "File is empty")
		return
	}

	inputExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	inputExt = normalizeExt(inputExt)

	if !models.ValidInputFormat(operation, inputExt) {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Unsupported input format .%s for %s", inputExt, operation))
		return
	}

	params, err := parseAndValidateParams(r, operation, inputExt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	session := sessionFromCtx(r)
	if session == nil {
		writeError(w, http.StatusInternalServerError, "Session error")
		return
	}

	job, err := a.db.CreateJob(ctx, database.CreateJobParams{
		SessionID:    session.ID,
		Operation:    operation,
		OriginalName: header.Filename,
		InputSize:    header.Size,
		Params:       params,
	}, a.cfg.FileRetentionHours)
	if err != nil {
		log.Printf("[upload] create job error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	key, err := filecrypto.DeriveKey(a.cfg.MasterKey, job.ID)
	if err != nil {
		log.Printf("[upload] key derivation error: %v", err)
		a.db.DeleteJob(ctx, job.ID)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	dstFile, err := a.store.CreateInput(job.ID)
	if err != nil {
		log.Printf("[upload] create storage file error: %v", err)
		a.db.DeleteJob(ctx, job.ID)
		writeError(w, http.StatusInternalServerError, "Storage error")
		return
	}

	encErr := filecrypto.EncryptStream(key, file, dstFile)
	syncErr := dstFile.Sync()
	dstFile.Close()

	if encErr != nil || syncErr != nil {
		log.Printf("[upload] encrypt error: enc=%v sync=%v", encErr, syncErr)
		a.db.DeleteJob(ctx, job.ID)
		a.store.DeleteJobFiles(job.ID)
		writeError(w, http.StatusInternalServerError, "Failed to process upload")
		return
	}

	if err := a.queue.Enqueue(ctx, job.ID); err != nil {
		log.Printf("[upload] enqueue error: %v", err)
		a.db.DeleteJob(ctx, job.ID)
		a.store.DeleteJobFiles(job.ID)
		writeError(w, http.StatusInternalServerError, "Failed to queue job")
		return
	}

	log.Printf("[upload] Job %s created: %s %s (%s)",
		job.ID, operation, header.Filename, formatBytes(header.Size))

	writeJSON(w, http.StatusCreated, job.ToResponse())
}

func (a *app) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if !isValidUUID(jobID) {
		writeError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := a.db.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Job not found")
		} else {
			log.Printf("[status] db error for %s: %v", jobID, err)
			writeError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	writeJSON(w, http.StatusOK, job.ToResponse())
}

func (a *app) handleDownload(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if !isValidUUID(jobID) {
		writeError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := a.db.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Job not found")
		} else {
			writeError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	if job.Status != models.StatusCompleted {
		switch job.Status {
		case models.StatusPending, models.StatusProcessing:
			writeError(w, http.StatusConflict, "Job is still processing")
		case models.StatusFailed:
			msg := "Job failed"
			if job.ErrorMessage.Valid {
				msg = job.ErrorMessage.String
			}
			writeError(w, http.StatusUnprocessableEntity, msg)
		default:
			writeError(w, http.StatusConflict, "Job not ready for download")
		}
		return
	}

	if !a.store.OutputExists(jobID) {
		writeError(w, http.StatusNotFound, "Output file not found (may have expired)")
		return
	}

	key, err := filecrypto.DeriveKey(a.cfg.MasterKey, jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	encFile, err := a.store.OpenOutput(jobID)
	if err != nil {
		log.Printf("[download] open error for %s: %v", jobID, err)
		writeError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	defer encFile.Close()

	outputName := "download"
	if job.OutputFilename.Valid && job.OutputFilename.String != "" {
		outputName = job.OutputFilename.String
	}

	ext := filepath.Ext(outputName)
	contentType := models.MimeForExtension(ext)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(outputName)))

	if job.OutputSize.Valid && job.OutputSize.Int64 > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(job.OutputSize.Int64, 10))
	}

	w.Header().Set("Cache-Control", "no-store")

	if err := filecrypto.DecryptStream(key, encFile, w); err != nil {
		log.Printf("[download] decrypt stream error for %s: %v", jobID, err)
	}
}

func (a *app) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if !isValidUUID(jobID) {
		writeError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	deleted, err := a.db.DeleteJob(r.Context(), jobID)
	if err != nil {
		log.Printf("[delete] db error for %s: %v", jobID, err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if !deleted {
		writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	a.store.DeleteJobFiles(jobID)

	log.Printf("[delete] Job %s deleted", jobID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"id":     jobID,
	})
}


func (a *app) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.db.GetAdminStats(r.Context())
	if err != nil {
		log.Printf("[admin] stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch stats")
		return
	}

	stats.StorageUsedMB = a.store.UsedMB()

	queueLen, err := a.queue.Length(r.Context())
	if err == nil {
		stats.QueueLength = int(queueLen)
	}

	writeJSON(w, http.StatusOK, stats)
}


func parseAndValidateParams(r *http.Request, operation, inputExt string) (models.JobParams, error) {
	var p models.JobParams

	p.OutputFormat = normalizeExt(strings.TrimSpace(r.FormValue("output_format")))

	switch operation {
	case models.OpImageConvert, models.OpAudioConvert:
		if p.OutputFormat == "" {
			return p, fmt.Errorf("output_format is required for %s", operation)
		}
		if !models.ValidOutputFormat(operation, p.OutputFormat) {
			return p, fmt.Errorf("unsupported output format: %s", p.OutputFormat)
		}

	case models.OpImageRemoveBG:
		if p.OutputFormat == "" {
			p.OutputFormat = "png"
		}
		if !models.ValidOutputFormat(operation, p.OutputFormat) {
			return p, fmt.Errorf("background removal supports png or webp output")
		}

	case models.OpImageCompress, models.OpAudioCompress:
		if p.OutputFormat == "" {
			p.OutputFormat = inputExt
		}

	case models.OpVideoCompress:
		if p.OutputFormat == "" {
			if models.ValidOutputFormat(operation, inputExt) {
				p.OutputFormat = inputExt
			} else {
				p.OutputFormat = "mp4"
			}
		}
		if !models.ValidOutputFormat(operation, p.OutputFormat) {
			return p, fmt.Errorf("unsupported video output format: %s", p.OutputFormat)
		}

	case models.OpPDFCompress:
		p.OutputFormat = "pdf"
	}

	if q := r.FormValue("quality"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || v < 1 || v > 100 {
			return p, fmt.Errorf("quality must be between 1 and 100")
		}
		p.Quality = v
	} else {
		switch operation {
		case models.OpImageCompress:
			p.Quality = 80
		case models.OpAudioCompress:
			p.Quality = 70
		case models.OpVideoCompress:
			p.Quality = 65
		}
	}

	if r.FormValue("lossless") == "true" {
		p.Lossless = true
	}

	if d := r.FormValue("image_dpi"); d != "" {
		v, err := strconv.Atoi(d)
		if err != nil {
			return p, fmt.Errorf("invalid image_dpi value")
		}
		valid := map[int]bool{72: true, 150: true, 300: true, 600: true}
		if !valid[v] {
			return p, fmt.Errorf("image_dpi must be 72, 150, 300, or 600")
		}
		p.ImageDPI = v
	} else if operation == models.OpPDFCompress {
		p.ImageDPI = 150
	}

	if iq := r.FormValue("image_quality"); iq != "" {
		v, err := strconv.Atoi(iq)
		if err != nil || v < 1 || v > 100 {
			return p, fmt.Errorf("image_quality must be between 1 and 100")
		}
		p.ImageQuality = v
	} else if operation == models.OpPDFCompress {
		p.ImageQuality = 75
	}

	return p, nil
}


func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	switch ext {
	case "jpg":
		return "jpeg"
	case "tif":
		return "tiff"
	default:
		return ext
	}
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "\"", "'")
	if name == "" {
		name = "download"
	}
	return name
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

func errStr(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
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