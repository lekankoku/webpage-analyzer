const HEADING_LEVELS = ['h1', 'h2', 'h3', 'h4', 'h5', 'h6'] as const

export function HeadingsChart({
  headings,
}: {
  headings: Partial<Record<(typeof HEADING_LEVELS)[number], number>>
}) {
  const entries = HEADING_LEVELS
    .filter((h) => (headings[h] ?? 0) > 0)
    .map((h) => ({ level: h, count: headings[h]! }))

  const max = Math.max(...entries.map((e) => e.count), 1)

  if (entries.length === 0) {
    return (
      <div className="result-card rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-5">
        <h3 className="mb-3 font-mono text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
          Headings
        </h3>
        <p className="text-sm text-[var(--text-muted)]">No headings found</p>
      </div>
    )
  }

  return (
    <div className="result-card rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-5">
      <h3 className="mb-3 font-mono text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
        Headings
      </h3>
      <div className="space-y-2">
        {entries.map(({ level, count }) => (
          <div key={level} className="flex items-center gap-3">
            <span className="w-6 shrink-0 font-mono text-xs text-[var(--text-muted)]">
              {level}
            </span>
            <div className="h-4 flex-1 overflow-hidden rounded bg-[var(--bg-elevated)]">
              <div
                className="h-full rounded bg-[var(--accent)] transition-[width] duration-500 ease-out"
                style={{ width: `${(count / max) * 100}%` }}
              />
            </div>
            <span className="w-8 text-right font-mono text-sm tabular-nums text-[var(--text-muted)]">
              {count}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
