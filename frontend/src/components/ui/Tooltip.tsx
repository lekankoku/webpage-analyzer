'use client'

import { useState } from 'react'

export function Tooltip({
  content,
  children,
}: {
  content: string
  children: React.ReactNode
}) {
  const [visible, setVisible] = useState(false)

  return (
    <div
      className="relative"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
      onFocus={() => setVisible(true)}
      onBlur={() => setVisible(false)}
    >
      {children}
      {visible && (
        <div
          role="tooltip"
          className="absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 whitespace-nowrap rounded-md border border-[var(--border)] bg-[var(--bg-elevated)] px-3 py-1.5 text-xs text-[var(--text-primary)] shadow-lg"
        >
          {content}
        </div>
      )}
    </div>
  )
}
