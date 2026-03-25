'use client'

import { useReducer, useRef, useEffect, useCallback } from 'react'
import type { AnalysisState, Action, AnalysisResult, SSEErrorPayload } from '@/types/analysis'

const RATE_LIMIT_RETRY_SECONDS = 5

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
    case 'RATE_LIMITED':
      return {
        status: 'rate_limited',
        retryIn: RATE_LIMIT_RETRY_SECONDS,
        url: action.url,
      }
    case 'RETRY_TICK':
      if (state.status !== 'rate_limited' || state.retryIn <= 0) return state
      return { ...state, retryIn: state.retryIn - 1 }
    case 'RESET':
      return { status: 'idle' }
    default:
      return state
  }
}

export function useAnalysis() {
  const [state, dispatch] = useReducer(reducer, { status: 'idle' })
  const esRef = useRef<EventSource | null>(null)
  const isRetryRef = useRef(false)
  const autoRetryGuardRef = useRef(false)

  const submitInternal = useCallback(async (url: string) => {
    esRef.current?.close()
    dispatch({ type: 'SUBMIT' })

    try {
      const res = await fetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url }),
      })

      if (res.status === 429) {
        if (isRetryRef.current) {
          isRetryRef.current = false
          const body = await res.json().catch(() => ({
            error: 'Server still busy — try again later',
          }))
          dispatch({ type: 'ERROR', message: body.error, statusCode: 429 })
        } else {
          dispatch({ type: 'RATE_LIMITED', url })
        }
        return
      }

      isRetryRef.current = false

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
      isRetryRef.current = false
      dispatch({ type: 'ERROR', message: 'Failed to reach the analysis server' })
    }
  }, [])

  const submit = useCallback(
    (url: string) => {
      isRetryRef.current = false
      void submitInternal(url)
    },
    [submitInternal]
  )

  useEffect(() => {
    if (state.status !== 'rate_limited') return
    const interval = setInterval(() => dispatch({ type: 'RETRY_TICK' }), 1000)
    return () => clearInterval(interval)
  }, [state.status])

  useEffect(() => {
    if (state.status !== 'rate_limited') {
      autoRetryGuardRef.current = false
      return
    }
    if (state.retryIn !== 0 || autoRetryGuardRef.current) return
    autoRetryGuardRef.current = true
    isRetryRef.current = true
    void submitInternal(state.url)
  }, [state.status, state.status === 'rate_limited' ? state.retryIn : -1, state.status === 'rate_limited' ? state.url : '', submitInternal])

  useEffect(() => () => { esRef.current?.close() }, [])

  const reset = useCallback(() => dispatch({ type: 'RESET' }), [])

  const cancelRetry = useCallback(() => dispatch({ type: 'RESET' }), [])

  return { state, submit, reset, cancelRetry }
}
