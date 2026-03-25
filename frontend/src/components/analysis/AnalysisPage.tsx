'use client'

import { useAnalysis } from '@/hooks/useAnalysis'
import { UrlForm } from './UrlForm'
import { ProgressPanel } from './ProgressPanel'
import { ErrorCard } from './ErrorCard'
import { RateLimitedCard } from './RateLimitedCard'
import { ResultPanel, ResultSkeleton } from './ResultPanel'

export function AnalysisPage() {
  const { state, submit, reset, cancelRetry } = useAnalysis()

  const isDisabled =
    state.status === 'submitting' ||
    state.status === 'streaming' ||
    state.status === 'rate_limited'

  const handleSubmit = (url: string) => {
    if (state.status === 'done' || state.status === 'error') {
      reset()
    }
    submit(url)
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-3xl flex-col gap-8 px-4 py-12">
      <header className="text-center">
        <h1 className="font-mono text-3xl font-semibold text-[var(--accent)]">
          Page Insight
        </h1>
        <p className="mt-2 text-sm text-[var(--text-muted)]">
          Analyse any web page — headings, links, HTML version, login form detection.
        </p>
      </header>

      <UrlForm
        onSubmit={handleSubmit}
        disabled={isDisabled}
        status={state.status}
      />

      {state.status === 'streaming' && (
        <>
          <ProgressPanel
            phase={state.phase}
            checked={state.checked}
            total={state.total}
          />
          {state.phase === 'checking_links' && <ResultSkeleton />}
        </>
      )}

      {state.status === 'rate_limited' && (
        <RateLimitedCard retryIn={state.retryIn} onCancel={cancelRetry} />
      )}

      {state.status === 'error' && (
        <ErrorCard message={state.message} statusCode={state.statusCode} />
      )}

      {state.status === 'done' && (
        <ResultPanel result={state.result} partial={state.partial} />
      )}
    </main>
  )
}
