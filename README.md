# Page Insight Tool

A web application that takes a URL and tells you what's inside it — HTML version, page title, heading structure, link counts, whether the page has a login form, and which links are broken or blocked. Built with a Go backend and a Next.js frontend.

---

## Getting Started

### Prerequisites

- **Go 1.22+**
- **Node.js 20.9+**
- **pnpm** (or npm/yarn — adjust commands accordingly)


### Docker Compose

From the repository root, build and run the API and web UI:

```bash
make docker-up
```

This runs `docker compose up --build` using `compose.yaml`: the **server** service is exposed on port **8080**, the **web** service on **3000**, and `BACKEND_URL` is set to `http://server:8080` for the frontend container.

Other targets:

```bash
make docker-down    # docker compose down
make docker-build   # docker compose build
```

---

## How It Works

When you submit a URL, a few things happen in sequence:

1. The frontend POSTs to `/analyze` and gets back a job ID.
2. It opens a Server-Sent Events connection to `/analyze/stream?id=<jobID>`.
3. The backend fetches the page, parses the HTML, then checks every link it finds — concurrently, using a shared worker pool.
4. As each phase completes, the backend pushes an event down the stream. The frontend updates in real time.
5. When all links are checked, the backend sends a final `result` event and closes the stream.

The whole thing is asynchronous on both ends. The backend never blocks on a single job, and the frontend never polls — it just listens.

---

## Design Decisions

### Backend

**Why non-blocking backpressure on POST `/analyze` (semaphore, no queue)?**
Accepting a new analysis acquires a counting semaphore with a **non-blocking** try: if every slot is in use, the handler returns **429** immediately instead of blocking the HTTP request until capacity frees up. That prevents backpressure from turning into long queues of stalled connections and keeps per-request behaviour predictable. There is **no enqueue** of overflow work a submission either succeeds (**202** + `job_id`, with analysis running asynchronously in a goroutine) or is refused outright. 

The default cap is 6 concurrent analyses (`WEB_ANALYZER_MAX_CONCURRENT_JOBS`), i set it to 6 because the SSE browser window limits under the assumption that only one client would be using it at a time.

**Why SSE ?**
Link checking takes a while sometimes 30+ seconds on pages with hundreds of slow external links. A plain HTTP response would just hang until the very end, which is a terrible user experience. SSE lets the backend push progress updates as they happen, so the user sees something moving rather than staring at a spinner.
It added a bit more complexity but made the UX slightly better

**Global worker pool**
Link checks are run through a fixed-size worker pool that is shared by every analysis job. The pool bounds how many outbound HTTP checks can run at once (default 100 workers), so load on remote hosts and on this process stays predictable even when a single page exposes a very large number of links or several analyses overlap. Workers are started once at process startup and reused for the lifetime of the server.

**Why HEAD before GET for link checking?**
HEAD requests are cheaper the server sends back headers but no body, so it's faster and uses less bandwidth. Most servers honour them correctly. For the ones that don't (returning 405 Method Not Allowed), the checker automatically retries with GET. This means link checking is as fast as possible in the common case and still correct in the awkward cases.

**Why distinguish "unverified" links from "inaccessible" ones?**
A link that returns 401, 403, or 429 isn't broken the server actively responded and said "you can't have this." That's meaningfully different from a link that times out or returns a 404. Lumping them together would give misleading counts.

**Why does the backend set a `User-Agent` header on every outbound request?**
Without it, requests go out as `Go-http-client/1.1`, which a surprising number of servers block or rate-limit by default. Setting a browser-like User-Agent string gets more accurate link check results — you're trying to simulate what a real user visiting the page would experience, not announce yourself as a bot.

**Why cap the fetched page at 10MB?(Failure mode)**
Some pages are enormous, some are malicious. Without a cap, a single job could exhaust available memory. 10MB is generous the vast majority of real pages are under 1MB. If a page is truncated, the analyser logs a warning and continues with what it has. Partial HTML is still parseable.

**Job state stored in memory instead of a database?**
 The in-memory store is fast, simple, and sufficient. There's a TTL reaper that cleans up completed jobs after an hour so memory doesn't grow unboundedly over time.

**Why TDD for the domain and infrastructure layers?**
The link classification rules, URL normalisation, login form detection, and HTML version detection are all pure functions — they take input and return output with no side effects. Pure functions are the easiest possible thing to test, and getting them wrong (e.g. miscounting internal vs external links, or missing a login form) silently produces wrong results that are hard to catch later. The tests also serve as documentation for the rules — you can read the test cases and immediately understand what the classifier is supposed to do.

### Frontend

**Why proxy the backend through Next.js API routes?**
It keeps `BACKEND_URL` server-side only — it never appears in the browser bundle. It also means the frontend and backend can be deployed independently without touching CORS configuration on the Go server. All the browser knows is that it's talking to `/api/analyze`.

**Why `useReducer` instead of separate `useState` calls?**
The application has seven distinct states (`idle`, `submitting`, `streaming`, `rate_limited`, `done`, `error`, and the three streaming phases). With separate state variables you end up with combinations that shouldn't be possible — like having a result and an error at the same time. A reducer makes the valid states explicit and transitions between them deliberate.

---

## Known Limitations

A few things are deliberately out of scope for this version:

- **JavaScript-rendered pages** — the fetcher gets static HTML only. Pages that render their content client-side (heavy React/Vue apps, SPAs) will come back mostly empty. This would require a headless browser like Playwright to fix, which is a significant added dependency.
- **Authenticated pages** — if a page requires login to access, the fetcher will get whatever the logged-out version looks like (usually a redirect or an empty shell).
- **Cross-domain redirects (rebrands)** — the HTTP client follows redirects, but when the final page is on a different host than the URL you entered (for example `twitter.com` → `x.com`), the report describes that final response. Internal vs external links, titles, and structure are based on the redirected page, which may not match how you think about the original URL.
- **Login form detection** — login detection is a lightweight HTML heuristic. It can miss real login screens or mark pages that are not actually login gates, so treat the “has login form” signal as approximate.
- **Deep crawling** — the tool analyses a single page at a time. It checks whether the links on that page are reachable, but it doesn't follow them and analyse those pages too.
- **No auth on the SSE stream** — the job ID (a UUID v4) acts as an unguessable token. Good enough for a single-user tool, not good enough for multi-tenant production use.
- **No retry-after backoff on external 429s** — when an external server rate-limits the link checker, the checker logs it as "unverified" and moves on. It doesn't parse the `Retry-After` header and wait. This keeps the analysis time predictable at the cost of potentially missing some links.


---

## Possible Future Improvements

**Make it smarter about what it fetches**
Adding a headless browser option (via Playwright or chromedp) would let the tool handle JS-rendered pages — useful for auditing modern SPAs. It would be an opt-in flag rather than the default, since it's significantly slower and heavier.

**Authentication support**
Letting users provide a session cookie or auth header would unlock the ability to analyse pages that are behind a login. The fetcher interface is already designed to accept headers, so the plumbing is mostly there.

**Retry-After backoff for link checking**
When an external server returns 429 with a `Retry-After` header, the link checker could honour it — wait the specified duration and retry. This would improve accuracy on sites that rate-limit aggressively.

**Persistent job store**
Swapping the in-memory store for Redis would mean jobs survive process restarts and the service could scale horizontally. The job store interface is already abstracted, so the swap wouldn't require changes to the application layer — just a new implementation behind the same interface.

**Distributed worker pool**
Right now all link checking happens in the same process. With a Redis-backed job queue (e.g. Asynq), you could run worker processes separately and scale them independently of the API. The backend design doc includes a detailed diagram of what this would look like.

---

## Dependencies

**Backend:**
- [`github.com/PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery) — HTML parsing
- [`github.com/google/uuid`](https://github.com/google/uuid) — job ID generation
- [`go.uber.org/goleak`](https://github.com/uber-go/goleak) — goroutine leak detection in tests

No web framework. Everything else is standard library.

**Frontend:**
- Next.js 16, React 19, TypeScript, Tailwind CSS
- Native `EventSource` API for SSE — no additional library needed