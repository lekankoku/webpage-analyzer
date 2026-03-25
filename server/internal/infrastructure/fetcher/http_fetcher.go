package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	userAgent      = "Mozilla/5.0 (compatible; WebAnalyzer/1.0)"
	maxBodySize    = 10 * 1024 * 1024 // 10 MB hard cap
	defaultTimeout = 10 * time.Second
)

// Typed sentinel errors allow callers to distinguish failure modes without
// string matching.
var (
	ErrUnreachable = errors.New("host unreachable")
	ErrTimeout     = errors.New("request timed out")
	ErrInvalidURL  = errors.New("invalid URL")
)

// HTTPStatusError is returned when the server responds with a 4xx or 5xx status code.
// It can be retrieved from an error chain with errors.As.
type HTTPStatusError struct {
	Code int    // HTTP status code (e.g. 404)
	URL  string // requested URL
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d fetching %s", e.Code, e.URL)
}

// Result holds the raw HTML and the final URL (after any HTTP redirects).
type Result struct {
	HTML     string
	FinalURL *url.URL
}

// Fetcher fetches a URL and returns the raw HTML body.
type Fetcher struct {
	client *http.Client
}

// New returns a Fetcher with a 10-second timeout.
func New() *Fetcher {
	return NewWithTimeout(defaultTimeout)
}

// NewWithTimeout returns a Fetcher with a custom client timeout.
func NewWithTimeout(timeout time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: timeout},
	}
}

// Fetch fetches rawURL and returns the HTML body plus the final URL after redirects.
// It respects ctx cancellation and caps the response body at 10 MB.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidURL, rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return nil, fmt.Errorf("%w: %w", ErrTimeout, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	// Treat 4xx/5xx as errors — callers can inspect HTTPStatusError for the code.
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, &HTTPStatusError{Code: resp.StatusCode, URL: rawURL}
	}

	limited := io.LimitReader(resp.Body, maxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %w", ErrUnreachable, err)
	}

	// Warn if the body was truncated at the cap.
	if int64(len(body)) == maxBodySize {
		log.Printf("fetcher: response body truncated at %d bytes for %s", maxBodySize, rawURL)
	}

	return &Result{
		HTML:     string(body),
		FinalURL: resp.Request.URL,
	}, nil
}

// isTimeoutError checks whether an error chain contains a net.Error that is a timeout.
func isTimeoutError(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}
