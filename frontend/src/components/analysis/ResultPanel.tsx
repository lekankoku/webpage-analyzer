import type { AnalysisResult } from '@/types/analysis'
import { Skeleton } from '@/components/ui/Skeleton'
import { PageMetaCard } from './PageMetaCard'
import { HeadingsChart } from './HeadingsChart'
import { LinksBreakdown } from './LinksBreakdown'
import { LoginFormBadge } from './LoginFormBadge'

export function ResultPanel({
  result,
  partial,
}: {
  result: AnalysisResult
  partial: boolean
}) {
  return (
    <div role="region" aria-label="Analysis results">
      {partial && (
        <div
          role="status"
          className="mb-4 flex items-center gap-2 rounded-lg border border-[var(--accent-warm)]/30 bg-[var(--accent-warm)]/10 px-4 py-3 text-sm text-[var(--accent-warm)]"
        >
          <span>⚠</span>
          <span>Analysis interrupted — link counts may be understated</span>
        </div>
      )}
      <div className="grid gap-4 md:grid-cols-2">
        <PageMetaCard htmlVersion={result.html_version} title={result.title} />
        <HeadingsChart headings={result.headings} />
        <LinksBreakdown
          internalLinks={result.internal_links}
          externalLinks={result.external_links}
          inaccessibleLinks={result.inaccessible_links}
          unverifiedLinks={result.unverified_links}
        />
        <LoginFormBadge detected={result.has_login_form} />
      </div>
    </div>
  )
}

export function ResultSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2" aria-hidden="true">
      <Skeleton className="h-36" />
      <Skeleton className="h-36" />
      <Skeleton className="h-36" />
      <Skeleton className="h-36" />
    </div>
  )
}
