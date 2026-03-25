# Page Insight Tool — Frontend System Design

> Next.js 16.2 · TypeScript · App Router · SSE · Tailwind CSS · React 19.2  
> Status: Ready for Implementation

---

## 0. Design Direction

**Aesthetic:** Dark, precision-tool. Think terminal meets dashboard — deep navy/charcoal backgrounds, sharp mono accents, clean data display. Not flashy, but *deliberate*. Every loading state has personality.

**Font pairing:**
- Display/headings: `IBM Plex Mono` — technical credibility
- Body: `DM Sans` — readable, modern without being generic

**Colour system:**
```css
--bg-base:      #0D1117   /* page background */
--bg-surface:   #161B22   /* cards */
--bg-elevated:  #1C2128   /* input, hover states */
--accent:       #58A6FF   /* primary actions, progress */
--accent-warm:  #F0883E   /* warnings, unverified */
--success:      #3FB950   /* accessible, login detected */
--danger:       #F85149   /* errors, inaccessible */
--text-primary: #E6EDF3
--text-muted:   #7D8590
--border:       #30363D
```

---

## 1. Tech Stack

| Layer | Choice | Notes |
|---|---|---|
| Framework | **Next.js 16.2** | Latest stable. App Router. Turbopack default. |
| React | **React 19.2** | Required by Next.js 16 — includes View Transitions, `<Activity />` |
| Language | **TypeScript** | Strict mode. Typed SSE payloads. |
| Styling | **Tailwind CSS** | Pre-installed by `create-next-app` defaults |
| Fonts | **`next/font`** | Zero layout shift, automatic optimisation |
| SSE Client | **Native `EventSource`** | Browser built-in, no library overhead |
| State | **`useReducer`** | Predictable state machine, no external lib |
| Bundler | **Turbopack** | Default in Next.js 16 — 5-10x faster Fast Refresh |
| Compiler | **React Compiler** | Opt-in stable — automatic memoisation, no manual `useMemo` |
| HTTP | **`fetch()`** | POST /analyze only |
| Testing | **Vitest + React Testing Library** | Fast, Vite-based |
| Node.js | **≥ 20.9** | Next.js 16 minimum — Node 18 support removed |

### Next.js 16 Scaffold Command

```bash
pnpm create next-app@latest page-insight --yes
# defaults: TypeScript, Tailwind, ESLint, App Router, Turbopack, @/* alias
cd page-insight
pnpm dev
```

### `next.config.ts`

```ts
import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  // Caching is opt-in in Next.js 16 — everything dynamic by default.
  // This app is fully request-driven, so no cacheComponents config needed.

  // React Compiler — stable in Next.js 16.
  // Automatically memoises result card components, no manual memo() needed.
  reactCompiler: true,
}

export default nextConfig
```

**Why `cacheComponents` is not enabled:** The entire application is request-driven — URL input, SSE stream, result display. There is nothing to cache. Next.js 16's new default (all dynamic unless marked `"use cache"`) is exactly right here with zero config.

---

## 2. Next.js 16 Breaking Changes Relevant to This Project

| Change | Impact | Action Required |
|---|---|---|
| `middleware.ts` → `proxy.ts` | Only matters if adding auth proxy later | Rename file + function to `proxy` |
| `experimental.ppr` removed | Not used in this project | None |
| Node 18 support dropped | Dev + CI environment requirement | Specify `"node": ">=20.9"` in `package.json` |
| `next lint` removed | Run ESLint directly | Update `package.json` lint script |
| React 19.2 required | `<Activity />` available | No action — use freely |
| Caching opt-in by default | Nothing cached unless `"use cache"` used | No action — desired behaviour |

---

## 3. Project Structure

```
src/
├── app/
│   ├── layout.tsx                    # Root layout — fonts, metadata
│   ├── page.tsx                      # Server Component — renders <AnalysisPage />
│   ├── globals.css                   # CSS variables + keyframe definitions
│   │
│   └── api/
│       ├── analyze/
│       │   └── route.ts              # POST proxy → Go backend
│       └── analyze/stream/
│           └── route.ts              # SSE stream proxy → Go backend
│
├── components/
│   ├── analysis/
│   │   ├── AnalysisPage.tsx          # 'use client' root — owns all state
│   │   ├── UrlForm.tsx               # Input, validation, submitting state
│   │   ├── ProgressPanel.tsx         # 3-step stepper + progress bar
│   │   ├── ResultPanel.tsx           # Orchestrates result cards
│   │   ├── ErrorCard.tsx             # Error display with optional status badge
│   │   ├── RateLimitedCard.tsx       # 429 — amber countdown + auto-retry
│   │   ├── PageMetaCard.tsx          # HTML version + title
│   │   ├── HeadingsChart.tsx         # h1–h6 bar visualisation
│   │   ├── LinksBreakdown.tsx        # 4 stat tiles
│   │   └── LoginFormBadge.tsx        # Detected / not detected pill
│   │
│   └── ui/
│       ├── Skeleton.tsx              # Shimmer skeleton placeholder
│       ├── Badge.tsx                 # Coloured pill badge
│       ├── StatTile.tsx              # Number + label tile
│       ├── Tooltip.tsx               # Hover tooltip
│       └── ProgressBar.tsx           # Animated progress bar
│
├── hooks/
│   └── useAnalysis.ts                # SSE + POST logic. Returns state + submit.
│
├── types/
│   └── analysis.ts                   # TypeScript interfaces for all SSE payloads
│
└── utils/
    ├── urlValidation.ts              # Client-side URL format check
    └── sseParser.ts                  # Typed SSE event parsing helpers
```

### Server vs Client Component Boundary

```
app/page.tsx                  ← Server Component (default)
  └── <AnalysisPage />        ← 'use client' — all interactivity lives here
        ├── <UrlForm />
        ├── <ProgressPanel />
        ├── <ErrorCard />
        ├── <RateLimitedCard />
        └── <ResultPanel />
              ├── <PageMetaCard />
              ├── <HeadingsChart />
              ├── <LinksBreakdown />
              └── <LoginFormBadge />
```

`app/page.tsx` stays a Server Component so `export const metadata` stays server-side. The `'use client'` boundary is `AnalysisPage` — one clean split.

---

## 4. State Machine

The entire application lives in one `useReducer`. Six distinct states, each with its own UI.

```
         submit
  IDLE ──────────► SUBMITTING
                        │
              POST 202  │  POST 400/5xx     POST 429
                        │                ──────────► RATE_LIMITED
                        │                  (countdown → auto-resubmit → IDLE)
                        ▼
              STREAMING ──────────► ERROR (terminal)
                        │
              result    │
              event     ▼
                      DONE (terminal)

  ERROR and DONE re-enable the form → RESET → IDLE
  RATE_LIMITED counts down then re-submits the same URL automatically
```

### State Shape

```typescript
// types/analysis.ts

export type Phase = 'fetching' | 'parsing' | 'checking_links'

export type AnalysisState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'streaming'; phase: Phase; checked: number; total: number }
  | { status: 'done'; result: AnalysisResult; partial: boolean }
  | { status: 'error'; message: string; statusCode?: number }
  | { status: 'rate_limited'; retryIn: number; url: string }
  // retryIn: seconds remaining. url: preserved so auto-retry can resubmit it.

export type Action =
  | { type: 'SUBMIT' }
  | { type: 'STREAM_OPEN' }
  | { type: 'PHASE'; payload: Phase }
  | { type: 'PROGRESS'; checked: number; total: number }
  | { type: 'RESULT'; payload: AnalysisResult }
  | { type: 'ERROR'; message: string; statusCode?: number }
  | { type: 'RATE_LIMITED'; url: string }
  | { type: 'RETRY_TICK' }        // decrements retryIn by 1 each second
  | { type: 'RESET' }

export interface AnalysisResult {
  html_version: string
  title: string
  headings: Partial<Record<'h1'|'h2'|'h3'|'h4'|'h5'|'h6', number>>
  internal_links: number
  external_links: number
  inaccessible_links: number
  unverified_links: number
  has_login_form: boolean
  partial?: boolean
}

export interface SSEErrorPayload {
  message: string
  job_id: string
  status_code?: number  // absent for network-level failures (DNS/timeout/TLS)
}
```

---

## 5. Loading States — Full Specification

Each state is visually and interactively distinct. This is the core UX differentiator.

---

### State 1: `idle`

The URL form. Clean, nothing else visible.

```
┌─────────────────────────────────────────────┐
│  🔍  https://                               │
│                                  [ Analyse ] │
└─────────────────────────────────────────────┘
```

- Submit button: solid `--accent`, full opacity
- Input autofocused on first render
- No results or progress panel visible

---

### State 2: `submitting`

**Duration:** ~100–300ms until SSE opens.  
Form locks. Button content swaps to spinner — no layout shift.

```
┌─────────────────────────────────────────────┐
│  🔍  https://example.com                   │  ← disabled, opacity-50
│                               [ ⠋ Sending ] │  ← spinner + text
└─────────────────────────────────────────────┘
```

```tsx
<button disabled={status !== 'idle'} aria-busy={status === 'submitting'}>
  {status === 'submitting' ? (
    <span className="flex items-center gap-2">
      <SpinnerIcon className="animate-spin size-4" />
      Sending
    </span>
  ) : 'Analyse'}
</button>
```

Both states use the same button width — `min-w-[100px]` prevents resize on text swap.

---

### State 3: `streaming` — Phase: `fetching`

Progress panel slides in. Step 1 active with radar-pulse dot.

```
 ●──○──○
 [1] Fetching page...   ← animate-ping dot
 [2] Parsing HTML       ← muted
 [3] Checking links     ← muted
```

**Entrance animation:**
```css
@keyframes fade-slide-in {
  from { opacity: 0; transform: translateY(8px); }
  to   { opacity: 1; transform: translateY(0); }
}
```

**Active step dot:** Relative-positioned wrapper with an absolute `animate-ping` ring at 60% opacity. CSS only.

---

### State 3: `streaming` — Phase: `parsing`

Step 1 checkmark pops in. Step 2 activates.

```
 ✓──●──○
 ✓  Fetched
 ●  [2] Parsing HTML...
 ○  [3] Checking links
```

```css
@keyframes scale-in {
  from { transform: scale(0) rotate(-20deg); opacity: 0; }
  to   { transform: scale(1) rotate(0deg);   opacity: 1; }
}
.step-check { animation: scale-in 200ms ease forwards; }
```

---

### State 3: `streaming` — Phase: `checking_links`

The richest loading state. Steps 1 + 2 complete. Progress bar + skeleton cards appear.

```
 ✓──✓──●
 ✓  Fetched
 ✓  Parsed
 ●  [3] Checking links...

 ████████████░░░░░░░░  47 / 200 links
```

**Progress bar — CSS width transition, no JS loop:**
```tsx
<div className="h-1 bg-[--border] rounded-full overflow-hidden">
  <div
    className="h-full bg-[--accent] rounded-full transition-[width] duration-300 ease-out"
    style={{ width: `${total > 0 ? (checked / total) * 100 : 0}%` }}
    role="progressbar"
    aria-valuenow={checked}
    aria-valuemin={0}
    aria-valuemax={total}
  />
</div>
<span className="font-mono tabular-nums text-sm text-[--text-muted]">
  {checked} / {total} links
</span>
```

`tabular-nums` prevents layout shift as the counter increments.

**Skeleton cards:** Four shimmer placeholders matching the exact dimensions of the real result cards appear below the progress panel. They eliminate the blank-space problem and communicate that results are loading.

```tsx
// Skeleton.tsx
export function Skeleton({ className }: { className?: string }) {
  return (
    <div className={cn('rounded-md skeleton-shimmer', className)} aria-hidden="true" />
  )
}
```

```css
/* globals.css */
@keyframes shimmer {
  0%   { background-position: -200% 0; }
  100% { background-position:  200% 0; }
}
.skeleton-shimmer {
  background: linear-gradient(
    90deg,
    var(--bg-elevated) 25%,
    var(--bg-surface)  50%,
    var(--bg-elevated) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.6s ease-in-out infinite;
}
```

React 19's `<Activity />` keeps skeleton cards mounted but visually hidden during the transition to real results — prevents a remount flash.

---

### State 4: `rate_limited`

**Trigger:** `POST /analyze` returns 429. The backend is at max concurrent job capacity (default 10).  
**What the user sees:** Amber card — not red. This is not a failure, just a queue. Countdown ticks down, then auto-retries the same URL.

```
┌──────────────────────────────────────────────────┐
│  ⏱  Server busy — retrying in 5s...             │
│     [ Cancel ]                                   │
└──────────────────────────────────────────────────┘
```

**Critically different from `error`:**
- Colour: `--accent-warm` (amber) not `--danger` (red)
- No shake animation — this isn't a failure
- Shows a live countdown: `retrying in 5s… 4s… 3s…`
- Has a Cancel button that resets to `idle` without retrying
- On countdown reaching 0: automatically dispatches `SUBMIT` with the preserved URL

**Countdown implementation — `useEffect` interval, not state ticks:**
```tsx
// RateLimitedCard.tsx
useEffect(() => {
  if (state.status !== 'rate_limited') return

  const interval = setInterval(() => {
    dispatch({ type: 'RETRY_TICK' })
  }, 1000)

  return () => clearInterval(interval)
}, [state.status])

// When retryIn hits 0, parent useAnalysis auto-submits
useEffect(() => {
  if (state.status === 'rate_limited' && state.retryIn <= 0) {
    submit(state.url)   // re-submits the preserved URL
  }
}, [state])
```

**Reducer cases:**
```typescript
case 'RATE_LIMITED':
  return { status: 'rate_limited', retryIn: 5, url: action.url }

case 'RETRY_TICK':
  if (state.status !== 'rate_limited') return state
  return { ...state, retryIn: state.retryIn - 1 }
```

**Why 5 seconds?** The backend 429 means all 10 semaphore slots are occupied. At ~2–30s per job, 5s gives a reasonable chance a slot frees up without being so long it feels broken. No backoff — a single retry is sufficient; if it 429s again, it falls through to a normal `error`.

**Second consecutive 429:** If the auto-retry also gets a 429, dispatch `ERROR` with the backend message. Do not loop indefinitely.

```typescript
// In submit(), track retry attempts
if (res.status === 429 && !isRetry) {
  dispatch({ type: 'RATE_LIMITED', url })
  return
}
if (res.status === 429 && isRetry) {
  // Second 429 — give up, show error
  dispatch({ type: 'ERROR', message: body.error, statusCode: 429 })
  return
}
```

**Aria:** `role="status"` on the card — it's informational, not an alert. The countdown uses `aria-live="polite"` with `aria-atomic="true"` so screen readers announce `"retrying in 4 seconds"` each tick without being disruptive.

---

### State 5: `done`

Progress panel fades out. Result cards stagger in with `fade-up`.

```css
@keyframes fade-up {
  from { opacity: 0; transform: translateY(12px); }
  to   { opacity: 1; transform: translateY(0); }
}

.result-card                  { animation: fade-up 0.35s ease forwards; }
.result-card:nth-child(1)     { animation-delay:   0ms; }
.result-card:nth-child(2)     { animation-delay:  70ms; }
.result-card:nth-child(3)     { animation-delay: 140ms; }
.result-card:nth-child(4)     { animation-delay: 210ms; }
```

With `reactCompiler: true`, result card components are automatically memoised — no manual `memo()` needed.

**If `partial: true`:** Amber warning banner above the cards:
```
⚠  Analysis interrupted — link counts may be understated
```

Form re-enables immediately.

---

### State 6: `error`

Red error card with a brief shake. `role="alert"` announces it immediately to screen readers.

**HTTP error (status_code present):**
```
┌──────────────────────────────────────────────────┐
│  ✕  [ 404 ]  Page not found at example.com      │
└──────────────────────────────────────────────────┘
```

**Network error (status_code absent — DNS/timeout/TLS):**
```
┌──────────────────────────────────────────────────┐
│  ✕  Could not reach host: example.com            │
└──────────────────────────────────────────────────┘
```

```css
@keyframes shake {
  0%, 100% { transform: translateX(0);    }
  20%       { transform: translateX(-5px); }
  40%       { transform: translateX( 5px); }
  60%       { transform: translateX(-3px); }
  80%       { transform: translateX( 3px); }
}
.error-card { animation: shake 0.4s ease; }
```

---

## 6. Loading State Summary

| State | Duration | Key Animation | Aria |
|---|---|---|---|
| `idle` | Until submit | — | — |
| `submitting` | 100–300ms | `animate-spin` on button | `aria-busy="true"` |
| `streaming / fetching` | ~0.5–2s | `animate-ping` dot step 1 | `aria-live="polite"` |
| `streaming / parsing` | ~0.1–0.5s | `scale-in` checkmark + ping step 2 | `aria-live="polite"` |
| `streaming / checking_links` | Longest | Progress bar fill + shimmer skeletons | `role="progressbar"` + `aria-valuenow` |
| `rate_limited` | 5s countdown | None — amber card, live countdown | `role="status"` + `aria-live="polite"` |
| `done` | — | Staggered `fade-up` on cards | `role="region"` |
| `error` | — | `shake` on card | `role="alert"` |

---

## 7. SSE Hook — `useAnalysis.ts`

```typescript
'use client'

import { useReducer, useRef, useEffect } from 'react'
import type { AnalysisState, Action, AnalysisResult, SSEErrorPayload } from '@/types/analysis'

function reducer(state: AnalysisState, action: Action): AnalysisState {
  switch (action.type) {
    case 'SUBMIT':       return { status: 'submitting' }
    case 'STREAM_OPEN':  return { status: 'streaming', phase: 'fetching', checked: 0, total: 0 }
    case 'PHASE':
      if (state.status !== 'streaming') return state
      return { ...state, phase: action.payload }
    case 'PROGRESS':
      if (state.status !== 'streaming') return state
      return { ...state, checked: action.checked, total: action.total }
    case 'RESULT':
      return { status: 'done', result: action.payload, partial: action.payload.partial ?? false }
    case 'ERROR':
      return { status: 'error', message: action.message, statusCode: action.statusCode }
    case 'RATE_LIMITED':
      return { status: 'rate_limited', retryIn: 5, url: action.url }
    case 'RETRY_TICK':
      if (state.status !== 'rate_limited') return state
      return { ...state, retryIn: state.retryIn - 1 }
    case 'RESET':
      return { status: 'idle' }
    default:
      return state
  }
}

export function useAnalysis() {
  const [state, dispatch] = useReducer(reducer, { status: 'idle' })
  const esRef = useRef<EventSource | null>(null)
  // Track whether this submit is an auto-retry after a 429
  // If a retry also gets a 429 we give up rather than looping indefinitely
  const isRetryRef = useRef(false)

  const submit = async (url: string) => {
    esRef.current?.close()
    dispatch({ type: 'SUBMIT' })

    try {
      const res = await fetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url }),
      })

      if (res.status === 429) {
        if (isRetryRef.current) {
          // Second consecutive 429 — server still saturated, stop retrying
          isRetryRef.current = false
          const body = await res.json().catch(() => ({ error: 'Server still busy — try again later' }))
          dispatch({ type: 'ERROR', message: body.error, statusCode: 429 })
        } else {
          // First 429 — enter rate_limited state, countdown will auto-retry
          dispatch({ type: 'RATE_LIMITED', url })
        }
        return
      }

      // Reset retry flag on any non-429 response
      isRetryRef.current = false

      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: 'Request failed' }))
        dispatch({ type: 'ERROR', message: body.error, statusCode: res.status })
        return
      }

      const { job_id } = await res.json()
      dispatch({ type: 'STREAM_OPEN' })

      const es = new EventSource(`/api/analyze/stream?id=${job_id}`)
      esRef.current = es

      es.addEventListener('phase', (e: MessageEvent) => {
        dispatch({ type: 'PHASE', payload: JSON.parse(e.data).phase })
      })

      es.addEventListener('progress', (e: MessageEvent) => {
        const { checked, total } = JSON.parse(e.data)
        dispatch({ type: 'PROGRESS', checked, total })
      })

      es.addEventListener('result', (e: MessageEvent) => {
        dispatch({ type: 'RESULT', payload: JSON.parse(e.data) as AnalysisResult })
        es.close()
      })

      es.addEventListener('error', (e: Event) => {
        try {
          const data = JSON.parse((e as MessageEvent).data) as SSEErrorPayload
          dispatch({ type: 'ERROR', message: data.message, statusCode: data.status_code })
        } catch {
          dispatch({ type: 'ERROR', message: 'Connection to server lost' })
        }
        es.close()
      })

    } catch {
      isRetryRef.current = false
      dispatch({ type: 'ERROR', message: 'Failed to reach the analysis server' })
    }
  }

  // Countdown ticker — runs only while rate_limited
  useEffect(() => {
    if (state.status !== 'rate_limited') return
    const interval = setInterval(() => dispatch({ type: 'RETRY_TICK' }), 1000)
    return () => clearInterval(interval)
  }, [state.status])

  // Auto-retry when countdown hits 0
  useEffect(() => {
    if (state.status === 'rate_limited' && state.retryIn <= 0) {
      isRetryRef.current = true
      submit(state.url)
    }
  }, [state])

  useEffect(() => () => { esRef.current?.close() }, [])

  return {
    state,
    submit: (url: string) => { isRetryRef.current = false; submit(url) },
    reset: () => dispatch({ type: 'RESET' }),
    cancelRetry: () => dispatch({ type: 'RESET' }),
  }
}
```

---

## 8. Next.js 16 API Route Proxies

Proxying through Next.js means no CORS config on the Go server and `BACKEND_URL` stays server-side only (never in the browser bundle).

### POST Proxy — `app/api/analyze/route.ts`

```typescript
import { NextRequest, NextResponse } from 'next/server'

export async function POST(req: NextRequest) {
  const body = await req.json()

  const res = await fetch(`${process.env.BACKEND_URL}/analyze`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  const data = await res.json()
  return NextResponse.json(data, { status: res.status })
}
```

### SSE Stream Proxy — `app/api/analyze/stream/route.ts`

```typescript
import { NextRequest } from 'next/server'

export async function GET(req: NextRequest) {
  const id = req.nextUrl.searchParams.get('id')
  if (!id) return new Response('Missing id', { status: 400 })

  const upstream = await fetch(
    `${process.env.BACKEND_URL}/analyze/stream?id=${id}`,
    {
      headers: { Accept: 'text/event-stream' },
      cache: 'no-store',  // Disable Next.js fetch cache for this stream
    }
  )

  if (!upstream.ok) {
    return new Response('Stream not found', { status: upstream.status })
  }

  // Pipe raw SSE body directly — no buffering, no parsing, all event types pass through
  return new Response(upstream.body, {
    headers: {
      'Content-Type':  'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection':    'keep-alive',
    },
  })
}
```

### Environment Variables

```bash
# .env.local  (never committed)
BACKEND_URL=http://localhost:8080

# Production — set in deployment environment, not in code
BACKEND_URL=https://api.your-service.com
```

### `app/layout.tsx`

```tsx
import type { Metadata } from 'next'
import { IBM_Plex_Mono, DM_Sans } from 'next/font/google'
import './globals.css'

const mono = IBM_Plex_Mono({
  subsets: ['latin'],
  weight: ['400', '600'],
  variable: '--font-mono',
  display: 'swap',
})

const sans = DM_Sans({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'Page Insight',
  description: 'Analyse any web page — headings, links, HTML version, login form detection.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${mono.variable} ${sans.variable}`}>
      <body className="bg-[var(--bg-base)] text-[var(--text-primary)] font-sans antialiased">
        {children}
      </body>
    </html>
  )
}
```

---

## 9. Component Specifications

### `<UrlForm />`

| Prop | Type | Description |
|---|---|---|
| `onSubmit` | `(url: string) => void` | Called with validated URL |
| `disabled` | `boolean` | Locks form during in-flight job |
| `status` | `AnalysisState['status']` | Drives button content swap |

Validation: must start with `http://` or `https://`, non-empty host. Checked on blur + submit. Error shown inline below input — not a toast.

---

### `<ProgressPanel />`

| Prop | Type | Description |
|---|---|---|
| `phase` | `Phase` | Current active step |
| `checked` | `number` | Links checked so far |
| `total` | `number` | Total unique links |

`aria-live="polite"` on the stepper container.

---

### `<ErrorCard />`

| Prop | Type | Description |
|---|---|---|
| `message` | `string` | Error message from backend |
| `statusCode` | `number \| undefined` | HTTP code — renders badge only if defined |

Never render `statusCode` of `0`. The `status_code` field is absent from the SSE payload for network failures — check for `undefined`, not falsiness.

---

### `<RateLimitedCard />`

| Prop | Type | Description |
|---|---|---|
| `retryIn` | `number` | Seconds remaining on countdown |
| `onCancel` | `() => void` | Resets to idle without retrying |

Visually amber (`--accent-warm`), never red. No shake animation.

```
┌──────────────────────────────────────────────────┐
│  ⏱  Server busy — retrying in 5s...             │
│                              [ Cancel ]          │
└──────────────────────────────────────────────────┘
```

```tsx
<div role="status" className="border border-[--accent-warm] rounded-lg p-4 ...">
  <span className="text-[--accent-warm]">⏱</span>
  <span>
    Server busy — retrying in{' '}
    <span aria-live="polite" aria-atomic="true" className="font-mono tabular-nums">
      {retryIn}s
    </span>
    ...
  </span>
  <button onClick={onCancel}>Cancel</button>
</div>
```

`aria-live="polite"` is scoped tightly to the countdown number only — not the whole card — so screen readers say `"4 seconds"` each tick without re-reading the full message.

---

### `<HeadingsChart />`

Horizontal bars for h1–h6. Only renders levels with count > 0.

```
h1  ████████████████  4
h2  ████████          2
h4  ████              1
    (h3, h5, h6 omitted — zero counts)
```

Bars proportional to the max count. CSS `width` transition on mount. `tabular-nums` on count labels.

---

### `<LinksBreakdown />`

2×2 grid of stat tiles (single column on mobile):

| Tile | Colour | Tooltip |
|---|---|---|
| Internal Links | `--text-primary` | "Links pointing to the same domain" |
| External Links | `--accent` | "Links pointing to other domains" |
| Inaccessible | `--danger` | "Returned an error or couldn't be reached" |
| Unverified | `--accent-warm` | "Server returned 401/403/429 — access denied, not confirmed broken" |

The unverified tooltip is mandatory.

---

### `<LoginFormBadge />`

```tsx
has_login_form
  ? <Badge variant="success">⚑  Login form detected</Badge>
  : <Badge variant="neutral">   No login form</Badge>
```

Always text + colour. Never colour alone.

---

## 10. Accessibility Checklist

- [ ] URL input has a visible `<label>` — not placeholder-only
- [ ] `aria-busy="true"` on form during submitting
- [ ] `aria-disabled="true"` on all form elements during in-flight job
- [ ] Progress stepper uses `aria-live="polite"`
- [ ] Progress bar: `role="progressbar"`, `aria-valuenow`, `aria-valuemin="0"`, `aria-valuemax`
- [ ] Error card: `role="alert"` — announced immediately
- [ ] Rate limited card: `role="status"` + countdown `aria-live="polite"` scoped to number only
- [ ] Partial warning: `role="status"`
- [ ] Skeleton placeholders: `aria-hidden="true"`
- [ ] All colour distinctions include text labels (badges, tiles, bars)
- [ ] Focus returns to URL input after analysis completes or errors
- [ ] Min 4.5:1 contrast on all text
- [ ] Full keyboard navigation, visible focus rings

---

## 11. Implementation Order

```
1.  Scaffold — pnpm create next-app@latest, verify Node ≥ 20.9
2.  next.config.ts — reactCompiler: true
3.  globals.css — CSS variables + all keyframes
4.  types/analysis.ts — TypeScript interfaces
5.  app/api/analyze/route.ts — POST proxy
6.  app/api/analyze/stream/route.ts — SSE stream proxy
7.  hooks/useAnalysis.ts — state machine + SSE lifecycle
8.  ui/ — Skeleton, Badge, StatTile, Tooltip, ProgressBar
9.  UrlForm.tsx — input, validation, submitting spinner
10. ProgressPanel.tsx — stepper all 3 phases, progress bar
11. ErrorCard.tsx — shake animation, conditional status badge
12. RateLimitedCard.tsx — amber card, live countdown, cancel button
12. PageMetaCard, HeadingsChart, LinksBreakdown, LoginFormBadge
13. ResultPanel.tsx — staggered fade-up, partial warning, skeleton placeholders
14. AnalysisPage.tsx — wire everything to useAnalysis
15. app/page.tsx — Server Component shell
16. Accessibility pass — aria attrs, focus management, contrast
17. Tests — reducer, UrlForm validation, ErrorCard variants, RateLimitedCard countdown + cancel
```

---

*End of document. Backend contract: web-analyzer-build-plan v1.0. Next.js: 16.2.1. React: 19.2. Node.js minimum: 20.9.*