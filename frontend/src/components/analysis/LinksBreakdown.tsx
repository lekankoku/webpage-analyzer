import { StatTile } from '@/components/ui/StatTile'

export function LinksBreakdown({
  internalLinks,
  externalLinks,
  inaccessibleLinks,
  unverifiedLinks,
}: {
  internalLinks: number
  externalLinks: number
  inaccessibleLinks: number
  unverifiedLinks: number
}) {
  return (
    <div className="result-card rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-5">
      <h3 className="mb-3 font-mono text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
        Links
      </h3>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <StatTile
          label="Internal Links"
          value={internalLinks}
          color="var(--text-primary)"
          tooltip="Links pointing to the same domain"
        />
        <StatTile
          label="External Links"
          value={externalLinks}
          color="var(--accent)"
          tooltip="Links pointing to other domains"
        />
        <StatTile
          label="Inaccessible"
          value={inaccessibleLinks}
          color="var(--danger)"
          tooltip="Returned an error or couldn't be reached"
        />
        <StatTile
          label="Unverified"
          value={unverifiedLinks}
          color="var(--accent-warm)"
          tooltip="Server returned 401/403/429 — access denied, not confirmed broken"
        />
      </div>
    </div>
  )
}
