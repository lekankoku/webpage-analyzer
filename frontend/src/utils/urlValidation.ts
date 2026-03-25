export function isValidUrl(input: string): boolean {
  const trimmed = input.trim()
  if (!trimmed) return false

  try {
    const url = new URL(trimmed)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return false
    if (!url.hostname) return false
    // Reject URLs like "https:///path" where authority is empty
    if (/^https?:\/\/\//.test(trimmed)) return false
    return true
  } catch {
    return false
  }
}
