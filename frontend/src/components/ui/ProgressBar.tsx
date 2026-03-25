export function ProgressBar({
  checked,
  total,
}: {
  checked: number
  total: number
}) {
  const pct = total > 0 ? (checked / total) * 100 : 0

  return (
    <div className="space-y-2">
      <div className="h-1 rounded-full bg-[var(--border)] overflow-hidden">
        <div
          className="h-full rounded-full bg-[var(--accent)] transition-[width] duration-300 ease-out"
          style={{ width: `${pct}%` }}
          role="progressbar"
          aria-valuenow={checked}
          aria-valuemin={0}
          aria-valuemax={total}
        />
      </div>
      <span className="font-mono text-sm tabular-nums text-[var(--text-muted)]">
        {checked} / {total} links
      </span>
    </div>
  )
}
