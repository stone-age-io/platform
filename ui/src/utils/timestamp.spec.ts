import { describe, it, expect } from 'vitest'
import { parseTimestamp, messageTimestamp } from './timestamp'

// One instant, in every unit a device might publish it in.
const ISO = '2026-09-23T12:34:56.789Z'
const MS = Date.parse(ISO)

describe('parseTimestamp', () => {
  it.each([
    ['seconds', Math.floor(MS / 1000), Math.floor(MS / 1000) * 1000],
    ['fractional seconds', MS / 1000, MS],
    ['milliseconds', MS, MS],
    ['microseconds', MS * 1000, MS],
    ['nanoseconds', MS * 1e6, MS],
  ])('%s', (_label, input, expected) => {
    expect(parseTimestamp(input)).toBe(expected)
  })

  it('reads a numeric string as a number, not a date', () => {
    expect(parseTimestamp(String(MS))).toBe(MS)
    expect(parseTimestamp(String(Math.floor(MS / 1000)))).toBe(Math.floor(MS / 1000) * 1000)
  })

  it('reads ISO 8601', () => {
    expect(parseTimestamp(ISO)).toBe(MS)
    expect(parseTimestamp('2026-09-23T07:34:56.789-05:00')).toBe(MS)
  })

  it.each([null, undefined, '', '  ', 'soon', NaN, Infinity, 0, -5, true, {}, []])(
    'refuses %s rather than guessing',
    (input) => {
      expect(parseTimestamp(input)).toBeNull()
    },
  )
})

describe('messageTimestamp precedence', () => {
  const STORED = MS - 60_000
  const RECEIVED = MS + 60_000
  const payload = { ts: ISO, value: 1 }

  it('payload time wins when the path parses', () => {
    expect(messageTimestamp(payload, '$.ts', STORED, RECEIVED)).toBe(MS)
  })

  it('falls back to the JetStream stored time when the path is missing or bad', () => {
    expect(messageTimestamp(payload, '$.nope', STORED, RECEIVED)).toBe(STORED)
    expect(messageTimestamp({ ts: 'soon' }, '$.ts', STORED, RECEIVED)).toBe(STORED)
    expect(messageTimestamp(payload, undefined, STORED, RECEIVED)).toBe(STORED)
  })

  it('falls back to receive time for a core message', () => {
    expect(messageTimestamp(payload, undefined, undefined, RECEIVED)).toBe(RECEIVED)
    expect(messageTimestamp('plain text', '$.ts', undefined, RECEIVED)).toBe(RECEIVED)
  })
})
