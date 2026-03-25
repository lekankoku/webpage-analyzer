import { Tooltip } from './Tooltip'

export function StatTile({
  label,
  value,
  color = 'var(--text-primary)',
  tooltip,
}: {
  label: string
  value: number
  color?: string
  tooltip?: string
}) {
  const tile = (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-4">
      <p
        className="font-mono text-3xl font-semibold tabular-nums"
        style={{ color }}
      >
        {value}
      </p>
      <p className="mt-1 text-sm text-[var(--text-muted)]">{label}</p>
    </div>
  )

  if (tooltip) {
    return <Tooltip content={tooltip}>{tile}</Tooltip>
  }

  return tile
}
