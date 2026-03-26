package linkchecker

import (
	"net/http"
	"time"
)

const checkerUserAgent = "Mozilla/5.0 (compatible; WebAnalyzer/1.0)"

// LinkStatus represents the three possible outcomes of checking a link.
type LinkStatus string

const (
	// StatusAccessible: response received with a non-error, non-refused HTTP status.
	StatusAccessible LinkStatus = "accessible"
	// StatusInaccessible: network error, 4xx (excl. 401/403/429), or 5xx.
	StatusInaccessible LinkStatus = "inaccessible"
	// StatusUnverified: server actively refused the check (401, 403, 429).
	// The link may well exist; we could not confirm it.
	StatusUnverified LinkStatus = "unverified"
)

// CheckResult is the outcome of checking a single URL.
type CheckResult struct {
	URL        string
	Status     LinkStatus
	StatusCode int
	Err        error
}

// CheckerConfig holds tuning parameters for the global worker pool.
type CheckerConfig struct {
	MaxWorkers    int
	JobBufferSize int
	Timeout       time.Duration
	Retries       int
}

// linkCheckJob carries one URL and the per-call result channel back to CheckAll.
type linkCheckJob struct {
	url        string
	resultChan chan CheckResult
}

// GlobalLinkChecker manages a shared pool of HTTP workers reused across all jobs.
// Workers are started once at boot and run until the root context is cancelled.
type GlobalLinkChecker struct {
	jobs   chan linkCheckJob
	client *http.Client
	config CheckerConfig
}
