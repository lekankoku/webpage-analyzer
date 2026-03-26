package linkchecker

import (
	"context"
	"net/http"
)

// New returns a GlobalLinkChecker with a default HTTP client. Call Start before use.
func New(config CheckerConfig) *GlobalLinkChecker {
	client := &http.Client{
		Timeout: config.Timeout,
	}
	return NewWithClient(config, client)
}

// NewWithClient allows injecting a custom *http.Client for testing.
func NewWithClient(config CheckerConfig, client *http.Client) *GlobalLinkChecker {
	return &GlobalLinkChecker{
		jobs:   make(chan linkCheckJob, config.JobBufferSize),
		client: client,
		config: config,
	}
}

// ClassifyResult maps an HTTP status code and error to a LinkStatus.
// This is a pure function — fully testable in isolation.
func ClassifyResult(statusCode int, err error) LinkStatus {
	if err != nil {
		return StatusInaccessible
	}
	switch statusCode {
	case 401, 403, 429:
		return StatusUnverified
	}
	if statusCode >= 400 {
		return StatusInaccessible
	}
	return StatusAccessible
}

// Start launches the worker goroutines against the root context.
// Call once from main.go. Workers stop when ctx is cancelled.
func (c *GlobalLinkChecker) Start(ctx context.Context) {
	for i := 0; i < c.config.MaxWorkers; i++ {
		go func() {
			for {
				select {
				case job, ok := <-c.jobs:
					if !ok {
						return
					}
					job.resultChan <- c.checkWithRetry(ctx, job.url)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// CheckAll submits all URLs to the global worker pool and collects results.
// Duplicate URLs are deduplicated before submission.
//
// If ctx is cancelled mid-submission, CheckAll stops submitting, collects results
// for already-submitted jobs, and returns (partialMap, ctx.Err()).
// Already-submitted jobs continue running under the root context.
func (c *GlobalLinkChecker) CheckAll(
	ctx context.Context,
	urls []string,
	onChecked ...func(string),
) (map[string]CheckResult, error) {
	var progressCb func(string)
	if len(onChecked) > 0 {
		progressCb = onChecked[0]
	}

	seen := make(map[string]struct{})
	var unique []string
	for _, u := range urls {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			unique = append(unique, u)
		}
	}

	total := len(unique)
	// Buffer = total so workers never block writing results back.
	resultChan := make(chan CheckResult, total)

	submitted := 0
	interrupted := false
	for _, u := range unique {
		// Check synchronously before select: if ctx is already done, both sending to
		// jobs and <-ctx.Done() can be ready; select would pick randomly.
		if ctx.Err() != nil {
			interrupted = true
			total = submitted
			break
		}
		select {
		case c.jobs <- linkCheckJob{url: u, resultChan: resultChan}:
			submitted++
		case <-ctx.Done():
			interrupted = true
			total = submitted
		}
		if interrupted {
			break
		}
	}

	output := make(map[string]CheckResult, total)
	for i := 0; i < total; i++ {
		res := <-resultChan
		output[res.URL] = res
		if progressCb != nil {
			progressCb(res.URL)
		}
	}

	if interrupted {
		return output, ctx.Err()
	}
	return output, nil
}

func (c *GlobalLinkChecker) checkWithRetry(ctx context.Context, rawURL string) CheckResult {
	var lastResult CheckResult

	for attempt := 0; attempt <= c.config.Retries; attempt++ {
		result := c.checkOnce(ctx, rawURL)
		if result.Err == nil {
			return result
		}
		// Workers run on root ctx — abort immediately on system shutdown.
		if ctx.Err() != nil {
			return CheckResult{URL: rawURL, Status: StatusInaccessible, Err: ctx.Err()}
		}
		lastResult = result
	}

	return lastResult
}

func (c *GlobalLinkChecker) checkOnce(ctx context.Context, rawURL string) CheckResult {
	status, err := c.doRequest(ctx, http.MethodHead, rawURL)

	// HEAD not supported by server — fall back to GET.
	if err == nil && status == http.StatusMethodNotAllowed {
		status, err = c.doRequest(ctx, http.MethodGet, rawURL)
	}

	return CheckResult{
		URL:        rawURL,
		Status:     ClassifyResult(status, err),
		StatusCode: status,
		Err:        err,
	}
}

func (c *GlobalLinkChecker) doRequest(ctx context.Context, method, rawURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", checkerUserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
