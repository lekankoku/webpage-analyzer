package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"web-analyzer/internal/application"
	infrafetcher "web-analyzer/internal/infrastructure/fetcher"
	"web-analyzer/internal/infrastructure/linkchecker"
	infraparser "web-analyzer/internal/infrastructure/parser"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// fetcherAdapter adapts infrastructure/fetcher to application.Fetcher.
type fetcherAdapter struct{ f *infrafetcher.Fetcher }

func (a *fetcherAdapter) Fetch(ctx context.Context, rawURL string) (*application.FetchResult, error) {
	r, err := a.f.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return &application.FetchResult{HTML: r.HTML, FinalURL: r.FinalURL}, nil
}

// checkerConfig carries optional per-test checker tuning.
type checkerCfg struct {
	maxWorkers    int
	jobBufferSize int
	client        *http.Client
}

func defaultCheckerCfg() checkerCfg {
	return checkerCfg{maxWorkers: 10, jobBufferSize: 50, client: nil}
}

// newPipeline builds a real AnalyzePageUseCase wired to infrastructure implementations.
// The caller must cancel rootCancel when done to stop workers.
func newPipeline(t *testing.T, cfg checkerCfg) (*application.AnalyzePageUseCase, context.CancelFunc) {
	t.Helper()
	if cfg.maxWorkers == 0 {
		cfg.maxWorkers = 10
	}
	if cfg.jobBufferSize < 0 {
		cfg.jobBufferSize = 0 // unbuffered
	} else if cfg.jobBufferSize == 0 {
		cfg.jobBufferSize = 50
	}
	httpClient := cfg.client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	rootCtx, cancel := context.WithCancel(context.Background())

	c := linkchecker.NewWithClient(linkchecker.CheckerConfig{
		MaxWorkers:    cfg.maxWorkers,
		JobBufferSize: cfg.jobBufferSize,
		Timeout:       5 * time.Second,
		Retries:       0,
	}, httpClient)
	c.Start(rootCtx)

	uc := &application.AnalyzePageUseCase{
		Fetcher: &fetcherAdapter{f: infrafetcher.New()},
		Parser:  infraparser.New(),
		Checker: c,
	}
	t.Cleanup(cancel)
	return uc, cancel
}

// runPipeline runs Execute and returns all collected SSE events.
func runPipeline(ctx context.Context, uc *application.AnalyzePageUseCase, rawURL string) []application.SSEEvent {
	var (
		events []application.SSEEvent
		mu     sync.Mutex
	)
	uc.Execute(ctx, "test-job", rawURL, func(e application.SSEEvent) { //nolint:errcheck
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	return events
}

// findEvent returns the first event with the given type, or nil.
func findEvent(events []application.SSEEvent, typ string) *application.SSEEvent {
	for i := range events {
		if events[i].Type == typ {
			return &events[i]
		}
	}
	return nil
}

// decodeResult JSON-round-trips event.Data into a map for easy field access.
func decodeResult(t *testing.T, event *application.SSEEvent) map[string]any {
	t.Helper()
	b, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("marshal result data: %v", err)
	}
	var m map[string]any
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal result data: %v", err)
	}
	return m
}

func mockServer(html string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		fmt.Fprint(w, html)
	}))
}

// ── Integration Test 1: full pipeline ────────────────────────────────────────

func TestIntegration_FullPipeline(t *testing.T) {
	linkSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer linkSrv.Close()

	// Build page HTML with internal and external links.
	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Full Pipeline Test</title></head>
<body>
<h1>Main</h1>
<h2>Sub One</h2>
<h2>Sub Two</h2>
<a href="/internal-1">i1</a>
<a href="/internal-2">i2</a>
<a href="%s/external-1">e1</a>
</body>
</html>`, linkSrv.URL)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, pageSrv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatalf("no result event emitted; events=%v", events)
	}

	d := decodeResult(t, result)

	if got := d["html_version"]; got != "HTML5" {
		t.Errorf("html_version: got %q, want %q", got, "HTML5")
	}
	if got := d["title"]; got != "Full Pipeline Test" {
		t.Errorf("title: got %q, want %q", got, "Full Pipeline Test")
	}

	headings, _ := d["headings"].(map[string]any)
	if headings["h1"] != float64(1) {
		t.Errorf("headings.h1: got %v, want 1", headings["h1"])
	}
	if headings["h2"] != float64(2) {
		t.Errorf("headings.h2: got %v, want 2", headings["h2"])
	}

	internal := d["internal_links"].(float64)
	external := d["external_links"].(float64)
	if internal+external != 3 {
		t.Errorf("internal(%v)+external(%v) = %v, want 3", internal, external, internal+external)
	}

	if partial, _ := d["partial"].(bool); partial {
		t.Error("expected partial=false")
	}
	if findEvent(events, "error") != nil {
		t.Error("unexpected error event")
	}
}

// ── Integration Test 2: unreachable URL ──────────────────────────────────────

func TestIntegration_UnreachableURL(t *testing.T) {
	// Port 1 is reserved/unreachable on all OSes.
	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, "http://127.0.0.1:1/page")

	errEvent := findEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error event for unreachable URL")
	}
	if findEvent(events, "result") != nil {
		t.Error("expected no result event when fetch fails")
	}

	// Error event must include job_id.
	d := decodeResult(t, errEvent)
	if d["job_id"] == "" {
		t.Error("error event missing job_id")
	}
}

// ── Integration Test 3: page with no links ───────────────────────────────────

func TestIntegration_NoLinks(t *testing.T) {
	srv := mockServer(`<!DOCTYPE html><html><head><title>Empty</title></head><body><p>No links</p></body></html>`, http.StatusOK)
	defer srv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, srv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	if d["internal_links"].(float64) != 0 {
		t.Errorf("expected internal_links=0, got %v", d["internal_links"])
	}
	if d["external_links"].(float64) != 0 {
		t.Errorf("expected external_links=0, got %v", d["external_links"])
	}
	if d["inaccessible_links"].(float64) != 0 {
		t.Errorf("expected inaccessible_links=0, got %v", d["inaccessible_links"])
	}
}

// ── Integration Test 4: page with login form ─────────────────────────────────

func TestIntegration_LoginForm(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
<form action="/login"><input type="password" name="pass"><button>Login</button></form>
</body></html>`
	srv := mockServer(html, http.StatusOK)
	defer srv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, srv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	if got, _ := d["has_login_form"].(bool); !got {
		t.Error("expected has_login_form=true")
	}
}

// ── Integration Test 5: context cancellation mid link-check ──────────────────

func TestIntegration_ContextCancellation_PartialResult(t *testing.T) {
	// Goroutine-leak coverage for the link checker is in linkchecker/checker_test.go.
	// Using goleak here is unreliable: HTTP keep-alive connections and httptest server
	// goroutines outlive their Close() calls at the point of the check.

	// Slow link server — each request sleeps 150ms.
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowSrv.Close()

	// 5 links all pointing to the slow server.
	links := ""
	for i := 0; i < 5; i++ {
		links += fmt.Sprintf(`<a href="%s/link%d">L%d</a>`, slowSrv.URL, i, i)
	}
	pageHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><title>Cancel</title></head><body>%s</body></html>`, links)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	// Use an unbuffered jobs channel (buffer=−1 → 0) and 1 worker so the 2nd
	// job submission blocks immediately after the 1st is picked up by the worker.
	uc, rootCancel := newPipeline(t, checkerCfg{
		maxWorkers:    1,
		jobBufferSize: -1, // unbuffered — forces blocking on 2nd submit
	})

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer reqCancel()

	events := runPipeline(reqCtx, uc, pageSrv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event even on partial completion")
	}
	d := decodeResult(t, result)
	if partial, _ := d["partial"].(bool); !partial {
		t.Error("expected partial=true after context cancellation")
	}

	rootCancel() // stop workers (also called by t.Cleanup, harmless to call early)
}

// ── Edge cases ────────────────────────────────────────────────────────────────

func TestEdge_BaseTag_LinksResolvedAgainstBase(t *testing.T) {
	var receivedPath string
	var mu sync.Mutex
	baseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer baseSrv.Close()

	// Page has a <base href="baseSrv.URL"> so relative /foo resolves to baseSrv.URL/foo.
	// Since both servers are on 127.0.0.1, classification is "internal" (same host),
	// but the KEY assertion is that the link RESOLVES to baseSrv.URL/foo (not pageSrv/foo).
	pageHTML := fmt.Sprintf(`<!DOCTYPE html><html><head>
<base href="%s">
<title>Base Test</title></head>
<body><a href="/base-resolved-path">link</a></body>
</html>`, baseSrv.URL)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	runPipeline(context.Background(), uc, pageSrv.URL)

	// The link checker should have requested /base-resolved-path from baseSrv,
	// proving that the <base> tag was respected for link resolution.
	mu.Lock()
	got := receivedPath
	mu.Unlock()
	if got != "/base-resolved-path" {
		t.Errorf("expected link checker to hit baseSrv at /base-resolved-path, got %q", got)
	}
}

func TestEdge_ZeroHeadings_EmptyMap(t *testing.T) {
	srv := mockServer(`<!DOCTYPE html><html><head><title>No Headings</title></head><body><p>text</p></body></html>`, http.StatusOK)
	defer srv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, srv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	// Marshal and unmarshal to verify headings is {} not null.
	b, _ := json.Marshal(result.Data)
	if strings.Contains(string(b), `"headings":null`) {
		t.Error("headings should be {} not null")
	}
	if !strings.Contains(string(b), `"headings":{}`) {
		t.Errorf("expected headings:{} in JSON, got: %s", b)
	}
}

func TestEdge_FragmentOnlyLinks_NotCounted(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Fragments</title></head><body>
<a href="#section1">jump1</a>
<a href="#section2">jump2</a>
</body></html>`
	srv := mockServer(html, http.StatusOK)
	defer srv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, srv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	if d["internal_links"].(float64) != 0 {
		t.Errorf("fragment-only links should not count: internal_links=%v", d["internal_links"])
	}
	if d["external_links"].(float64) != 0 {
		t.Errorf("fragment-only links should not count: external_links=%v", d["external_links"])
	}
}

func TestEdge_DuplicateLinks_CheckedOnce(t *testing.T) {
	var calls int
	var mu sync.Mutex
	linkSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer linkSrv.Close()

	// 3 copies of the same link.
	pageHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><title>Dupes</title></head><body>
<a href="%s/same">1</a><a href="%s/same">2</a><a href="%s/same">3</a>
</body></html>`, linkSrv.URL, linkSrv.URL, linkSrv.URL)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, pageSrv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	// All 3 occurrences are counted (raw, not deduplicated). Both servers are on 127.0.0.1
	// so they're classified as internal; the key test is raw count + checker dedup.
	totalLinks := d["internal_links"].(float64) + d["external_links"].(float64)
	if totalLinks != 3 {
		t.Errorf("expected 3 total links (raw count), got %v", totalLinks)
	}

	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Errorf("link checker should check each URL once (dedup), got %d calls", gotCalls)
	}
}

func TestEdge_429Link_CountedAsUnverified(t *testing.T) {
	linkSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer linkSrv.Close()

	pageHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><title>T</title></head><body>
<a href="%s/rate-limited">link</a>
</body></html>`, linkSrv.URL)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, pageSrv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	if d["unverified_links"].(float64) != 1 {
		t.Errorf("expected unverified_links=1 for 429, got %v", d["unverified_links"])
	}
	if d["inaccessible_links"].(float64) != 0 {
		t.Errorf("expected inaccessible_links=0 for 429, got %v", d["inaccessible_links"])
	}
}

func TestEdge_401Link_CountedAsUnverified(t *testing.T) {
	linkSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer linkSrv.Close()

	pageHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><title>T</title></head><body>
<a href="%s/auth-required">link</a>
</body></html>`, linkSrv.URL)

	pageSrv := mockServer(pageHTML, http.StatusOK)
	defer pageSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	events := runPipeline(context.Background(), uc, pageSrv.URL)

	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event")
	}
	d := decodeResult(t, result)
	if d["unverified_links"].(float64) != 1 {
		t.Errorf("expected unverified_links=1 for 401, got %v", d["unverified_links"])
	}
}

func TestEdge_LargeResponseBody_TruncatedNoPanic(t *testing.T) {
	// Serve a page whose body exceeds the 10MB cap.
	bigSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Big</title></head><body>`)
		// Push well past the 10 MB limit.
		chunk := strings.Repeat("x", 512*1024) // 512 KB chunks
		for i := 0; i < 22; i++ {               // 22 * 512 KB ≈ 11 MB
			fmt.Fprint(w, chunk)
		}
		fmt.Fprint(w, `</body></html>`)
	}))
	defer bigSrv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	// Should complete without panic; title may be parsed from the truncated body.
	events := runPipeline(context.Background(), uc, bigSrv.URL)

	if findEvent(events, "error") != nil {
		t.Error("expected no error event for truncated body (truncation is handled gracefully)")
	}
	result := findEvent(events, "result")
	if result == nil {
		t.Fatal("expected result event even for truncated body")
	}
}

func TestEdge_UserAgent_SetOnEveryRequest(t *testing.T) {
	var userAgents []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>UA</title></head><body>
<a href="/check">link</a></body></html>`)
	}))
	defer srv.Close()

	uc, _ := newPipeline(t, defaultCheckerCfg())
	runPipeline(context.Background(), uc, srv.URL)

	mu.Lock()
	defer mu.Unlock()
	for i, ua := range userAgents {
		if ua == "" {
			t.Errorf("request %d: User-Agent header missing", i)
		}
	}
	if len(userAgents) == 0 {
		t.Error("no requests recorded")
	}
}
