# Web Page Analyzer

A Go service that analyses a URL and extracts structural and accessibility
information from the corresponding web page. Analysis runs asynchronously —
submit a URL to receive a job ID, then stream real-time progress via
Server-Sent Events (SSE).

---

## How to run

```bash
# Optional DX setup:
cp .env.example .env

go run cmd/app/main.go
# Server starts on WEB_ANALYZER_HTTP_ADDR (default: :8080)
# Ctrl+C for graceful shutdown (30s drain)
```

---

## Configuration (env)

The app auto-loads `.env` (if present) and falls back to defaults.

| Variable | Default | Purpose |
|---|---|---|
| `WEB_ANALYZER_HTTP_ADDR` | `:8080` | HTTP listen address |
| `WEB_ANALYZER_MAX_CONCURRENT_JOBS` | `10` | Max concurrent analysis jobs |
| `WEB_ANALYZER_JOB_TTL` | `1h` | In-memory job retention before reaper cleanup |
| `WEB_ANALYZER_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown drain timeout |
| `WEB_ANALYZER_LINK_CHECKER_MAX_WORKERS` | `100` | Global link-checker worker pool size |
| `WEB_ANALYZER_LINK_CHECKER_JOB_BUFFER_SIZE` | `500` | Link-checker shared jobs channel buffer |
| `WEB_ANALYZER_LINK_CHECKER_TIMEOUT` | `10s` | Per-link checker HTTP timeout |
| `WEB_ANALYZER_LINK_CHECKER_RETRIES` | `2` | Retry attempts for transient link-check failures |
| `WEB_ANALYZER_PAGE_FETCH_TIMEOUT` | `10s` | Initial page fetch timeout |

---

## API reference

### OpenAPI docs

- Interactive docs: [http://localhost:8080/docs](http://localhost:8080/docs)
- Raw spec: [http://localhost:8080/openapi.yaml](http://localhost:8080/openapi.yaml)
- Source file: `docs/openapi.yaml`

---

### POST /analyze

Submit a URL for analysis. Returns a job ID immediately.

```bash
curl -s -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
# {"job_id":"550e8400-e29b-41d4-a716-446655440000"}
```

**Responses**

| Status | Meaning |
|--------|---------|
| `202`  | Job accepted — use `job_id` to stream results |
| `400`  | Invalid or missing URL |
| `429`  | Server at maximum concurrent analysis capacity (default: 10) — retry shortly |

---

### GET /analyze/stream?id=\<jobID\>

Open an SSE stream for the job. Events are flushed as they are produced.

```bash
curl -N "http://localhost:8080/analyze/stream?id=550e8400-e29b-41d4-a716-446655440000"
```

**Event sequence**

```
event: phase
data: {"phase":"fetching","message":"Fetching page..."}

event: phase
data: {"phase":"parsing","message":"Parsing HTML..."}

event: phase
data: {"phase":"checking_links","message":"Checking links...","total":47}

event: progress
data: {"checked":12,"total":47}

event: result
data: {"html_version":"HTML5","title":"Example Domain","headings":{"h1":1},
       "internal_links":3,"external_links":12,"inaccessible_links":1,
       "unverified_links":2,"has_login_form":false}
```

Or on failure:

```
event: error
data: {"message":"could not fetch https://example.com: ...","job_id":"550e8400-..."}
```

**Notes**

- `result` and `error` are terminal — the stream closes after either.
- If the job has already completed when you connect, the `result` event is
  replayed immediately.
- If the analysis was interrupted mid link-check (e.g. graceful shutdown),
  a `result` event is still emitted with `"partial": true`; link counts may
  be understated.

---

## Architecture

```
Client (e.g. Next.js)
        │
        ├── POST /analyze          → { job_id }
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
              │  SSE Event Stream  │
              └────────────────────┘
```

---

## Design decisions

### Global shared worker pool

The `GlobalLinkChecker` starts a fixed-size pool of goroutines at boot time
(`MaxWorkers: 100` by default). All concurrent analysis jobs share this pool.
`CheckAll` submits jobs to a shared channel and collects results via a
per-call buffered result channel — no routing table, no shared state.

This gives a hard ceiling on outbound HTTP connections regardless of how many
analyses are running concurrently.

### HEAD→GET fallback

Link reachability checks try `HEAD` first (lightweight). If the server
returns `405 Method Not Allowed`, the checker transparently retries with
`GET`. All requests include a realistic `User-Agent` header to avoid `403`
responses from servers that block `Go-http-client/1.1`.

### Three-valued link status

A link can be `accessible`, `inaccessible`, or `unverified`. HTTP `401`,
`403`, and `429` responses are classified as `unverified` rather than
`inaccessible` — the server actively refused the check; the link may well
exist. This prevents false positives in the broken-link count.

### SSE over WebSocket

SSE is unidirectional, fire-and-forget, and works over plain HTTP/1.1 with
no special infrastructure. The job ID acts as an unguessable token for the
stream — no additional auth is required for this implementation.

---

## Scalability note

### Current architecture limits

The current implementation uses an in-memory job store, a concurrency
semaphore, and a global shared worker pool. This provides hard resource
ceilings suitable for moderate load, with the following constraints:

- **Bounded concurrency** — max 10 concurrent analyses (semaphore) and 100
  link checker workers (global pool). Requests beyond capacity receive a
  `429` immediately rather than degrading system resources. Explicit
  rejection is preferable to silent degradation.
- **Semaphore slots are held for job duration** — a job with 500 slow
  external links holds its slot for the full analysis. Under sustained load
  with slow external servers, all 10 slots can remain occupied for extended
  periods. Tune `MaxConcurrentJobs` and `MaxWorkers` together based on
  observed p95 job latency.
- **State is process-local** — horizontal scaling requires sticky sessions
  or a shared job store (e.g. Redis). Two API instances cannot share job
  state or SSE subscribers.
- **No persistence** — jobs are lost on process restart. The TTL reaper
  manages memory within a process lifetime but does not survive restarts.

### Path to production scale

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

This pattern decouples ingestion from processing and allows independent
scaling of API nodes and worker nodes.

---

## Testing

```bash
go test ./...              # all tests
go test -race ./...        # with race detector
go test -v ./...           # verbose output
```

The test suite covers 50+ test cases across unit, TDD, and integration tests.
All integration tests use `httptest.NewServer` fixtures — zero real outbound
network calls.

---

## Out of scope

- JavaScript-rendered pages (static HTML only)
- Authentication / CAPTCHA flows
- Deep crawling
- Persistent storage (jobs are in-memory, lost on restart)
- Auth on the SSE stream (job UUID acts as bearer token)
