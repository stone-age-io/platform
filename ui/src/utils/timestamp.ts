// ui/src/utils/timestamp.ts
import { extractJsonPath } from './jsonPath'

// Epoch magnitudes for "now" (2026): 1.8e9 s, 1.8e12 ms, 1.8e15 µs, 1.8e18 ns.
// Each boundary sits two or more orders of magnitude from both neighbours, so
// a real timestamp in any unit lands in exactly one band for centuries either
// side of today.
const MS_FLOOR = 1e11
const US_FLOOR = 1e14
const NS_FLOOR = 1e17

/**
 * Read a timestamp out of a payload value, in whatever unit a device chose.
 * Numbers are classified by magnitude (s / ms / µs / ns); numeric strings are
 * numbers; other strings go through Date.parse (ISO 8601). Returns epoch ms, or
 * null when the value is not a usable timestamp -- never a guess.
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
 * The timestamp a buffered message carries, in order of precedence:
 *   1. the payload's own time, when the widget names a `timestampPath` and it
 *      parses -- the only right answer for edge data that arrives late;
 *   2. the time JetStream STORED the message -- so a replay spreads across the
 *      window instead of collapsing onto the moment it was delivered;
 *   3. the time the browser received it, which is all a core message has.
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
