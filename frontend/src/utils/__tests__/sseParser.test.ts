import { describe, it, expect } from 'vitest'
import { parseSSEEvent } from '../sseParser'

describe('parseSSEEvent', () => {
  it('parses a phase event', () => {
    const data = JSON.stringify({ phase: 'fetching', message: 'Fetching page...' })
    const result = parseSSEEvent('phase', data)
    expect(result).toEqual({
      type: 'phase',
      data: { phase: 'fetching', message: 'Fetching page...' },
    })
  })

  it('parses a progress event', () => {
    const data = JSON.stringify({ checked: 47, total: 200 })
    const result = parseSSEEvent('progress', data)
    expect(result).toEqual({
      type: 'progress',
      data: { checked: 47, total: 200 },
    })
  })

  it('parses a result event', () => {
    const payload = {
      html_version: 'HTML5',
      title: 'Example',
      headings: { h1: 1, h2: 3 },
      internal_links: 10,
      external_links: 5,
      inaccessible_links: 1,
      unverified_links: 2,
      has_login_form: false,
    }
    const result = parseSSEEvent('result', JSON.stringify(payload))
    expect(result).toEqual({ type: 'result', data: payload })
  })

  it('parses an error event with status_code', () => {
    const data = JSON.stringify({
      message: 'Page not found',
      job_id: 'abc-123',
      status_code: 404,
    })
    const result = parseSSEEvent('error', data)
    expect(result).toEqual({
      type: 'error',
      data: { message: 'Page not found', job_id: 'abc-123', status_code: 404 },
    })
  })

  it('parses an error event without status_code (network failure)', () => {
    const data = JSON.stringify({
      message: 'DNS resolution failed',
      job_id: 'abc-456',
    })
    const result = parseSSEEvent('error', data)
    expect(result).toEqual({
      type: 'error',
      data: { message: 'DNS resolution failed', job_id: 'abc-456' },
    })
  })

  it('returns null for malformed JSON', () => {
    const result = parseSSEEvent('phase', 'not-json{{{')
    expect(result).toBeNull()
  })

  it('returns null for empty data', () => {
    const result = parseSSEEvent('result', '')
    expect(result).toBeNull()
  })
})
