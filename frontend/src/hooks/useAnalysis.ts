'use client'

import { useReducer, useRef, useEffect, useCallback } from 'react'
import type { AnalysisState, Action, AnalysisResult, SSEErrorPayload } from '@/types/analysis'

export function reducer(state: AnalysisState, action: Action): AnalysisState {
  switch (action.type) {
    case 'SUBMIT':
      return { status: 'submitting' }
    case 'STREAM_OPEN':
      return { status: 'streaming', phase: 'fetching', checked: 0, total: 0 }
    case 'PHASE':
      if (state.status !== 'streaming') return state
      return { ...state, phase: action.payload }
    case 'PROGRESS':
      if (state.status !== 'streaming') return state
      return { ...state, checked: action.checked, total: action.total }
    case 'RESULT':
      return { status: 'done', result: action.payload, partial: action.payload.partial ?? false }
    case 'ERROR':
      return { status: 'error', message: action.message, statusCode: action.statusCode }
    case 'RESET':
      return { status: 'idle' }
    default:
      return state
  }
}

export function useAnalysis() {
  const [state, dispatch] = useReducer(reducer, { status: 'idle' })
  const esRef = useRef<EventSource | null>(null)

  const submit = useCallback(async (url: string) => {
    esRef.current?.close()
    dispatch({ type: 'SUBMIT' })

    try {
      const res = await fetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url }),
      })

      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: 'Request failed' }))
        dispatch({ type: 'ERROR', message: body.error, statusCode: res.status })
        return
      }

      const { job_id } = await res.json()
      dispatch({ type: 'STREAM_OPEN' })

      const es = new EventSource(`/api/analyze/stream?id=${job_id}`)
      esRef.current = es

      es.addEventListener('phase', (e: MessageEvent) => {
        dispatch({ type: 'PHASE', payload: JSON.parse(e.data).phase })
      })

      es.addEventListener('progress', (e: MessageEvent) => {
        const { checked, total } = JSON.parse(e.data)
        dispatch({ type: 'PROGRESS', checked, total })
      })

      es.addEventListener('result', (e: MessageEvent) => {
        dispatch({ type: 'RESULT', payload: JSON.parse(e.data) as AnalysisResult })
        es.close()
      })

      es.addEventListener('error', (e: Event) => {
        try {
          const data = JSON.parse((e as MessageEvent).data) as SSEErrorPayload
          dispatch({ type: 'ERROR', message: data.message, statusCode: data.status_code })
        } catch {
          dispatch({ type: 'ERROR', message: 'Connection to server lost' })
        }
        es.close()
      })
    } catch {
      dispatch({ type: 'ERROR', message: 'Failed to reach the analysis server' })
    }
  }, [])

  useEffect(() => () => { esRef.current?.close() }, [])

  const reset = useCallback(() => dispatch({ type: 'RESET' }), [])

  return { state, submit, reset }
}
