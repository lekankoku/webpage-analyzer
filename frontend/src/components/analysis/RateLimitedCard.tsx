export function RateLimitedCard({
  retryIn,
  onCancel,
}: {
  retryIn: number
  onCancel: () => void
}) {
  return (
    <div
      role="status"
      className="flex flex-col gap-3 rounded-lg border border-[var(--accent-warm)]/40 bg-[var(--accent-warm)]/10 px-4 py-3"
    >
      <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--accent-warm)]">
        <span aria-hidden="true">⏱</span>
        <span>
          Server busy — retrying in{' '}
          <span aria-live="polite" aria-atomic="true" className="font-mono tabular-nums">
            {retryIn} seconds
          </span>
        </span>
      </div>
      <div>
        <button
          type="button"
          onClick={onCancel}
          className="rounded-md border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-1.5 text-sm text-[var(--text-primary)] hover:bg-[var(--bg-surface)] focus:outline-none focus:ring-2 focus:ring-[var(--accent-warm)] focus:ring-offset-2 focus:ring-offset-[var(--bg-base)]"
        >
          Cancel
        </button>
      </div>
    </div>
  )
}
