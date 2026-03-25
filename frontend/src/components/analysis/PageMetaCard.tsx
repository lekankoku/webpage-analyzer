export function PageMetaCard({
  htmlVersion,
  title,
}: {
  htmlVersion: string
  title: string
}) {
  return (
    <div className="result-card rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-5">
      <h3 className="mb-3 font-mono text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
        Page Info
      </h3>
      <dl className="space-y-2">
        <div className="flex items-baseline justify-between">
          <dt className="text-sm text-[var(--text-muted)]">HTML Version</dt>
          <dd className="font-mono text-sm">{htmlVersion}</dd>
        </div>
        <div className="flex items-baseline justify-between gap-4">
          <dt className="text-sm text-[var(--text-muted)] shrink-0">Title</dt>
          <dd className="text-sm text-right truncate" title={title}>{title}</dd>
        </div>
      </dl>
    </div>
  )
}
