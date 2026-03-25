# Web Page Analyzer — Master Design & LLM Build Plan

> Go + DDD-lite + TDD + SSE  
> Status: Ready for iterative LLM implementation

---

## 0. How to Use This Document

This document bridges two design specs into a single source of truth, then breaks the
implementation into discrete, ordered steps for an LLM to execute iteratively.

**Rules for the LLM executing this plan:**
- Complete one step fully before starting the next
- Run tests after every step — do not proceed on red tests
- Never skip the TDD sequence within a step
- Flag ambiguities before writing code, not after
- Do not invent requirements not stated here

---

## 1. Final Architecture Overview

```
Client (Next.js — out of scope)
        │
        ├── POST /analyze          → { jobID }
        └── GET  /analyze/stream?id=<jobID>  → SSE stream
                        │
              ┌─────────▼──────────┐
              │   HTTP Interface   │  (interfaces/http/)
              └─────────┬──────────┘
                        │
              ┌─────────▼──────────┐
              │  Application Layer │  (application/analyze_page.go)
              │  - orchestrates    │
              │  - emits SSE events│
              └──┬──────┬──────────┘
                 │      │
        ┌────────▼─┐  ┌─▼────────────┐
        │ Fetcher  │  │   Parser     │  (infrastructure/)
        └────────┬─┘  └─┬────────────┘
                 │      │
              ┌──▼──────▼──────────┐
              │   Domain Services  │  (domain/service/)
              │  - LinkClassifier  │
              │  - LoginDetector   │
              │  - Analyzer        │
              └──────────┬─────────┘
                         │
              ┌──────────▼─────────┐
              │   LinkChecker      │  (infrastructure/linkchecker/)
              │   Worker Pool      │
              │   HEAD→GET+Retry   │
              └──────────┬─────────┘
                         │
              ┌──────────▼─────────┐
              │  SSE Event Stream  │  (emitted back up through app layer)
              └────────────────────┘
```

---

## 2. Project Structure (Final)

```
web-analyzer/
├── cmd/app/main.go
├── internal/
│   ├── domain/
│   │   ├── model/
│   │   │   ├── link.go
│   │   │   ├── webpage.go
│   │   │   └── result.go
│   │   └── service/
│   │       ├── analyzer.go
│   │       ├── link_classifier.go
│   │       └── login_detector.go
│   ├── application/
│   │   └── analyze_page.go
│   ├── infrastructure/
│   │   ├── fetcher/
│   │   │   └── http_fetcher.go
│   │   ├── parser/
│   │   │   └── html_parser.go
│   │   └── linkchecker/
│   │       ├── checker.go
│   │       └── checker_test.go
│   └── interfaces/
│       └── http/
│           ├── handler.go
│           └── sse.go
├── go.mod
└── README.md
```

---

## 3. Domain Models (Locked)

```go
// domain/model/link.go
type Link struct {
    Raw        string
    Resolved   *url.URL
    IsInternal bool
}

// domain/model/webpage.go
type WebPage struct {
    BaseURL  *url.URL
    HTML     string
    Title    string
    Headings map[string]int  // "h1" → count
    Links    []Link
}

// domain/model/result.go
type AnalysisResult struct {
    HTMLVersion       string         `json:"html_version"`
    Title             string         `json:"title"`
    Headings          map[string]int `json:"headings"`
    InternalLinks     int            `json:"internal_links"`
    ExternalLinks     int            `json:"external_links"`
    InaccessibleLinks int            `json:"inaccessible_links"`
    UnverifiedLinks   int            `json:"unverified_links"` // 429, 401, 403 — server refused check, link not confirmed broken
    HasLoginForm      bool           `json:"has_login_form"`
    Partial           bool           `json:"partial,omitempty"` // true if interrupted mid link-check (context cancelled)
}
```

---

## 4. SSE Event Schema (Locked)

All events are newline-delimited JSON sent as SSE `data:` fields.

### Event Types

```
event: phase
data: {"phase": "fetching", "message": "Fetching page..."}

event: phase
data: {"phase": "parsing", "message": "Parsing HTML..."}

event: phase
data: {"phase": "checking_links", "message": "Checking links..."}

event: progress
data: {"checked": 47, "total": 200}

event: result
data: { ...AnalysisResult }

event: error
data: {"message": "Could not reach host: example.com", "job_id": "550e8400-..."}
```

### Phase Sequence
1. `fetching`
2. `parsing`
3. `checking_links` (with interleaved `progress` events)
4. `result` (terminal — close stream after sending)
5. `error` (terminal — close stream after sending, replaces `result`)

---

## 5. API Contract (Locked)

### POST /analyze
**Request:**
```json
{ "url": "https://example.com" }
```

**Response 202:**
```json
{ "job_id": "uuid-v4" }
```

**Response 400:**
```json
{ "error": "invalid URL" }
```

### GET /analyze/stream?id=<jobID>

**Headers set by server:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Notes:**
- If `jobID` unknown → 404
- If job already completed → immediately emit `result` and close
- Stream closes after terminal event (`result` or `error`)

---

## 6. Application Layer Contract (analyze_page.go)

This is the orchestrator. It must:

1. Receive a URL string and a send function: `func(event SSEEvent)`
2. Emit `phase: fetching` → call Fetcher
3. Emit `phase: parsing` → call Parser
4. Emit `phase: checking_links` → call LinkChecker, forwarding progress
5. Aggregate domain results
6. Emit `result` or `error`

```go
type AnalyzePageUseCase struct {
    Fetcher     Fetcher
    Parser      Parser
    Checker     LinkChecker
    Classifier  LinkClassifier
    Detector    LoginDetector
}

func (uc *AnalyzePageUseCase) Execute(
    ctx context.Context,
    rawURL string,
    emit func(SSEEvent),
) error
```

**The application layer must NOT:**
- Know about HTTP request/response types
- Depend on `net/http` directly
- Own concurrency — that belongs to LinkChecker

---

## 7. LinkChecker Design (Locked + Bug-Fixed)

### Interface
```go
type LinkChecker interface {
    // progress is NOT on the interface — it belongs to the application layer.
    // The application layer wraps CheckAll and emits SSE progress events itself.
    // Keeping the interface minimal makes mocking clean and enforces separation.
    CheckAll(ctx context.Context, urls []string) (map[string]CheckResult, error)
}

// LinkStatus replaces the old Accessible bool — three outcomes, not two.
type LinkStatus string

const (
    StatusAccessible   LinkStatus = "accessible"
    StatusInaccessible LinkStatus = "inaccessible" // network error, 4xx (excl. 401/403/429), 5xx
    StatusUnverified   LinkStatus = "unverified"   // 429, 401, 403 — server refused, not confirmed broken
)

type CheckResult struct {
    URL        string
    Status     LinkStatus // replaces Accessible bool
    StatusCode int
    Err        error
}

type CheckerConfig struct {
    MaxWorkers int
    Timeout    time.Duration
    Retries    int
}
```

### Link Classification Rule
```go
// ClassifyResult replaces IsAccessible — pure function, fully testable.
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
```

### Worker Pool (Global — replaces per-request pool)

Workers are started **once at application boot** and shared across all concurrent
analysis jobs. `CheckAll` is a producer — it submits jobs to the shared channel and
collects only its own results via a per-call result channel.

**Why per-call result channels?**
With multiple jobs submitting to the same worker pool simultaneously, each `CheckAll`
call passes its own `resultChan` inside each job struct. Workers write results back
to that channel directly — no routing table, no shared result state.

```go
type CheckerConfig struct {
    MaxWorkers    int           // global pool size, e.g. 100
    JobBufferSize int           // jobs channel buffer, e.g. 500
    Timeout       time.Duration
    Retries       int
}

type linkCheckJob struct {
    url        string
    resultChan chan CheckResult // owned by the CheckAll call that submitted this job
}

type GlobalLinkChecker struct {
    jobs   chan linkCheckJob
    client *http.Client
    config CheckerConfig
}

// Start launches workers against a root context. Call once from main.go.
// Workers run until ctx is cancelled (graceful shutdown).
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

func (c *GlobalLinkChecker) CheckAll(
    ctx context.Context,
    urls []string,
) (map[string]CheckResult, error) {

    // 1. Deduplicate
    seen := make(map[string]struct{})
    var unique []string
    for _, u := range urls {
        if _, ok := seen[u]; !ok {
            seen[u] = struct{}{}
            unique = append(unique, u)
        }
    }

    total := len(unique)
    // Buffer = total so workers never block writing results back
    resultChan := make(chan CheckResult, total)

    // 2. Submit jobs — stop if request context cancelled
    // NOTE: jobs already submitted will still be executed by workers and written
    // to resultChan. Since nobody reads after ctx cancel, resultChan is GC'd
    // once the submitted jobs drain. This is intentional, not a leak.
    submitted := 0
    interrupted := false
    for _, url := range unique {
        select {
        case c.jobs <- linkCheckJob{url: url, resultChan: resultChan}:
            submitted++
        case <-ctx.Done():
            interrupted = true
            total = submitted // only collect what was submitted
        }
    }

    // 3. Collect submitted results
    output := make(map[string]CheckResult, total)
    for i := 0; i < total; i++ {
        res := <-resultChan
        output[res.URL] = res
    }

    // Return sentinel error so application layer can set Partial = true
    if interrupted {
        return output, ctx.Err()
    }
    return output, nil
}
```

### Retry + HEAD→GET Fallback
```go
func (c *GlobalLinkChecker) checkWithRetry(ctx context.Context, rawURL string) CheckResult {
    var lastResult CheckResult

    for attempt := 0; attempt <= c.config.Retries; attempt++ {
        result := c.checkOnce(ctx, rawURL)

        if result.Err == nil {
            return result
        }

        // Workers run on root ctx — check if the whole system is shutting down
        if ctx.Err() != nil {
            return CheckResult{URL: rawURL, Status: StatusInaccessible, Err: ctx.Err()}
        }

        lastResult = result
    }

    return lastResult
}

func (c *GlobalLinkChecker) checkOnce(ctx context.Context, rawURL string) CheckResult {
    // Try HEAD first
    status, err := c.doRequest(ctx, http.MethodHead, rawURL)

    // Fallback to GET if HEAD not supported
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
```

---

## 8. URL Normalization Rules (Locked)

Applied before classification and before link checking.

1. Resolve relative URLs against `<base>` tag href if present, else against page URL
2. Strip fragments (`#section`) — fragment-only links (`#anchor`) are skipped entirely
3. Normalize scheme to lowercase
4. Normalize host to lowercase
5. Remove default ports (`:80` for http, `:443` for https)
6. Invalid URLs → marked inaccessible, not checked

---

## 9. Login Detection Rules (Locked)

A page has a login form if any `<form>` element satisfies ALL of:
- Contains an `<input type="password">`
- AND the form's text content or `action`/`id`/`class` attribute contains any of:
  `"login"`, `"sign in"`, `"signin"`, `"anmelden"`, `"log in"`
  (case-insensitive)

Fallback: if `<input type="password">` exists anywhere on the page with no matching
form context → still return `HasLoginForm: true`. Real-world pages are messy.

---

## 10. HTML Version Detection Rules (Locked)

| Doctype pattern | Detected version |
|---|---|
| `<!DOCTYPE html>` (case-insensitive) | `HTML5` |
| `HTML 4.01 Strict` in doctype | `HTML 4.01 Strict` |
| `HTML 4.01 Transitional` in doctype | `HTML 4.01 Transitional` |
| `XHTML 1.0` in doctype | `XHTML 1.0` |
| `XHTML 1.1` in doctype | `XHTML 1.1` |
| No doctype found | `Unknown` |

---

## 11. In-Memory Job Store Design

Since persistence is out of scope, jobs live in memory with a simple store:

```go
type JobStatus string

const (
    JobPending    JobStatus = "pending"
    JobRunning    JobStatus = "running"
    JobDone       JobStatus = "done"
    JobFailed     JobStatus = "failed"
)

type Job struct {
    ID        string
    Status    JobStatus
    Result    *AnalysisResult  // nil until done
    Error     string           // set on failure
    CreatedAt time.Time
}

type JobStore struct {
    mu   sync.RWMutex
    jobs map[string]*Job
}
```

**SSE subscriber pattern:**
- When `GET /analyze/stream?id=...` arrives, it registers a channel on the job
- The application layer sends events to that channel
- If job is already `done` when subscriber connects, immediately replay `result` event

```go
type JobStore struct {
    mu          sync.RWMutex
    jobs        map[string]*Job
    subscribers map[string][]chan SSEEvent  // jobID → list of subscriber channels
}
```

---

## 12. Scalability Note (README Section — Not Implemented)

> This section should appear verbatim in the project README.

### Current Architecture Limits

The current implementation uses an in-memory job store, a concurrency semaphore,
and a global shared worker pool. This provides hard resource ceilings suitable for
moderate load, with the following constraints:

- **Bounded concurrency** — max 10 concurrent analyses (semaphore) and 100 link
  checker workers (global pool). Requests beyond capacity receive a 429 immediately
  rather than degrading system resources. Explicit rejection is preferable to silent
  degradation.
- **Semaphore slots are held for job duration** — a job with 500 slow external links
  holds its slot for the full analysis. Under sustained load with slow external
  servers, all 10 slots can remain occupied for extended periods. Tune
  `MaxConcurrentJobs` and `MaxWorkers` together based on observed p95 job latency.
- **State is process-local** — horizontal scaling requires sticky sessions or a
  shared job store (e.g. Redis). Two API instances cannot share job state or SSE
  subscribers.
- **No persistence** — jobs are lost on process restart. The TTL reaper manages
  memory within a process lifetime but does not survive restarts.

### Path to Production Scale

```
Current                          →  Scaled
──────────────────────────────────────────────────────────
In-memory job store              →  Redis (job state + pub/sub for SSE)
Semaphore-gated goroutines       →  Job queue (e.g. Asynq over Redis)
Global shared worker pool        →  Distributed worker pool across nodes
SSE fan-out via channels         →  Redis pub/sub → SSE gateway
Single process                   →  Multiple API instances + shared queue workers
In-process TTL reaper            →  Redis key expiry (automatic, no reaper needed)
```

**Architecture diagram:**

```
[API Instances (stateless)]
        │
        ├── POST /analyze → push job to Redis queue → return jobID
        └── GET /analyze/stream → subscribe to Redis pub/sub channel for jobID
                        │
              [Worker Pool (separate process or goroutine pool)]
                        │
                  Pull job from queue
                        │
                  Run analysis + publish SSE events to Redis pub/sub
                        │
              [API SSE handler streams events to client]
```

This pattern decouples ingestion from processing and allows independent scaling of
API nodes and worker nodes.

---

## 13. LLM Build Steps (Iterative)

Execute these steps **in order**. Each step is self-contained with inputs, outputs,
and a definition of done.

---

### STEP 1 — Project Scaffold

**Goal:** Runnable Go module with correct folder structure, no logic yet.

**Tasks:**
1. Create `go.mod` with module name `web-analyzer`, Go 1.22+
2. Add dependencies: `github.com/PuerkitoBio/goquery`, `github.com/google/uuid`
3. Create all folders from Section 2 with `.gitkeep` files
4. Create `cmd/app/main.go` that:
   - Creates a root context wired to `os.Signal` (SIGINT/SIGTERM) for graceful shutdown
   - Initialises `GlobalLinkChecker` with config (MaxWorkers: 100, JobBufferSize: 500)
   - Calls `checker.Start(rootCtx)` to boot the global worker pool
   - Initialises the analysis semaphore (capacity 10)
   - Initialises `JobStore` and calls `store.StartReaper(rootCtx, time.Hour)`
   - Starts HTTP server on `:8080` and logs "ready"
   - On signal: cancels root context (stops workers + reaper), then shuts down HTTP server with 30s timeout
5. Verify: `go run cmd/app/main.go` starts cleanly, Ctrl+C shuts down gracefully

**Definition of done:** `go build ./...` passes with zero errors.

---

### STEP 2 — Domain Models

**Goal:** All domain models defined, no logic.

**Tasks:**
1. Implement `domain/model/link.go` — `Link` struct from Section 3
2. Implement `domain/model/webpage.go` — `WebPage` struct from Section 3
3. Implement `domain/model/result.go` — `AnalysisResult` struct with JSON tags from Section 3

**Definition of done:** `go build ./internal/domain/...` passes.

---

### STEP 3 — Domain Service: URL Normalization (TDD)

**Goal:** Pure, tested URL normalization function.

**TDD sequence — write each test first, then implement:**

```
Test 1: relative path "/about" resolved against "https://example.com" → "https://example.com/about"
Test 2: fragment stripped — "https://example.com/page#section" → "https://example.com/page"
Test 3: fragment-only "#anchor" → returns ("", skip=true)
Test 4: scheme normalized — "HTTP://EXAMPLE.COM/path" → "http://example.com/path"
Test 5: default port stripped — "https://example.com:443/path" → "https://example.com/path"
Test 6: <base> tag respected — relative "/foo" with base "https://other.com" → "https://other.com/foo"
Test 7: invalid URL → returns ("", skip=false, err)
```

**Location:** `domain/service/link_classifier.go` + `_test.go`

**Definition of done:** All 7 tests green.

---

### STEP 4 — Domain Service: Link Classification (TDD)

**Goal:** Pure, tested internal/external classification.

**TDD sequence:**
```
Test 1: same host → IsInternal = true
Test 2: different host → IsInternal = false
Test 3: subdomain of base → IsInternal = false (subdomain = external)
Test 4: www variant of same host → IsInternal = true
         (strip www. prefix before comparing)
```

**Location:** `domain/service/link_classifier.go` + `_test.go`

**Definition of done:** All 4 tests green.

---

### STEP 5 — Domain Service: Login Detection (TDD)

**Goal:** Pure, tested login form heuristic operating on parsed HTML nodes.

**TDD sequence:**
```
Test 1: form with <input type="password"> + action contains "login" → true
Test 2: form with <input type="password"> + no login signal → true (fallback rule)
Test 3: form with no password input → false
Test 4: no form at all → false
Test 5: "anmelden" in form text → true
Test 6: "sign in" in form class attr → true
```

**Location:** `domain/service/login_detector.go` + `_test.go`

**Definition of done:** All 6 tests green.

---

### STEP 6 — Domain Service: HTML Version Detection (TDD)

**Goal:** Pure, tested doctype detection.

**TDD sequence — use the table from Section 10:**
```
Test 1: "<!DOCTYPE html>" → "HTML5"
Test 2: "<!doctype HTML>" (uppercase) → "HTML5"
Test 3: HTML 4.01 Strict doctype string → "HTML 4.01 Strict"
Test 4: XHTML 1.0 doctype string → "XHTML 1.0"
Test 5: no doctype → "Unknown"
```

**Location:** `domain/service/analyzer.go` + `_test.go`

**Definition of done:** All 5 tests green.

---

### STEP 7 — Infrastructure: HTML Parser

**Goal:** Parse raw HTML into `WebPage` domain model using goquery.

**Must extract:**
- `<title>` text
- `<base href="...">` if present
- Heading counts h1–h6
- All `<a href="...">` raw values
- Raw HTML for doctype detection (first 512 bytes is sufficient)
- Full document for login detection

**Edge cases to handle:**
- Missing `<title>` → empty string
- Multiple `<base>` tags → use first
- `<a>` with no href → skip

**No tests required for this step** (integration-level, hard to unit test meaningfully).
Manual verification via a known URL is sufficient.

---

### STEP 8 — Infrastructure: HTTP Fetcher

**Goal:** Fetch a URL and return raw HTML string + resolved final URL (after redirects).

**Requirements:**
- Use `net/http` with a 10s timeout
- Follow redirects (default client behaviour)
- Return the final URL (for base resolution when no `<base>` tag)
- Return typed errors: `ErrUnreachable`, `ErrTimeout`, `ErrInvalidURL`
- Accept `context.Context` — cancel immediately on ctx cancellation
- Set a realistic `User-Agent` header on every request:
  ```go
  req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebAnalyzer/1.0)")
  ```
  Without this, many servers return 403 or serve degraded content to `Go-http-client/1.1`.
- Cap response body reads to prevent OOM on oversized pages:
  ```go
  body := io.LimitReader(resp.Body, 10*1024*1024) // 10MB hard cap
  ```
  If the body is truncated, log a warning but continue — partial HTML is still parseable.

**No unit tests required.** Integration test: fetch `https://example.com`, assert
non-empty HTML returned.

---

### STEP 9 — Infrastructure: LinkChecker (TDD)

**Goal:** Full LinkChecker from Section 7. This is the most critical step.

**TDD sequence — strictly in this order:**

**9a. Link classification rule (pure function) — replaces IsAccessible:**
```
Test: ClassifyResult(200, nil)                 → StatusAccessible
Test: ClassifyResult(399, nil)                 → StatusAccessible
Test: ClassifyResult(404, nil)                 → StatusInaccessible
Test: ClassifyResult(500, nil)                 → StatusInaccessible
Test: ClassifyResult(0, errors.New("timeout")) → StatusInaccessible
Test: ClassifyResult(429, nil)                 → StatusUnverified
Test: ClassifyResult(401, nil)                 → StatusUnverified
Test: ClassifyResult(403, nil)                 → StatusUnverified
```

**9b. Retry logic:**
```
Test: retries on network error, succeeds on 2nd attempt — assert 2 attempts made
Test: does NOT retry on 404
Test: stops after max retries — assert attempt count = retries + 1
```

**9c. HEAD→GET fallback (use mock HTTP client via httptest.NewServer):**
```
Test: HEAD returns 405 → falls back to GET → uses GET status code
Test: HEAD returns 200 → does not call GET
Test: User-Agent header is set on every request — assert header present in mock server
```

**9d. Global worker pool:**
```
Test: Start(ctx) + CheckAll with 5 URLs → returns 5 results, nil error
Test: CheckAll with duplicate URLs → deduplicates, returns 1 result per unique URL
Test: two concurrent CheckAll calls → each receives only its own results (no cross-contamination)
```

**9e. Context cancellation + partial result:**
```
Test: cancelled context before submission → CheckAll returns (partial map, ctx.Err())
Test: cancelled context → CheckAll returns promptly, no goroutine leak
      (use goleak or a timeout assertion)
```

**Location:** `infrastructure/linkchecker/checker.go` + `checker_test.go`

**Definition of done:** All tests in 9a–9e green. `go vet ./...` clean.

---

### STEP 10 — Application Layer: AnalyzePageUseCase

**Goal:** Wire all domain services and infrastructure into the orchestrator.

**Implement `application/analyze_page.go` per Section 6 contract.**

**The application layer now owns progress tracking** — `CheckAll` no longer takes
a progress callback. The use case wraps `CheckAll` and emits SSE progress events
by launching a goroutine that polls or by using a ticker. Simplest approach: run
`CheckAll` in a goroutine, emit a `progress` event on a ticker every 500ms using
the result count so far.

**jobID must be threaded through `Execute`** so error SSE events can include it:

```go
func (uc *AnalyzePageUseCase) Execute(
    ctx context.Context,
    jobID  string,   // added — for correlation in error events and logs
    rawURL string,
    emit   func(SSEEvent),
) error
```

**SSE emission sequence:**
1. Validate URL → emit `error` (with `job_id`) and return if invalid
2. Emit `phase: fetching`
3. Call `Fetcher.Fetch(ctx, url)` → emit `error` (with `job_id`) and return on failure
4. Emit `phase: parsing`
5. Call `Parser.Parse(html)` → build `WebPage`
6. Normalize all links
7. Classify links (internal/external count)
8. Detect login form
9. Detect HTML version
10. Emit `phase: checking_links` with total count
11. Call `results, err := Checker.CheckAll(ctx, normalizedURLs)`
    - Emit `progress` events on a 500ms ticker while CheckAll runs
    - On return: if `err != nil` → set `result.Partial = true`
12. Aggregate results into three buckets:
    ```go
    for _, res := range checkerResults {
        switch res.Status {
        case StatusInaccessible:
            result.InaccessibleLinks++
        case StatusUnverified:
            result.UnverifiedLinks++
        // StatusAccessible — no count needed
        }
    }
    ```
13. Emit `result` with completed `AnalysisResult` (Partial may be true)
    - Always emit `result` even if partial — client can inspect the `partial` flag

**Error events must include jobID for log correlation:**
```go
emit(SSEEvent{
    Type: "error",
    Data: map[string]string{
        "message": err.Error(),
        "job_id":  jobID,
    },
})
```

**Server-side logging:** log `jobID` at every phase transition at INFO level.
This is the only way to join client-side error reports to server-side traces.

**Definition of done:** Manual integration test — call `Execute` with
`https://example.com`, confirm all SSE events emitted in correct order.

---

### STEP 11 — In-Memory Job Store

**Goal:** Thread-safe job lifecycle management with TTL-based cleanup.

**Implement `JobStore` from Section 11.**

**Required methods:**
```go
func (s *JobStore) Create() *Job                               // generates UUID, status=pending
func (s *JobStore) Get(id string) (*Job, bool)
func (s *JobStore) SetRunning(id string)
func (s *JobStore) SetDone(id string, result *AnalysisResult)
func (s *JobStore) SetFailed(id string, errMsg string)
func (s *JobStore) Subscribe(id string) (<-chan SSEEvent, func()) // returns channel + unsubscribe func
func (s *JobStore) Publish(id string, event SSEEvent)
func (s *JobStore) StartReaper(ctx context.Context, ttl time.Duration) // background cleanup
```

**TTL Reaper — required, not optional.**
Without cleanup, each completed job holds its full `AnalysisResult` in memory
(~100KB for a 500-link page). At 1,000 jobs that's ~100MB. The reaper runs as
a background goroutine and deletes jobs older than `ttl` (default: 1 hour):

```go
func (s *JobStore) StartReaper(ctx context.Context, ttl time.Duration) {
    go func() {
        ticker := time.NewTicker(ttl / 2) // scan at half the TTL interval
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                s.reap(ttl)
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (s *JobStore) reap(ttl time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    cutoff := time.Now().Add(-ttl)
    for id, job := range s.jobs {
        if job.CreatedAt.Before(cutoff) {
            delete(s.jobs, id)
            delete(s.subscribers, id)
        }
    }
}
```

Call `store.StartReaper(rootCtx, time.Hour)` from `main.go` alongside pool startup.

**Concurrency requirements:**
- All methods must be safe for concurrent use
- `Subscribe` must buffer channel (size 50) to avoid blocking publisher

**TDD — 5 tests minimum:**
```
Test: Create → Get returns job with pending status
Test: SetDone → Get returns job with result
Test: Subscribe → Publish → channel receives event
Test: reap removes jobs older than TTL
Test: reap does not remove jobs younger than TTL
```

**Definition of done:** Tests green, race detector clean: `go test -race ./...`

---

### STEP 12 — HTTP Interface Layer

**Goal:** Two HTTP handlers wired to job store and use case.

**POST /analyze handler:**
1. Decode JSON body → validate URL (must have scheme, must be http/https)
2. Acquire analysis semaphore — reject with **429** if at capacity:
   ```go
   // NOTE: this 429 is US rate limiting the CLIENT.
   // Distinct from link checker 429s, which are EXTERNAL SERVERS rate limiting us.
   select {
   case sem <- struct{}{}:
       defer func() { <-sem }()
   default:
       w.WriteHeader(http.StatusTooManyRequests)
       json.NewEncoder(w).Encode(map[string]string{
           "error": "too many concurrent analyses, try again shortly",
       })
       return
   }
   ```
3. Create job via `JobStore.Create()`
4. Launch goroutine: `go uc.Execute(ctx, url, func(e SSEEvent){ store.Publish(jobID, e) })`
   - Semaphore slot released when `Execute` returns (via defer in step 2)
5. Return 202 `{ "job_id": "..." }`

**Semaphore sizing:** configurable via `AnalyzerConfig.MaxConcurrentJobs`, default 10.
Each job can run up to 100 link checker workers (from the global pool), so 10 concurrent
jobs means at most 10 orchestrator goroutines contending for the 100-worker pool.

**GET /analyze/stream handler:**
1. Read `id` query param → 404 if unknown
2. Set SSE headers (Section 5)
3. Subscribe via `store.Subscribe(id)`
4. Range over channel, write each event as:
   ```
   event: <event.Type>\n
   data: <json>\n\n
   ```
5. Flush after each event (`http.Flusher`)
6. Stop when channel closes OR client disconnects (`r.Context().Done()`)

**Definition of done:** 
- `curl -X POST localhost:8080/analyze -d '{"url":"https://example.com"}'` returns jobID
- `curl -N localhost:8080/analyze/stream?id=<jobID>` streams SSE events to terminal

---

### STEP 13 — Integration Test & Edge Cases

**Goal:** Verify full pipeline end-to-end using deterministic local mock servers.
No test may make a real outbound network call — all external URLs are served by
`httptest.NewServer` with controlled HTML fixtures.

**Test infrastructure — create once, reuse across all tests:**
```go
// testserver package or test helper
func NewMockServer(html string, statusCode int) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(statusCode)
        w.Write([]byte(html))
    }))
}
```

**Tests to write:**

```
Integration Test 1: full pipeline — mock server returns valid HTML5 page
  - HTML contains <title>, <h1>, <h2>, internal links, external links
  - assert html_version = "HTML5"
  - assert title matches fixture
  - assert headings counts match fixture
  - assert internal_links + external_links match fixture
  - assert partial = false

Integration Test 2: unreachable URL
  - submit URL to a port with no listener
  - assert SSE stream emits exactly one "error" event containing job_id
  - assert no "result" event emitted

Integration Test 3: page with no links
  - mock server returns valid HTML with no <a> tags
  - assert inaccessible_links = 0, internal_links = 0, external_links = 0

Integration Test 4: page with login form
  - mock server returns HTML with <form> + <input type="password"> + action="login"
  - assert has_login_form = true

Integration Test 5: context cancellation mid link-check
  - mock server for links has artificial delay (100ms per request)
  - cancel ctx after 50ms
  - assert result is emitted with partial = true
  - assert no goroutine leak (goleak)

Edge case tests (all using mock servers):
  - <base> tag → links resolved against base host, not page host
  - 0 headings → headings field is empty map {}, not null
  - fragment-only links (#anchor) → not counted in any link totals
  - duplicate links → counted once in link checker, correct total in result
  - link returns 429 → counted in unverified_links, not inaccessible_links
  - link returns 401 → counted in unverified_links
  - response body > 10MB → fetcher truncates, analysis continues without panic
  - User-Agent header → mock server asserts correct User-Agent on every request
```

**Definition of done:** All tests pass with zero real network calls. `go test -race ./...` clean.

---

### STEP 14 — README

**Goal:** Professional README with architecture diagram + scalability section.

**Sections to include:**
1. Overview
2. How to run (`go run cmd/app/main.go`)
3. API reference (POST /analyze + GET /analyze/stream with example curl commands)
4. Architecture overview (ASCII diagram from Section 1)
5. Design decisions (worker pool rationale, HEAD→GET fallback, SSE choice)
6. Scalability note — copy verbatim from Section 12 of this document
7. Testing (`go test ./...`)
8. Out of scope

---

## 14. Known Gaps & Decisions Not Implemented

| Gap | Decision |
|---|---|
| Rate limiting on `/analyze` | **Implemented** — semaphore (default cap 10) on POST handler. Returns 429 to client when at capacity. Note: this 429 is us throttling clients; unrelated to external server 429s in the link checker. |
| Global link checker pool | **Implemented** — 100-worker global pool started at boot. Hard ceiling on outbound connections regardless of concurrent job count. |
| Job TTL / cleanup | **Implemented** — TTL reaper goroutine (default 1hr) started at boot via `StartReaper`. Prevents unbounded memory growth in long-running processes. |
| `progress` callback on interface | **Fixed** — removed from `LinkChecker` interface. Application layer owns progress emission via 500ms ticker. Interface stays minimal and mockable. |
| Partial result on interrupted analysis | **Implemented** — `CheckAll` returns `(map, error)`. Application layer sets `Partial = true` on `AnalysisResult` when err != nil. Result always emitted, client inspects flag. |
| Correlation ID on SSE error events | **Implemented** — `job_id` included in all SSE error event payloads. Server logs jobID at every phase transition. |
| User-Agent on HTTP clients | **Implemented** — `Mozilla/5.0 (compatible; WebAnalyzer/1.0)` set on fetcher and link checker. Prevents 403s from servers blocking `Go-http-client/1.1`. |
| Body size limit on fetcher | **Implemented** — `io.LimitReader` cap at 10MB. Prevents OOM on oversized or malicious pages. |
| Integration tests hitting real network | **Fixed** — all integration tests use `httptest.NewServer` with HTML fixtures. Zero real outbound calls. Fully deterministic in CI. |
| Auth on SSE stream | Out of scope — jobID acts as unguessable token (UUID v4). `securitySchemes` placeholder in OpenAPI spec shows where auth would be added. |
| JS-rendered pages | Explicitly out of scope — static HTML only |
| CAPTCHA / auth flows | Explicitly out of scope |
| Deep crawling | Explicitly out of scope |
| `429 / 401 / 403` handling | **Implemented** — classified as `StatusUnverified`, counted in `unverified_links`. Not inaccessible: server refused check, link existence unconfirmed. No retry-after backoff. |

---

## 15. Dependencies (go.mod)

```
github.com/PuerkitoBio/goquery  v1.9.x   // HTML parsing
github.com/google/uuid          v1.6.x   // Job ID generation
go.uber.org/goleak              v1.3.x   // Goroutine leak detection in tests
```

No web framework. Standard library `net/http` only.

---

---

## 16. OpenAPI Specification

```yaml
openapi: 3.1.0

info:
  title: Web Page Analyzer API
  version: 1.0.0
  description: >
    Analyzes a given URL and extracts structural and accessibility information
    from the corresponding web page. Analysis runs asynchronously — submit a
    URL to receive a job ID, then stream real-time progress via SSE.

servers:
  - url: http://localhost:8080
    description: Local development
  - url: https://api.your-service.com
    description: Production

# Auth is out of scope for this implementation.
# The job_id UUID acts as an unguessable bearer token for the SSE stream.
# When auth is added, use the pattern below:
security: []

tags:
  - name: Analysis
    description: Submit and stream web page analysis jobs

paths:

  /analyze:
    post:
      tags: [Analysis]
      summary: Submit a URL for analysis
      description: >
        Validates the URL and enqueues an analysis job. Returns a job ID
        immediately. Use GET /analyze/stream?id={jobId} to stream progress
        and receive the final result.

        Returns 429 if the server is at maximum concurrent analysis capacity
        (default: 10 simultaneous jobs). This is server-side throttling —
        retry after a short delay.
      operationId: submitAnalysis
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AnalyzeRequest'
            example:
              url: "https://example.com"
      responses:
        "202":
          description: Job accepted — use the job_id to subscribe to the SSE stream
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AnalyzeResponse'
              example:
                job_id: "550e8400-e29b-41d4-a716-446655440000"
        "400":
          description: Invalid or malformed URL
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              examples:
                missing_scheme:
                  summary: Missing scheme
                  value:
                    error: "invalid URL: missing scheme (must be http or https)"
                invalid_url:
                  summary: Malformed URL
                  value:
                    error: "invalid URL"
        "429":
          description: >
            Server at maximum concurrent analysis capacity.
            This is the server throttling the client — unrelated to any 429s
            received from external servers during link checking.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              example:
                error: "too many concurrent analyses, try again shortly"

  /analyze/stream:
    get:
      tags: [Analysis]
      summary: Stream analysis progress and result via SSE
      description: >
        Opens a Server-Sent Events stream for the given job ID. Emits phase
        updates, a running link-check progress count, and a final result or
        error event.


        **Event sequence:**
        1. `phase` — fetching
        2. `phase` — parsing
        3. `phase` — checking_links
        4. `progress` — repeated on 500ms ticker during link checking
        5. `result` — terminal, stream closes after this (may have partial=true)
           OR `error` — terminal, replaces result on unrecoverable failure


        **Reconnection:** if the client disconnects and reconnects with the
        same job ID before the job completes, it will resume receiving events.
        If the job has already completed, the `result` event is replayed
        immediately and the stream closes.


        **Partial results:** if the analysis is interrupted mid link-check
        (e.g. context cancelled, graceful shutdown), a `result` event is still
        emitted with `partial: true`. Link counts may be incomplete but all
        other fields are accurate.
      operationId: streamAnalysis
      parameters:
        - name: id
          in: query
          required: true
          description: Job ID returned by POST /analyze
          schema:
            type: string
            format: uuid
          example: "550e8400-e29b-41d4-a716-446655440000"
      responses:
        "200":
          description: SSE stream opened successfully
          headers:
            Content-Type:
              schema:
                type: string
                enum: ["text/event-stream"]
            Cache-Control:
              schema:
                type: string
                enum: ["no-cache"]
            Connection:
              schema:
                type: string
                enum: ["keep-alive"]
          content:
            text/event-stream:
              schema:
                type: string
                description: >
                  Newline-delimited SSE frames. Each frame has an `event` type
                  and a `data` field containing a JSON payload.
              examples:
                phase_event:
                  summary: Phase update
                  value: |
                    event: phase
                    data: {"phase":"fetching","message":"Fetching page..."}

                    event: phase
                    data: {"phase":"parsing","message":"Parsing HTML..."}

                    event: phase
                    data: {"phase":"checking_links","message":"Checking links..."}

                progress_event:
                  summary: Link check progress
                  value: |
                    event: progress
                    data: {"checked":47,"total":200}

                result_event:
                  summary: Final result (terminal)
                  value: |
                    event: result
                    data: {"html_version":"HTML5","title":"Example Domain","headings":{"h1":1},"internal_links":3,"external_links":12,"inaccessible_links":1,"unverified_links":2,"has_login_form":false}

                result_partial_event:
                  summary: Partial result — interrupted mid link-check (terminal)
                  value: |
                    event: result
                    data: {"html_version":"HTML5","title":"Example Domain","headings":{"h1":1},"internal_links":3,"external_links":12,"inaccessible_links":0,"unverified_links":0,"has_login_form":false,"partial":true}

                error_event:
                  summary: Error (terminal — replaces result on unrecoverable failure)
                  value: |
                    event: error
                    data: {"message":"Could not reach host: example.com","job_id":"550e8400-e29b-41d4-a716-446655440000"}

        "404":
          description: Job ID not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              example:
                error: "job not found"

components:
  schemas:

    AnalyzeRequest:
      type: object
      required: [url]
      properties:
        url:
          type: string
          format: uri
          description: >
            Fully qualified URL to analyze. Must include scheme (http or https).
          example: "https://example.com"

    AnalyzeResponse:
      type: object
      required: [job_id]
      properties:
        job_id:
          type: string
          format: uuid
          description: >
            Opaque job identifier. Treat as an unguessable token — there is no
            auth on the SSE stream beyond possession of this ID.
          example: "550e8400-e29b-41d4-a716-446655440000"

    AnalysisResult:
      type: object
      description: >
        Final analysis output — emitted as the `result` SSE event payload.
        Always emitted, even on partial completion. Check the `partial` field
        before trusting link counts.
      required:
        - html_version
        - title
        - headings
        - internal_links
        - external_links
        - inaccessible_links
        - unverified_links
        - has_login_form
      properties:
        html_version:
          type: string
          description: >
            Detected HTML version from the page doctype.
          enum:
            - "HTML5"
            - "HTML 4.01 Strict"
            - "HTML 4.01 Transitional"
            - "XHTML 1.0"
            - "XHTML 1.1"
            - "Unknown"
          example: "HTML5"
        title:
          type: string
          description: Content of the page <title> tag. Empty string if absent.
          example: "Example Domain"
        headings:
          type: object
          description: >
            Count of each heading level found on the page.
            Only levels with at least one occurrence are included —
            missing keys mean zero occurrences, not an error.
          properties:
            h1:
              type: integer
              minimum: 0
            h2:
              type: integer
              minimum: 0
            h3:
              type: integer
              minimum: 0
            h4:
              type: integer
              minimum: 0
            h5:
              type: integer
              minimum: 0
            h6:
              type: integer
              minimum: 0
          example:
            h1: 1
            h2: 3
        internal_links:
          type: integer
          minimum: 0
          description: >
            Count of links pointing to the same hostname as the analyzed page.
            www prefix is normalized before comparison.
          example: 10
        external_links:
          type: integer
          minimum: 0
          description: >
            Count of links pointing to a different hostname.
            Subdomains are treated as external.
          example: 5
        inaccessible_links:
          type: integer
          minimum: 0
          description: >
            Count of links that returned HTTP 4xx (excluding 401/403/429)
            or 5xx, or failed with a network error (timeout, DNS, TLS).
            Fragment-only links and duplicate URLs are not double-counted.
            May be understated if partial=true.
          example: 2
        unverified_links:
          type: integer
          minimum: 0
          description: >
            Count of links that returned HTTP 401, 403, or 429.
            These are not confirmed broken — the server actively refused
            the check. Distinct from inaccessible_links.
            May be understated if partial=true.
          example: 1
        has_login_form:
          type: boolean
          description: >
            True if the page contains a form with a password input field
            and a login-related signal (action, class, id, or visible text
            containing "login", "sign in", "signin", "log in", or "anmelden").
            Falls back to true if a password input exists anywhere on the page
            with no matching form context.
          example: false
        partial:
          type: boolean
          description: >
            Present and true only if the analysis was interrupted before all
            links could be checked (e.g. server shutdown, context cancellation).
            html_version, title, headings, and has_login_form are always accurate.
            Link counts (inaccessible_links, unverified_links) may be understated.
          example: true

    PhaseEvent:
      type: object
      description: SSE payload for `event: phase`
      required: [phase, message]
      properties:
        phase:
          type: string
          enum: [fetching, parsing, checking_links]
          example: "checking_links"
        message:
          type: string
          example: "Checking links..."

    ProgressEvent:
      type: object
      description: SSE payload for `event: progress` — emitted on 500ms ticker during link checking
      required: [checked, total]
      properties:
        checked:
          type: integer
          minimum: 0
          description: Number of unique links checked so far
          example: 47
        total:
          type: integer
          minimum: 0
          description: Total unique links to check
          example: 200

    ErrorResponse:
      type: object
      required: [error]
      properties:
        error:
          type: string
          example: "invalid URL"

    SSEErrorEvent:
      type: object
      description: >
        SSE payload for `event: error` (terminal).
        Emitted on unrecoverable failures only — fetch failure, invalid URL.
        Partial link-check results use `result` with partial=true instead.
      required: [message, job_id]
      properties:
        message:
          type: string
          example: "Could not reach host: example.com"
        job_id:
          type: string
          format: uuid
          description: >
            Included for log correlation. Join this against server-side logs
            to trace the full request lifecycle.
          example: "550e8400-e29b-41d4-a716-446655440000"

  # Security schemes placeholder — auth is out of scope for this implementation.
  # When added, replace the job_id-as-token pattern with one of:
  #   - Bearer JWT on both POST /analyze and GET /analyze/stream
  #   - API key in X-Api-Key header
  securitySchemes: {}
```

---

*End of document. Total steps: 14. Estimated LLM sessions: 7–10 (2 steps per session recommended).*
