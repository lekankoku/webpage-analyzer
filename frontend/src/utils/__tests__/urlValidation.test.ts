import { describe, it, expect } from 'vitest'
import { isValidUrl } from '../urlValidation'

describe('isValidUrl', () => {
  it('accepts a valid http URL', () => {
    expect(isValidUrl('http://example.com')).toBe(true)
  })

  it('accepts a valid https URL', () => {
    expect(isValidUrl('https://example.com')).toBe(true)
  })

  it('accepts a URL with path and query', () => {
    expect(isValidUrl('https://example.com/path?q=1')).toBe(true)
  })

  it('rejects a URL without scheme', () => {
    expect(isValidUrl('example.com')).toBe(false)
  })

  it('rejects an ftp URL', () => {
    expect(isValidUrl('ftp://example.com')).toBe(false)
  })

  it('rejects an empty string', () => {
    expect(isValidUrl('')).toBe(false)
  })

  it('rejects whitespace-only input', () => {
    expect(isValidUrl('   ')).toBe(false)
  })

  it('rejects a URL with scheme but no host', () => {
    expect(isValidUrl('http://')).toBe(false)
  })

  it('rejects a URL with empty host after scheme', () => {
    expect(isValidUrl('https:///path')).toBe(false)
  })

  it('accepts a URL with port', () => {
    expect(isValidUrl('http://localhost:3000')).toBe(true)
  })

  it('accepts a URL with subdomain', () => {
    expect(isValidUrl('https://www.example.co.uk')).toBe(true)
  })
})
