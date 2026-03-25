export interface ParsedSSEEvent {
  type: string
  data: Record<string, unknown>
}

export function parseSSEEvent(type: string, rawData: string): ParsedSSEEvent | null {
  if (!rawData) return null

  try {
    const data = JSON.parse(rawData)
    return { type, data }
  } catch {
    return null
  }
}
