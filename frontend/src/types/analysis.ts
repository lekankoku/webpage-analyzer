export type Phase = 'fetching' | 'parsing' | 'checking_links'

export type AnalysisState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'streaming'; phase: Phase; checked: number; total: number }
  | { status: 'done'; result: AnalysisResult; partial: boolean }
  | { status: 'error'; message: string; statusCode?: number }

export type Action =
  | { type: 'SUBMIT' }
  | { type: 'STREAM_OPEN' }
  | { type: 'PHASE'; payload: Phase }
  | { type: 'PROGRESS'; checked: number; total: number }
  | { type: 'RESULT'; payload: AnalysisResult }
  | { type: 'ERROR'; message: string; statusCode?: number }
  | { type: 'RESET' }

export interface AnalysisResult {
  html_version: string
  title: string
  headings: Partial<Record<'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6', number>>
  internal_links: number
  external_links: number
  inaccessible_links: number
  unverified_links: number
  has_login_form: boolean
  partial?: boolean
}

export interface SSEErrorPayload {
  message: string
  job_id: string
  status_code?: number
}
