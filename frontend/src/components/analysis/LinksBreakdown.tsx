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
          label="Internal (all discovered)"
          value={internalLinks}
          color="var(--text-primary)"
          tooltip="All discovered links pointing to the same domain"
        />
        <StatTile
          label="External (all discovered)"
          value={externalLinks}
          color="var(--accent)"
          tooltip="All discovered links pointing to other domains"
        />
        <StatTile
          label="Inaccessible (subset)"
          value={inaccessibleLinks}
          color="var(--danger)"
          tooltip="Subset of discovered links that returned an error or couldn't be reached"
        />
        <StatTile
          label="Unverified (subset)"
          value={unverifiedLinks}
          color="var(--accent-warm)"
          tooltip="Subset of discovered links where server returned 401/403/429 (access denied)"
        />
      </div>
      <p className="mt-3 text-xs text-[var(--text-muted)]">
        Inaccessible and unverified are subsets of internal/external discovered links.
      </p>
    </div>
  )
}
