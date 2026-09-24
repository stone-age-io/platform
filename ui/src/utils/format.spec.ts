import { describe, it, expect } from 'vitest'
import { formatColumnValue } from './format'

// Timestamp columns in the KV table, stream table and dynamic-marker panel.
// Their parser used to disagree with the one messages are stamped with (see
// utils/timestamp): a Go UnixNano value threw inside the table's computed and
// blanked the whole table, and anything unparseable read as "just now".

const MS = Date.parse('2026-09-23T12:00:00Z')

describe('formatColumnValue timestamps', () => {
  it.each([
    ['nanoseconds (Go UnixNano)', MS * 1e6],
    ['microseconds', MS * 1e3],
    ['milliseconds', MS],
    ['seconds', MS / 1000],
    ['seconds as a string', String(MS / 1000)],
    ['ISO 8601', '2026-09-23T12:00:00Z'],
    ['a Date (the KV __timestamp__ meta-path)', new Date(MS)],
  ])('reads %s as the same instant', (_label, value) => {
    expect(formatColumnValue(value, 'datetime', 'yyyy-MM-dd')).toBe('2026-09-23')
  })

  it.each([0, '', 'soon', {}, new Date('nope')])('shows "-" for %s rather than "just now"', (value) => {
    expect(formatColumnValue(value, 'relative-time')).toBe('-')
    expect(formatColumnValue(value, 'datetime')).toBe('-')
  })

  it('does not throw on a nanosecond epoch', () => {
    expect(() => formatColumnValue(MS * 1e6, 'relative-time')).not.toThrow()
  })
})
