export function Skeleton({ className = '' }: { className?: string }) {
  return (
    <div
      className={`rounded-md skeleton-shimmer ${className}`}
      aria-hidden="true"
    />
  )
}
