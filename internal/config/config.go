package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIPort int

	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	RedisHost     string
	RedisPort     int
	RedisPoolSize int

	MasterKey []byte 

	RateLimitPerHour int
	FlagThreshold    int

	MaxFileSize        int64
	StoragePath        string
	CleanupIntervalMin int
	FileRetentionHours int

	WorkerConcurrency int
	RembgURL          string
	TmpDir            string

	Timeouts map[string]time.Duration

	Retries map[string]int
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.RedisHost, c.RedisPort)
}

func (c *Config) TimeoutFor(operation string) time.Duration {
	if d, ok := c.Timeouts[operation]; ok {
		return d
	}
	return 5 * time.Minute
}

func (c *Config) MaxRetriesFor(operation string) int {
	if r, ok := c.Retries[operation]; ok {
		return r
	}
	return 2
}

func Load() (*Config, error) {
	// ── Master encryption key (required) ──
	masterKeyHex := os.Getenv("ENCRYPTION_MASTER_KEY")
	if masterKeyHex == "" {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY is required (generate with: openssl rand -hex 32)")
	}

	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY must be valid hex: %w", err)
	}
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY must be 64 hex chars (32 bytes), got %d bytes", len(masterKey))
	}

	cfg := &Config{
		APIPort: envInt("API_PORT", 3015),

		DBHost:     envStr("POSTGRES_HOST", "postgres"),
		DBPort:     envInt("POSTGRES_PORT", 5432),
		DBUser:     envStr("POSTGRES_USER", "fileforge"),
		DBPassword: envStr("POSTGRES_PASSWORD", "changeme"),
		DBName:     envStr("POSTGRES_DB", "fileforge"),

		RedisHost:     envStr("REDIS_HOST", "redis"),
		RedisPort:     envInt("REDIS_PORT", 6379),
		RedisPoolSize: envInt("REDIS_POOL_SIZE", 10),

		MasterKey:        masterKey,
		RateLimitPerHour: envInt("RATE_LIMIT_PER_HOUR", 60),
		FlagThreshold:    envInt("FLAG_THRESHOLD", 200),

		MaxFileSize:        envInt64("MAX_FILE_SIZE", 524288000), // 500MB default
		StoragePath:        envStr("STORAGE_PATH", "/app/storage"),
		CleanupIntervalMin: envInt("CLEANUP_INTERVAL_MINUTES", 10),
		FileRetentionHours: envInt("FILE_RETENTION_HOURS", 24),

		WorkerConcurrency: envInt("WORKER_CONCURRENCY", 4),
		RembgURL:          envStr("REMBG_URL", "http://rembg:5000"),
		TmpDir:            envStr("TMP_DIR", "/tmp/processing"),

		Timeouts: map[string]time.Duration{
			"image_convert":   secDuration(envInt("TIMEOUT_IMAGE_CONVERT", 120)),
			"image_compress":  secDuration(envInt("TIMEOUT_IMAGE_COMPRESS", 120)),
			"image_remove_bg": secDuration(envInt("TIMEOUT_IMAGE_REMOVE_BG", 180)),
			"pdf_compress":    secDuration(envInt("TIMEOUT_PDF_COMPRESS", 300)),
			"audio_convert":   secDuration(envInt("TIMEOUT_AUDIO_CONVERT", 300)),
			"audio_compress":  secDuration(envInt("TIMEOUT_AUDIO_COMPRESS", 300)),
			"video_compress":  secDuration(envInt("TIMEOUT_VIDEO_COMPRESS", 1800)),
		},

		Retries: map[string]int{
			"image_convert":   envInt("RETRY_IMAGE", 2),
			"image_compress":  envInt("RETRY_IMAGE", 2),
			"image_remove_bg": envInt("RETRY_IMAGE", 2),
			"pdf_compress":    envInt("RETRY_PDF", 2),
			"audio_convert":   envInt("RETRY_AUDIO", 2),
			"audio_compress":  envInt("RETRY_AUDIO", 2),
			"video_compress":  envInt("RETRY_VIDEO", 1),
		},
	}

	return cfg, nil
}


func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func secDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}