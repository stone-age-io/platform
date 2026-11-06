import { extractJsonPath } from './jsonPath'

// Epoch magnitudes for "now" (2026): 1.8e9 s, 1.8e12 ms, 1.8e15 µs, 1.8e18 ns.
// Each boundary is orders of magnitude from both neighbours, so a real
// timestamp lands in exactly one band for centuries.
const MS_FLOOR = 1e11
const US_FLOOR = 1e14
const NS_FLOOR = 1e17

/**
 * A payload value as epoch ms, or null. Numbers (and numeric strings) are
 * classified by magnitude (s / ms / µs / ns); other strings go through
 * Date.parse.
 */
export function parseTimestamp(v: unknown): number | null {
  let n: number
  if (typeof v === 'number') {
    n = v
  } else if (typeof v === 'string' && v.trim() !== '') {
    const s = v.trim()
    if (/^-?\d+(\.\d+)?$/.test(s)) {
      n = Number(s)
    } else {
      const parsed = Date.parse(s)
      return Number.isNaN(parsed) ? null : parsed
    }
  } else {
    return null
  }

  if (!Number.isFinite(n) || n <= 0) return null
  if (n < MS_FLOOR) return Math.round(n * 1000)
  if (n < US_FLOOR) return Math.round(n)
  if (n < NS_FLOOR) return Math.round(n / 1e3)
  return Math.round(n / 1e6)
}

/**
 * A buffered message's timestamp, in order of precedence:
 *   1. the payload's own time via `timestampPath` (edge data can arrive late);
 *   2. when JetStream stored it, so a replay spreads across the window;
 *   3. when the browser received it.
 */
export function messageTimestamp(
  data: unknown,
  timestampPath: string | undefined,
  storedAt: number | undefined,
  receivedAt: number,
): number {
  if (timestampPath) {
    const fromPayload = parseTimestamp(extractJsonPath(data, timestampPath))
    if (fromPayload !== null) return fromPayload
  }
  return storedAt ?? receivedAt
}
