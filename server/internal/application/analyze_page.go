package application

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"

	"web-analyzer/internal/domain/model"
	"web-analyzer/internal/domain/service"
	"web-analyzer/internal/infrastructure/linkchecker"
)

// ── Ports ─────────────────────────────────────────────────────────────────────

// FetchResult is the application-layer representation of a successful page fetch.
type FetchResult struct {
	HTML     string
	FinalURL *url.URL
}

// Fetcher is the port for fetching a remote URL.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*FetchResult, error)
}

// Parser is the port for parsing raw HTML into the WebPage domain model.
type Parser interface {
	Parse(rawHTML string, pageURL *url.URL) (*model.WebPage, error)
}

// LinkChecker is the port for concurrently checking reachability of a URL batch.
// DDD-lite: CheckResult lives in infrastructure/linkchecker to avoid duplication.
type LinkChecker interface {
	CheckAll(ctx context.Context, urls []string) (map[string]linkchecker.CheckResult, error)
}

// ── Use case ──────────────────────────────────────────────────────────────────

// AnalyzePageUseCase orchestrates the full page analysis pipeline.
// Infrastructure owns concurrency; this layer owns event emission and progress.
type AnalyzePageUseCase struct {
	Fetcher Fetcher
	Parser  Parser
	Checker LinkChecker
}

// Execute runs the analysis pipeline for rawURL, emitting SSE events via emit.
// jobID is threaded through for log correlation and included in error events.
func (uc *AnalyzePageUseCase) Execute(
	ctx context.Context,
	jobID string,
	rawURL string,
	emit func(SSEEvent),
) error {
	// 1. Validate URL.
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		msg := fmt.Sprintf("invalid URL: %s", rawURL)
		emitError(emit, msg, jobID)
		return fmt.Errorf("%s", msg)
	}

	// 2. Fetch.
	log.Printf("[%s] phase=fetching url=%s", jobID, rawURL)
	emit(SSEEvent{Type: "phase", Data: map[string]string{
		"phase": "fetching", "message": "Fetching page...",
	}})

	fetchResult, err := uc.Fetcher.Fetch(ctx, rawURL)
	if err != nil {
		emitError(emit, fmt.Sprintf("could not fetch %s: %v", rawURL, err), jobID)
		return err
	}

	// 3. Parse.
	log.Printf("[%s] phase=parsing", jobID)
	emit(SSEEvent{Type: "phase", Data: map[string]string{
		"phase": "parsing", "message": "Parsing HTML...",
	}})

	page, err := uc.Parser.Parse(fetchResult.HTML, fetchResult.FinalURL)
	if err != nil {
		emitError(emit, fmt.Sprintf("could not parse HTML: %v", err), jobID)
		return err
	}

	// Ensure Headings is never nil in the final result.
	if page.Headings == nil {
		page.Headings = make(map[string]int)
	}

	// 4. Normalize links and classify as internal/external.
	// InternalLinks/ExternalLinks count all (non-fragment) links in the HTML.
	// Invalid URLs are counted as inaccessible (not submitted to the checker).
	var normalizedURLs []string
	internalCount, externalCount, invalidCount := 0, 0, 0

	for _, link := range page.Links {
		norm, skip, normErr := service.NormalizeURL(link.Raw, page.BaseURL)
		if skip {
			continue
		}
		if normErr != nil {
			invalidCount++
			continue
		}
		parsedLink, parseErr := url.Parse(norm)
		if parseErr != nil {
			invalidCount++
			continue
		}
		// Classify against the actual page URL, not the <base> tag URL.
		// The <base> tag only affects resolution, not host identity.
		if service.ClassifyLink(parsedLink, fetchResult.FinalURL) {
			internalCount++
		} else {
			externalCount++
		}
		normalizedURLs = append(normalizedURLs, norm)
	}

	// 5. Login detection (requires a goquery document).
	doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(fetchResult.HTML))
	hasLogin := false
	if docErr == nil {
		hasLogin = service.DetectLoginForm(doc)
	}

	// 6. HTML version detection (pure string inspection).
	htmlVersion := service.DetectHTMLVersion(fetchResult.HTML)

	// 7. Link checking with 500ms progress ticker.
	total := len(normalizedURLs)
	log.Printf("[%s] phase=checking_links total=%d", jobID, total)
	emit(SSEEvent{Type: "phase", Data: map[string]any{
		"phase": "checking_links", "message": "Checking links...", "total": total,
	}})

	var checkedSoFar atomic.Int64
	progressDone := make(chan struct{})

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				emit(SSEEvent{Type: "progress", Data: map[string]any{
					"checked": checkedSoFar.Load(),
					"total":   total,
				}})
			case <-progressDone:
				return
			}
		}
	}()

	checkerResults, checkErr := uc.Checker.CheckAll(ctx, normalizedURLs)
	close(progressDone)
	checkedSoFar.Store(int64(len(checkerResults)))

	// 8. Aggregate accessibility counts.
	// Start inaccessible at invalidCount (links that couldn't be normalised/checked).
	inaccessible := invalidCount
	unverified := 0
	for _, res := range checkerResults {
		switch res.Status {
		case linkchecker.StatusInaccessible:
			inaccessible++
		case linkchecker.StatusUnverified:
			unverified++
		}
	}

	result := &model.AnalysisResult{
		HTMLVersion:       htmlVersion,
		Title:             page.Title,
		Headings:          page.Headings,
		InternalLinks:     internalCount,
		ExternalLinks:     externalCount,
		InaccessibleLinks: inaccessible,
		UnverifiedLinks:   unverified,
		HasLoginForm:      hasLogin,
		Partial:           checkErr != nil,
	}

	log.Printf("[%s] done partial=%v inaccessible=%d unverified=%d",
		jobID, result.Partial, inaccessible, unverified)
	emit(SSEEvent{Type: "result", Data: result})
	return nil
}

func emitError(emit func(SSEEvent), message, jobID string) {
	emit(SSEEvent{Type: "error", Data: map[string]string{
		"message": message,
		"job_id":  jobID,
	}})
}
