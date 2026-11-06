// The one parser for human-readable durations ("30s", "5m", "1h30m", "7d").

export const NANOS_PER_MS = 1_000_000
export const NANOS_PER_SEC = 1_000_000_000
export const NANOS_PER_MIN = 60 * NANOS_PER_SEC
export const NANOS_PER_HOUR = 60 * NANOS_PER_MIN
export const NANOS_PER_DAY = 24 * NANOS_PER_HOUR

/**
 * Parse a duration to nanoseconds. A plain number is read as nanoseconds
 * (stream config convention).
 */
export function parseHumanDuration(str: string): number {
  if (!str || str === '0') return 0
  const trimmed = str.trim()

  if (/^\d+$/.test(trimmed)) return parseInt(trimmed, 10)

  let total = 0
  const regex = /(\d+(?:\.\d+)?)\s*(d|h|m|s|ms|us|ns)/gi
  let match
  while ((match = regex.exec(trimmed)) !== null) {
    const val = parseFloat(match[1])
    switch (match[2].toLowerCase()) {
      case 'd': total += val * NANOS_PER_DAY; break
      case 'h': total += val * NANOS_PER_HOUR; break
      case 'm': total += val * NANOS_PER_MIN; break
      case 's': total += val * NANOS_PER_SEC; break
      case 'ms': total += val * NANOS_PER_MS; break
      case 'us': total += val * 1_000; break
      case 'ns': total += val; break
    }
  }
  return Math.round(total)
}

/**
 * Parse a dashboard duration ("10m", "1h30m") to milliseconds, or null for
 * empty or unparseable input so each caller picks its own fallback. A plain
 * number is rejected: parseHumanDuration would read it as nanoseconds.
 */
export function parseDurationMs(str: string | undefined | null): number | null {
  if (!str) return null
  const trimmed = str.trim()
  if (!trimmed || /^\d+$/.test(trimmed)) return null
  const ms = parseHumanDuration(trimmed) / NANOS_PER_MS
  return ms > 0 ? ms : null
}
