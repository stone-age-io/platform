import { describe, it, expect } from 'vitest'
import { parseDurationMs, parseHumanDuration, NANOS_PER_MS } from './duration'

// parseDurationMs replaced two single-unit regexes (`^(\d+)([smhd])$`) that
// silently rejected "1h30m" and fell back to five minutes. The fallback now
// belongs to each caller, so null has to mean exactly "no usable duration".

describe('parseDurationMs', () => {
  it.each([
    ['30s', 30_000],
    ['5m', 300_000],
    ['2h', 7_200_000],
    ['7d', 604_800_000],
    ['1h30m', 5_400_000],
    [' 10m ', 600_000],
  ])('%s → %d ms', (input, ms) => {
    expect(parseDurationMs(input)).toBe(ms)
  })

  it.each([undefined, null, '', '   ', 'abc', '0', '0m'])('%s → null', (input) => {
    expect(parseDurationMs(input as any)).toBeNull()
  })

  // parseHumanDuration reads a bare number as NANOSECONDS (stream config
  // convention). Nobody typing "30" into a chart window means 30ns.
  it('rejects a bare number rather than reading it as nanoseconds', () => {
    expect(parseDurationMs('30')).toBeNull()
  })

  it('agrees with parseHumanDuration, which stream config still uses', () => {
    for (const s of ['45s', '15m', '3h', '1d', '1h30m']) {
      expect(parseDurationMs(s)).toBe(parseHumanDuration(s) / NANOS_PER_MS)
    }
  })
})
