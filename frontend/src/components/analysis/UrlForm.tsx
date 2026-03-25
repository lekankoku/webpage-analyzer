'use client'

import { useState, useRef, useEffect } from 'react'
import { isValidUrl } from '@/utils/urlValidation'
import type { AnalysisState } from '@/types/analysis'

function SpinnerIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  )
}

export function UrlForm({
  onSubmit,
  disabled,
  status,
}: {
  onSubmit: (url: string) => void
  disabled: boolean
  status: AnalysisState['status']
}) {
  const [url, setUrl] = useState('')
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  // Return focus to input after analysis completes or errors
  useEffect(() => {
    if (status === 'done' || status === 'error') {
      inputRef.current?.focus()
    }
  }, [status])

  const validate = (value: string) => {
    if (!value.trim()) {
      setError('Please enter a URL')
      return false
    }
    if (!isValidUrl(value)) {
      setError('URL must start with http:// or https://')
      return false
    }
    setError('')
    return true
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (validate(url)) {
      onSubmit(url)
    }
  }

  const handleBlur = () => {
    if (url.trim()) validate(url)
  }

  return (
    <form onSubmit={handleSubmit} aria-busy={status === 'submitting'}>
      <div className="flex flex-col gap-2">
        <label htmlFor="url-input" className="text-sm font-medium text-[var(--text-muted)]">
          Page URL
        </label>
        <div className="flex gap-3">
          <div className="relative flex-1">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="size-5">
                <path fillRule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clipRule="evenodd" />
              </svg>
            </span>
            <input
              ref={inputRef}
              id="url-input"
              type="text"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value)
                if (error) setError('')
              }}
              onBlur={handleBlur}
              placeholder="https://example.com"
              disabled={disabled}
              aria-disabled={disabled}
              aria-invalid={!!error}
              aria-describedby={error ? 'url-error' : undefined}
              className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-elevated)] py-3 pl-10 pr-4 text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none focus:ring-1 focus:ring-[var(--accent)] disabled:opacity-50"
            />
          </div>
          <button
            type="submit"
            disabled={disabled}
            aria-disabled={disabled}
            className="min-w-[120px] rounded-lg bg-[var(--accent)] px-6 py-3 font-semibold text-white transition-opacity hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-[var(--accent)] focus:ring-offset-2 focus:ring-offset-[var(--bg-base)] disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {status === 'submitting' ? (
              <span className="flex items-center justify-center gap-2">
                <SpinnerIcon className="size-4 animate-spin" />
                Sending
              </span>
            ) : (
              'Analyse'
            )}
          </button>
        </div>
        {error && (
          <p id="url-error" className="text-sm text-[var(--danger)]" role="alert">
            {error}
          </p>
        )}
      </div>
    </form>
  )
}
