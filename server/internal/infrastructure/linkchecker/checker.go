package linkchecker

import (
	"context"
	"time"
)

// CheckerConfig holds tuning parameters for the global link-checker worker pool.
type CheckerConfig struct {
	MaxWorkers    int
	JobBufferSize int
	Timeout       time.Duration
	Retries       int
}

// GlobalLinkChecker manages a shared pool of HTTP workers used by all concurrent
// analysis jobs. Full implementation (CheckAll, HEAD→GET retry logic) added in Step 9.
type GlobalLinkChecker struct {
	config CheckerConfig
}

// New returns a configured GlobalLinkChecker. Call Start before submitting any work.
func New(config CheckerConfig) *GlobalLinkChecker {
	return &GlobalLinkChecker{config: config}
}

// Start launches the worker pool goroutines. Call once from main.go with the root context.
// Workers stop when ctx is cancelled.
func (c *GlobalLinkChecker) Start(_ context.Context) {}
