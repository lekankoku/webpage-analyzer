'use client'

import type { Phase } from '@/types/analysis'
import { ProgressBar } from '@/components/ui/ProgressBar'

const steps: { phase: Phase; label: string }[] = [
  { phase: 'fetching', label: 'Fetching page' },
  { phase: 'parsing', label: 'Parsing HTML' },
  { phase: 'checking_links', label: 'Checking links' },
]

function phaseIndex(phase: Phase): number {
  return steps.findIndex((s) => s.phase === phase)
}

function StepDot({ state }: { state: 'done' | 'active' | 'pending' }) {
  if (state === 'done') {
    return (
      <span className="step-check flex size-6 items-center justify-center rounded-full bg-[var(--success)] text-white text-xs">
        ✓
      </span>
    )
  }

  if (state === 'active') {
    return (
      <span className="relative flex size-6 items-center justify-center">
        <span className="absolute size-6 animate-ping rounded-full bg-[var(--accent)] opacity-60" />
        <span className="relative size-3 rounded-full bg-[var(--accent)]" />
      </span>
    )
  }

  return <span className="size-3 rounded-full bg-[var(--border)]" />
}

export function ProgressPanel({
  phase,
  checked,
  total,
}: {
  phase: Phase
  checked: number
  total: number
}) {
  const activeIdx = phaseIndex(phase)

  return (
    <div className="fade-slide-in space-y-4" aria-live="polite">
      <div className="flex items-center gap-3">
        {steps.map((step, i) => {
          const state = i < activeIdx ? 'done' : i === activeIdx ? 'active' : 'pending'
          return (
            <div key={step.phase} className="flex items-center gap-3">
              {i > 0 && (
                <div
                  className={`h-px w-8 ${
                    i <= activeIdx ? 'bg-[var(--accent)]' : 'bg-[var(--border)]'
                  }`}
                />
              )}
              <StepDot state={state} />
            </div>
          )
        })}
      </div>

      <div className="space-y-1.5">
        {steps.map((step, i) => {
          const isDone = i < activeIdx
          const isActive = i === activeIdx
          return (
            <p
              key={step.phase}
              className={`text-sm ${
                isDone
                  ? 'text-[var(--success)]'
                  : isActive
                    ? 'text-[var(--text-primary)]'
                    : 'text-[var(--text-muted)]'
              }`}
            >
              {isDone && <span className="mr-1.5">✓</span>}
              {isActive && <span className="mr-1.5">●</span>}
              {isDone ? step.label.replace(/ing/, 'ed').replace('Fetching page', 'Fetched').replace('Parsing HTML', 'Parsed').replace('Checking links', 'Checked') : `${step.label}${isActive ? '...' : ''}`}
            </p>
          )
        })}
      </div>

      {phase === 'checking_links' && (
        <ProgressBar checked={checked} total={total} />
      )}
    </div>
  )
}
