export function ErrorCard({
  message,
  statusCode,
}: {
  message: string
  statusCode?: number
}) {
  return (
    <div
      role="alert"
      className="error-card flex items-center gap-3 rounded-lg border border-[var(--danger)]/30 bg-[var(--danger)]/10 px-4 py-3"
    >
      <span className="text-[var(--danger)] text-lg">✕</span>
      {statusCode && statusCode > 0 && (
        <span className="rounded bg-[var(--danger)]/20 px-2 py-0.5 font-mono text-sm text-[var(--danger)]">
          {statusCode}
        </span>
      )}
      <p className="text-sm text-[var(--danger)]">{message}</p>
    </div>
  )
}
