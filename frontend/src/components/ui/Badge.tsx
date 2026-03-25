const variants = {
  success: 'bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/30',
  danger: 'bg-[var(--danger)]/15 text-[var(--danger)] border-[var(--danger)]/30',
  warning: 'bg-[var(--accent-warm)]/15 text-[var(--accent-warm)] border-[var(--accent-warm)]/30',
  neutral: 'bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border)]',
} as const

type BadgeVariant = keyof typeof variants

export function Badge({
  variant = 'neutral',
  children,
}: {
  variant?: BadgeVariant
  children: React.ReactNode
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium ${variants[variant]}`}
    >
      {children}
    </span>
  )
}
