import { describe, it, expect } from 'vitest'
import { seriesPoints, numericPoints, buildSegments, stableColorIndex } from './chartSeries'
import type { BufferedMessage } from '@/stores/widgetData'

// A chart can be fed by one payload carrying every row, or by several subjects
// publishing the same shape. One rule has to serve both, and it is these tests
// that say so -- ChartWidget itself is never rendered in CI.

function m(timestamp: number, raw: unknown, subject = 'pumps.snapshot', value?: unknown): BufferedMessage {
  return { timestamp, raw, subject, value: value ?? raw }
}

describe('seriesPoints', () => {
  it('pulls N series out of one payload', () => {
    const buf = [
      m(1, { s1: { flow: 10 }, s2: { flow: 20 } }),
      m(2, { s1: { flow: 11 }, s2: { flow: 21 } }),
    ]
    expect(seriesPoints(buf, { label: 'S1', path: '$.s1.flow' })).toEqual([[1, 10], [2, 11]])
    expect(seriesPoints(buf, { label: 'S2', path: '$.s2.flow' })).toEqual([[1, 20], [2, 21]])
  })

  // The case the subject filter exists for: the path alone cannot tell
  // S1 from S2 when both publish {"running": ...}.
  it('separates subjects that publish an identical shape', () => {
    const buf = [
      m(1, { running: true }, 'pumps.s1.state'),
      m(2, { running: false }, 'pumps.s2.state'),
      m(3, { running: false }, 'pumps.s1.state'),
    ]
    const s1 = { label: 'S1', path: '$.running', subject: 'pumps.s1.state' }
    const s2 = { label: 'S2', path: '$.running', subject: 'pumps.s2.state' }
    expect(seriesPoints(buf, s1)).toEqual([[1, true], [3, false]])
    expect(seriesPoints(buf, s2)).toEqual([[2, false]])
  })

  it('resolves dashboard variables in the subject filter', () => {
    const buf = [m(1, { v: 1 }, 'site.S01.temp'), m(2, { v: 2 }, 'site.S02.temp')]
    const resolve = (s: string) => s.replace('{{site}}', 'S02')
    expect(seriesPoints(buf, { label: '', path: '$.v', subject: 'site.{{site}}.temp' }, resolve))
      .toEqual([[2, 2]])
  })

  it('treats a blank subject filter as every message', () => {
    const buf = [m(1, { v: 1 }, 'a'), m(2, { v: 2 }, 'b')]
    expect(seriesPoints(buf, { label: '', path: '$.v', subject: '  ' })).toHaveLength(2)
  })

  it('skips messages where the path finds nothing', () => {
    const buf = [m(1, { a: 1 }), m(2, { b: 2 }), m(3, { a: 3 })]
    expect(seriesPoints(buf, { label: '', path: '$.a' })).toEqual([[1, 1], [3, 3]])
  })

  // Timestamps come from payloads or JetStream, so arrival order is not time
  // order; a line drawn in arrival order doubles back across the axis.
  it('returns points in time order whatever order they arrived in', () => {
    const buf = [m(30, { v: 3 }), m(10, { v: 1 }), m(20, { v: 2 })]
    expect(seriesPoints(buf, { label: '', path: '$.v' }).map(p => p[0])).toEqual([10, 20, 30])
  })

  it('draws a pre-multi-series chart from the already-extracted value', () => {
    const buf = [m(1, { value: 5 }, 'x', 5), m(2, { value: 6 }, 'x', 6)]
    expect(seriesPoints(buf, null)).toEqual([[1, 5], [2, 6]])
  })
})

describe('numericPoints', () => {
  it('keeps numbers and numeric strings, maps booleans to 1/0', () => {
    expect(numericPoints([[1, 3], [2, '4.5'], [3, true], [4, false]]))
      .toEqual([[1, 3], [2, 4.5], [3, 1], [4, 0]])
  })

  // Drawing these as 0 would plot a reading the device never sent.
  it('drops what is not a number instead of plotting it as zero', () => {
    expect(numericPoints([[1, 'running'], [2, null], [3, ''], [4, { a: 1 }], [5, [1]], [6, NaN]]))
      .toEqual([])
  })
})

describe('buildSegments', () => {
  const NOW = 1000

  it('merges consecutive equal values into one segment', () => {
    const segs = buildSegments([[10, 'on'], [20, 'on'], [30, 'off'], [40, 'off'], [50, 'on']], NOW)
    expect(segs).toEqual([
      { start: 10, end: 30, value: 'on' },
      { start: 30, end: 50, value: 'off' },
      { start: 50, end: NOW, value: 'on' },
    ])
  })

  // A state that has not changed for ten minutes is the normal case; ending
  // the last segment at its own start would draw it as nothing at all.
  it('runs the last segment to now', () => {
    expect(buildSegments([[10, true]], NOW)).toEqual([{ start: 10, end: NOW, value: true }])
  })

  it('treats true and "true" as one state, as the colour rules do', () => {
    expect(buildSegments([[10, true], [20, 'true']], NOW)).toHaveLength(1)
  })

  it('does not turn a null reading into a state called "null"', () => {
    expect(buildSegments([[10, 'on'], [20, null], [30, 'on']], NOW))
      .toEqual([{ start: 10, end: NOW, value: 'on' }])
  })

  it('draws nothing for a row with no data (unknown, not guessed)', () => {
    expect(buildSegments([], NOW)).toEqual([])
  })
})

describe('stableColorIndex', () => {
  it('gives a value the same colour however the window slides', () => {
    expect(stableColorIndex('idle', 8)).toBe(stableColorIndex('idle', 8))
    for (const v of ['idle', 'running', 'stopped', 'FAULT', '', 'x'.repeat(500)]) {
      const i = stableColorIndex(v, 8)
      expect(i).toBeGreaterThanOrEqual(0)
      expect(i).toBeLessThan(8)
    }
  })

  it('spreads common state names across the palette', () => {
    const idx = new Set(['running', 'stopped', 'idle', 'fault', 'demand', 'no demand'].map(v => stableColorIndex(v, 8)))
    expect(idx.size).toBeGreaterThan(2)
  })
})
