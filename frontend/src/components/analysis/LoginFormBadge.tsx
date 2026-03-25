import { Badge } from '@/components/ui/Badge'

export function LoginFormBadge({ detected }: { detected: boolean }) {
  return (
    <div className="result-card rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-5">
      <h3 className="mb-3 font-mono text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
        Login Form
      </h3>
      {detected ? (
        <Badge variant="success">⚑ Login form detected</Badge>
      ) : (
        <Badge variant="neutral">No login form</Badge>
      )}
    </div>
  )
}
