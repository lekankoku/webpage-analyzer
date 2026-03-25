package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	envHTTPAddr             = "WEB_ANALYZER_HTTP_ADDR"
	envMaxConcurrentJobs    = "WEB_ANALYZER_MAX_CONCURRENT_JOBS"
	envJobTTL               = "WEB_ANALYZER_JOB_TTL"
	envShutdownTimeout      = "WEB_ANALYZER_SHUTDOWN_TIMEOUT"
	envCheckerMaxWorkers    = "WEB_ANALYZER_LINK_CHECKER_MAX_WORKERS"
	envCheckerJobBufferSize = "WEB_ANALYZER_LINK_CHECKER_JOB_BUFFER_SIZE"
	envCheckerTimeout       = "WEB_ANALYZER_LINK_CHECKER_TIMEOUT"
	envCheckerRetries       = "WEB_ANALYZER_LINK_CHECKER_RETRIES"
	envPageFetchTimeout     = "WEB_ANALYZER_PAGE_FETCH_TIMEOUT"
)

// Config is the app runtime configuration assembled from environment variables.
type Config struct {
	HTTPAddr              string
	MaxConcurrentJobs     int
	JobTTL                time.Duration
	ShutdownTimeout       time.Duration
	LinkCheckerMaxWorkers int
	LinkCheckerJobBuffer  int
	LinkCheckerTimeout    time.Duration
	LinkCheckerRetries    int
	PageFetchTimeout      time.Duration
}

// Load loads .env (if present), then reads env vars with safe defaults.
func Load() Config {
	_ = godotenv.Load()
	env := newEnvSource()

	return Config{
		HTTPAddr:              env.String(envHTTPAddr, ":8080"),
		MaxConcurrentJobs:     env.NonNegativeInt(envMaxConcurrentJobs, 10),
		JobTTL:                env.NonNegativeDuration(envJobTTL, time.Hour),
		ShutdownTimeout:       env.NonNegativeDuration(envShutdownTimeout, 30*time.Second),
		LinkCheckerMaxWorkers: env.NonNegativeInt(envCheckerMaxWorkers, 100),
		LinkCheckerJobBuffer:  env.NonNegativeInt(envCheckerJobBufferSize, 500),
		LinkCheckerTimeout:    env.NonNegativeDuration(envCheckerTimeout, 10*time.Second),
		LinkCheckerRetries:    env.NonNegativeInt(envCheckerRetries, 2),
		PageFetchTimeout:      env.NonNegativeDuration(envPageFetchTimeout, 10*time.Second),
	}
}

// envSource encapsulates env parsing and validation concerns.
type envSource struct{}

func newEnvSource() envSource {
	return envSource{}
}

func (e envSource) String(key, fallback string) string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}

func (e envSource) NonNegativeInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		log.Printf("invalid %s=%q, using default %d", key, raw, fallback)
		return fallback
	}
	if parsed < 0 {
		log.Printf("invalid %s=%d (must be >= 0), using default %d", key, parsed, fallback)
		return fallback
	}
	return parsed
}

func (e envSource) NonNegativeDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		log.Printf("invalid %s=%q, using default %s", key, raw, fallback)
		return fallback
	}
	if parsed < 0 {
		log.Printf("invalid %s=%s (must be >= 0), using default %s", key, parsed, fallback)
		return fallback
	}
	return parsed
}
