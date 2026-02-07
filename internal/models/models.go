package models

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
)


const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)


const (
	OpImageConvert  = "image_convert"
	OpImageCompress = "image_compress"
	OpImageRemoveBG = "image_remove_bg"
	OpPDFCompress   = "pdf_compress"
	OpAudioConvert  = "audio_convert"
	OpAudioCompress = "audio_compress"
	OpVideoCompress = "video_compress"
)

var ValidOperations = map[string]bool{
	OpImageConvert:  true,
	OpImageCompress: true,
	OpImageRemoveBG: true,
	OpPDFCompress:   true,
	OpAudioConvert:  true,
	OpAudioCompress: true,
	OpVideoCompress: true,
}


var MimeTypes = map[string]string{
	// Images
	"jpeg": "image/jpeg", "jpg": "image/jpeg",
	"png": "image/png", "webp": "image/webp",
	"tiff": "image/tiff", "tif": "image/tiff",
	"gif": "image/gif", "avif": "image/avif",
	"heif": "image/heif", "heic": "image/heic",
	"bmp": "image/bmp",
	// Audio
	"mp3": "audio/mpeg", "wav": "audio/wav",
	"flac": "audio/flac", "ogg": "audio/ogg",
	"opus": "audio/opus", "aac": "audio/aac",
	"m4a": "audio/mp4", "aiff": "audio/aiff",
	"wma": "audio/x-ms-wma",
	// Video
	"mp4": "video/mp4", "mkv": "video/x-matroska",
	"webm": "video/webm", "avi": "video/x-msvideo",
	"mov": "video/quicktime",
	// Document
	"pdf": "application/pdf",
}

func MimeForExtension(ext string) string {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if m, ok := MimeTypes[ext]; ok {
		return m
	}
	return "application/octet-stream"
}


var inputFormats = map[string][]string{
	OpImageConvert:  {"jpeg", "jpg", "png", "webp", "tiff", "tif", "gif", "avif", "heif", "heic", "bmp"},
	OpImageCompress: {"jpeg", "jpg", "png", "webp", "tiff", "tif", "gif", "avif", "heif", "heic", "bmp"},
	OpImageRemoveBG: {"jpeg", "jpg", "png", "webp", "tiff", "tif", "bmp"},
	OpPDFCompress:   {"pdf"},
	OpAudioConvert:  {"mp3", "wav", "flac", "ogg", "opus", "aac", "m4a", "aiff", "wma"},
	OpAudioCompress: {"mp3", "wav", "flac", "ogg", "opus", "aac", "m4a", "aiff", "wma"},
	OpVideoCompress: {"mp4", "mkv", "webm", "avi", "mov"},
}

var outputFormats = map[string][]string{
	OpImageConvert:  {"jpeg", "png", "webp", "tiff", "gif", "avif", "heif", "bmp"},
	OpImageCompress: {}, // same as input
	OpImageRemoveBG: {"png", "webp"},
	OpPDFCompress:   {"pdf"},
	OpAudioConvert:  {"mp3", "wav", "flac", "ogg", "opus", "aac", "m4a", "aiff"},
	OpAudioCompress: {}, // same as input
	OpVideoCompress: {"mp4", "mkv", "webm"},
}

func ValidInputFormat(operation, ext string) bool {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	formats, ok := inputFormats[operation]
	if !ok {
		return false
	}
	for _, f := range formats {
		if f == ext {
			return true
		}
	}
	return false
}

func ValidOutputFormat(operation, ext string) bool {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	formats, ok := outputFormats[operation]
	if !ok {
		return false
	}
	if len(formats) == 0 {
		return ValidInputFormat(operation, ext)
	}
	for _, f := range formats {
		if f == ext {
			return true
		}
	}
	return false
}

type Session struct {
	ID                 string    `json:"id"`
	IPAddress          string    `json:"ip_address"`
	CreatedAt          time.Time `json:"created_at"`
	LastRequestAt      time.Time `json:"last_request_at"`
	HourlyRequestCount int       `json:"hourly_request_count"`
	TotalRequestCount  int       `json:"total_request_count"`
	IsFlagged          bool      `json:"is_flagged"`
}

type Job struct {
	ID             string
	SessionID      string
	Operation      string
	Status         string
	InputFilename  string
	OutputFilename sql.NullString
	InputSize      int64
	OutputSize     sql.NullInt64
	OriginalName   string
	Params         json.RawMessage
	FileNonce      []byte
	ErrorMessage   sql.NullString
	RetryCount     int
	CreatedAt      time.Time
	StartedAt      sql.NullTime
	CompletedAt    sql.NullTime
	ExpiresAt      time.Time
}

func (j *Job) InputExt() string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(j.OriginalName)), ".")
}

func (j *Job) ToResponse() JobResponse {
	resp := JobResponse{
		ID:           j.ID,
		Operation:    j.Operation,
		Status:       j.Status,
		InputSize:    j.InputSize,
		OriginalName: j.OriginalName,
		CreatedAt:    j.CreatedAt,
	}

	if j.OutputSize.Valid {
		v := j.OutputSize.Int64
		resp.OutputSize = &v
	}
	if j.OutputFilename.Valid {
		v := j.OutputFilename.String
		resp.OutputFilename = &v
	}
	if j.ErrorMessage.Valid {
		v := j.ErrorMessage.String
		resp.ErrorMessage = &v
	}
	if j.CompletedAt.Valid {
		v := j.CompletedAt.Time
		resp.CompletedAt = &v
	}
	if j.StartedAt.Valid {
		v := j.StartedAt.Time
		resp.StartedAt = &v
	}

	return resp
}

type JobResponse struct {
	ID             string     `json:"id"`
	Operation      string     `json:"operation"`
	Status         string     `json:"status"`
	InputSize      int64      `json:"input_size"`
	OutputSize     *int64     `json:"output_size,omitempty"`
	OriginalName   string     `json:"original_name"`
	OutputFilename *string    `json:"output_filename,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type AdminStats struct {
	QueueLength    int   `json:"queue_length"`
	ActiveJobs     int   `json:"active_jobs"`
	Completed24h   int   `json:"completed_24h"`
	Failed24h      int   `json:"failed_24h"`
	ActiveSessions int   `json:"active_sessions"`
	StorageUsedMB  int64 `json:"storage_used_mb"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}


type JobParams struct {
	OutputFormat string `json:"output_format,omitempty"`
	Quality      int    `json:"quality,omitempty"`
	Lossless     bool   `json:"lossless,omitempty"`
	ImageDPI     int    `json:"image_dpi,omitempty"`
	ImageQuality int    `json:"image_quality,omitempty"`
}

func ParseParams(raw json.RawMessage) (JobParams, error) {
	var p JobParams
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return p, nil
	}
	err := json.Unmarshal(raw, &p)
	return p, err
}

func OutputName(originalName, outputFormat string) string {
	if outputFormat == "" {
		return originalName
	}
	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	return base + "." + strings.ToLower(outputFormat)
}