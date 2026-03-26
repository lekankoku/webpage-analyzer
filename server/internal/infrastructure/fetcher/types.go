package fetcher

import (
	"errors"
	"fmt"
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
