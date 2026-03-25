package linkchecker_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"web-analyzer/internal/infrastructure/linkchecker"
)

// ── 9a: ClassifyResult (pure function) ───────────────────────────────────────

func TestClassifyResult_200_Accessible(t *testing.T) {
	if got := linkchecker.ClassifyResult(200, nil); got != linkchecker.StatusAccessible {
		t.Errorf("got %q, want %q", got, linkchecker.StatusAccessible)
	}
}

func TestClassifyResult_399_Accessible(t *testing.T) {
	if got := linkchecker.ClassifyResult(399, nil); got != linkchecker.StatusAccessible {
		t.Errorf("got %q, want %q", got, linkchecker.StatusAccessible)
	}
}

func TestClassifyResult_404_Inaccessible(t *testing.T) {
	if got := linkchecker.ClassifyResult(404, nil); got != linkchecker.StatusInaccessible {
		t.Errorf("got %q, want %q", got, linkchecker.StatusInaccessible)
	}
}

func TestClassifyResult_500_Inaccessible(t *testing.T) {
	if got := linkchecker.ClassifyResult(500, nil); got != linkchecker.StatusInaccessible {
		t.Errorf("got %q, want %q", got, linkchecker.StatusInaccessible)
	}
}

func TestClassifyResult_NetworkError_Inaccessible(t *testing.T) {
	if got := linkchecker.ClassifyResult(0, errors.New("timeout")); got != linkchecker.StatusInaccessible {
		t.Errorf("got %q, want %q", got, linkchecker.StatusInaccessible)
	}
}

func TestClassifyResult_429_Unverified(t *testing.T) {
	if got := linkchecker.ClassifyResult(429, nil); got != linkchecker.StatusUnverified {
		t.Errorf("got %q, want %q", got, linkchecker.StatusUnverified)
	}
}

func TestClassifyResult_401_Unverified(t *testing.T) {
	if got := linkchecker.ClassifyResult(401, nil); got != linkchecker.StatusUnverified {
		t.Errorf("got %q, want %q", got, linkchecker.StatusUnverified)
	}
}

func TestClassifyResult_403_Unverified(t *testing.T) {
	if got := linkchecker.ClassifyResult(403, nil); got != linkchecker.StatusUnverified {
		t.Errorf("got %q, want %q", got, linkchecker.StatusUnverified)
	}
}

// ── 9b: Retry logic ──────────────────────────────────────────────────────────

// failingTransport errors for the first `failN` calls then delegates to inner.
type failingTransport struct {
	failN    int
	attempts atomic.Int32
	inner    http.RoundTripper
}

func (ft *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	n := int(ft.attempts.Add(1))
	if n <= ft.failN {
		return nil, errors.New("simulated network error")
	}
	return ft.inner.RoundTrip(req)
}

func newTestChecker(t *testing.T, transport http.RoundTripper, retries int) *linkchecker.GlobalLinkChecker {
	t.Helper()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	cfg := linkchecker.CheckerConfig{
		MaxWorkers:    5,
		JobBufferSize: 50,
		Timeout:       5 * time.Second,
		Retries:       retries,
	}
	return linkchecker.NewWithClient(cfg, client)
}

func TestRetry_NetworkError_SucceedsOnSecondAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ft := &failingTransport{failN: 1, inner: http.DefaultTransport}
	checker := newTestChecker(t, ft, 2)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, err := checker.CheckAll(context.Background(), []string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res := results[srv.URL]
	if res.Status != linkchecker.StatusAccessible {
		t.Errorf("expected accessible after retry, got %q (err: %v)", res.Status, res.Err)
	}
	// failN=1 means 1 failure + 1 success = 2 total attempts
	if got := int(ft.attempts.Load()); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestRetry_404_NoRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 2)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, err := checker.CheckAll(context.Background(), []string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[srv.URL].Status != linkchecker.StatusInaccessible {
		t.Errorf("expected inaccessible for 404")
	}
	// 404 has Err==nil so no retry; HEAD is called once.
	if got := int(calls.Load()); got != 1 {
		t.Errorf("expected 1 call (no retry on 404), got %d", got)
	}
}

func TestRetry_StopsAfterMaxRetries(t *testing.T) {
	const retries = 2
	ft := &failingTransport{failN: retries + 10, inner: http.DefaultTransport}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, ft, retries)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, _ := checker.CheckAll(context.Background(), []string{srv.URL})
	if results[srv.URL].Status != linkchecker.StatusInaccessible {
		t.Error("expected inaccessible after exhausting retries")
	}
	// attempt count = retries + 1 (initial + retries)
	if got := int(ft.attempts.Load()); got != retries+1 {
		t.Errorf("expected %d attempts, got %d", retries+1, got)
	}
}

// ── 9c: HEAD→GET fallback ─────────────────────────────────────────────────────

func TestHEAD405_FallsBackToGET(t *testing.T) {
	var methods []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, err := checker.CheckAll(context.Background(), []string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[srv.URL].Status != linkchecker.StatusAccessible {
		t.Errorf("expected accessible after GET fallback, got %q", results[srv.URL].Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 2 || methods[0] != "HEAD" || methods[1] != "GET" {
		t.Errorf("expected HEAD then GET, got %v", methods)
	}
}

func TestHEAD200_DoesNotCallGET(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodHead {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	checker.CheckAll(context.Background(), []string{srv.URL}) //nolint:errcheck
	if got := int(calls.Load()); got != 1 {
		t.Errorf("expected exactly 1 call (HEAD only), got %d", got)
	}
}

func TestUserAgent_SetOnEveryRequest(t *testing.T) {
	var agents []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.Header.Get("User-Agent"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	checker.CheckAll(context.Background(), []string{srv.URL}) //nolint:errcheck
	mu.Lock()
	defer mu.Unlock()
	for i, ua := range agents {
		if ua == "" {
			t.Errorf("request %d: User-Agent header missing", i)
		}
	}
}

// ── 9d: Global worker pool ───────────────────────────────────────────────────

func TestCheckAll_Returns5Results(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, 5)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/path%d", srv.URL, i)
	}

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, err := checker.CheckAll(context.Background(), urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestCheckAll_DeduplicatesURLs(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	results, err := checker.CheckAll(context.Background(), []string{srv.URL, srv.URL, srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 deduplicated result, got %d", len(results))
	}
	if got := int(calls.Load()); got != 1 {
		t.Errorf("expected 1 HTTP call after dedup, got %d", got)
	}
}

func TestCheckAll_NoCrossContamination(t *testing.T) {
	// Two concurrent CheckAll calls must receive only their own results.
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()

	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srvB.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	var wg sync.WaitGroup
	wg.Add(2)

	var resA, resB map[string]linkchecker.CheckResult

	go func() {
		defer wg.Done()
		resA, _ = checker.CheckAll(context.Background(), []string{srvA.URL + "/a"})
	}()
	go func() {
		defer wg.Done()
		resB, _ = checker.CheckAll(context.Background(), []string{srvB.URL + "/b"})
	}()

	wg.Wait()

	if len(resA) != 1 {
		t.Errorf("call A: expected 1 result, got %d", len(resA))
	}
	if len(resB) != 1 {
		t.Errorf("call B: expected 1 result, got %d", len(resB))
	}
	for url, r := range resA {
		if r.Status != linkchecker.StatusAccessible {
			t.Errorf("call A: %s should be accessible, got %q", url, r.Status)
		}
	}
	for url, r := range resB {
		if r.Status != linkchecker.StatusInaccessible {
			t.Errorf("call B: %s should be inaccessible (404), got %q", url, r.Status)
		}
	}
}

// ── 9e: Context cancellation ─────────────────────────────────────────────────

func TestCheckAll_CancelledBeforeSubmission(t *testing.T) {
	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(rootCtx)

	reqCtx, reqCancel := context.WithCancel(context.Background())
	reqCancel() // cancel immediately

	results, err := checker.CheckAll(reqCtx, []string{"http://example.com/1", "http://example.com/2"})
	if err == nil {
		t.Error("expected context error, got nil")
	}
	// Partial map: 0 or more results depending on race between cancel and first submit.
	_ = results
}

func TestCheckAll_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := newTestChecker(t, http.DefaultTransport, 0)
	rootCtx, cancel := context.WithCancel(context.Background())
	checker.Start(rootCtx)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer reqCancel()

	checker.CheckAll(reqCtx, []string{srv.URL + "/a", srv.URL + "/b"}) //nolint:errcheck

	// Stop worker pool — goleak.IgnoreCurrent already snapshots workers before Start,
	// so cancelling root ctx (stopping workers) makes VerifyNone clean.
	cancel()
	// Give workers time to drain.
	time.Sleep(100 * time.Millisecond)
}
