import { describe, it, expect } from 'vitest'
import { reducer } from '../useAnalysis'
import type { AnalysisState, AnalysisResult } from '@/types/analysis'

const idleState: AnalysisState = { status: 'idle' }
const submittingState: AnalysisState = { status: 'submitting' }
const streamingState: AnalysisState = {
  status: 'streaming',
  phase: 'fetching',
  checked: 0,
  total: 0,
}

const mockResult: AnalysisResult = {
  html_version: 'HTML5',
  title: 'Test Page',
  headings: { h1: 1, h2: 3 },
  internal_links: 10,
  external_links: 5,
  inaccessible_links: 1,
  unverified_links: 2,
  has_login_form: false,
}

describe('reducer', () => {
  describe('SUBMIT', () => {
    it('transitions from idle to submitting', () => {
      const result = reducer(idleState, { type: 'SUBMIT' })
      expect(result).toEqual({ status: 'submitting' })
    })
  })

  describe('STREAM_OPEN', () => {
    it('transitions from submitting to streaming with fetching phase', () => {
      const result = reducer(submittingState, { type: 'STREAM_OPEN' })
      expect(result).toEqual({
        status: 'streaming',
        phase: 'fetching',
        checked: 0,
        total: 0,
      })
    })
  })

  describe('PHASE', () => {
    it('updates phase when streaming', () => {
      const result = reducer(streamingState, { type: 'PHASE', payload: 'parsing' })
      expect(result).toEqual({
        status: 'streaming',
        phase: 'parsing',
        checked: 0,
        total: 0,
      })
    })

    it('ignores PHASE when not streaming', () => {
      const result = reducer(idleState, { type: 'PHASE', payload: 'parsing' })
      expect(result).toEqual(idleState)
    })
  })

  describe('PROGRESS', () => {
    it('updates checked and total when streaming', () => {
      const state: AnalysisState = {
        status: 'streaming',
        phase: 'checking_links',
        checked: 0,
        total: 100,
      }
      const result = reducer(state, { type: 'PROGRESS', checked: 47, total: 100 })
      expect(result).toEqual({
        status: 'streaming',
        phase: 'checking_links',
        checked: 47,
        total: 100,
      })
    })

    it('ignores PROGRESS when not streaming', () => {
      const result = reducer(idleState, { type: 'PROGRESS', checked: 10, total: 50 })
      expect(result).toEqual(idleState)
    })
  })

  describe('RESULT', () => {
    it('transitions to done with result', () => {
      const result = reducer(streamingState, { type: 'RESULT', payload: mockResult })
      expect(result).toEqual({
        status: 'done',
        result: mockResult,
        partial: false,
      })
    })

    it('sets partial to true when result has partial flag', () => {
      const partialResult = { ...mockResult, partial: true }
      const result = reducer(streamingState, { type: 'RESULT', payload: partialResult })
      expect(result).toEqual({
        status: 'done',
        result: partialResult,
        partial: true,
      })
    })
  })

  describe('ERROR', () => {
    it('transitions to error with message', () => {
      const result = reducer(streamingState, {
        type: 'ERROR',
        message: 'Page not found',
        statusCode: 404,
      })
      expect(result).toEqual({
        status: 'error',
        message: 'Page not found',
        statusCode: 404,
      })
    })

    it('handles error without statusCode', () => {
      const result = reducer(submittingState, {
        type: 'ERROR',
        message: 'Connection lost',
      })
      expect(result).toEqual({
        status: 'error',
        message: 'Connection lost',
        statusCode: undefined,
      })
    })

    it('can transition to error from submitting', () => {
      const result = reducer(submittingState, {
        type: 'ERROR',
        message: 'Request failed',
        statusCode: 500,
      })
      expect(result.status).toBe('error')
    })
  })

  describe('RESET', () => {
    it('transitions from done to idle', () => {
      const doneState: AnalysisState = {
        status: 'done',
        result: mockResult,
        partial: false,
      }
      const result = reducer(doneState, { type: 'RESET' })
      expect(result).toEqual({ status: 'idle' })
    })

    it('transitions from error to idle', () => {
      const errorState: AnalysisState = {
        status: 'error',
        message: 'something broke',
      }
      const result = reducer(errorState, { type: 'RESET' })
      expect(result).toEqual({ status: 'idle' })
    })
  })

  describe('unknown action', () => {
    it('returns current state for unknown action type', () => {
      // @ts-expect-error testing unknown action
      const result = reducer(idleState, { type: 'UNKNOWN' })
      expect(result).toEqual(idleState)
    })
  })
})
