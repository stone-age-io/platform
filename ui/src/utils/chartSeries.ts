// ui/src/utils/chartSeries.ts
//
// Everything a chart decides about its data, kept out of ChartWidget.vue so it
// can be tested: nothing renders a component in CI, and `vue-tsc && vite build`
// stays green however wrong these are.
import type { BufferedMessage } from '@/stores/widgetData'
import type { ChartSeries } from '@/types/dashboard'
import { extractJsonPath } from './jsonPath'

export type Point = [timestamp: number, value: unknown]

/**
 * The points one series draws, oldest first.
 *
 * With no `series` (a chart saved before multi-series), the caller passes
 * `null` and gets the one series it always had: `m.value`, which the widget's
 * own jsonPath already extracted at ingest.
 *
 * Sorted because a timestamp can come from the payload or from JetStream, so
 * the buffer's arrival order is not time order -- and a line drawn in arrival
 * order zig-zags back across the axis.
 */
export function seriesPoints(
  buffer: readonly BufferedMessage[],
  series: ChartSeries | null,
  resolveSubject: (s: string) => string = (s) => s,
): Point[] {
  const points: Point[] = []

  if (!series) {
    for (const m of buffer) points.push([m.timestamp, m.value])
  } else {
    const subject = series.subject?.trim() ? resolveSubject(series.subject.trim()) : ''
    for (const m of buffer) {
      if (subject && m.subject !== subject) continue
      const v = extractJsonPath(m.raw, series.path)
      if (v === undefined) continue
      points.push([m.timestamp, v])
    }
  }

  return points.sort((a, b) => a[0] - b[0])
}

/**
 * Line and bar charts plot numbers. A value that is not one (a string, null,
 * an object the path landed on) is dropped rather than drawn as zero, which
 * would be a reading the device never sent.
 */
export function numericPoints(points: readonly Point[]): [number, number][] {
  const out: [number, number][] = []
  for (const [ts, v] of points) {
    if (v === null || v === '' || typeof v === 'object') continue
    const n = typeof v === 'boolean' ? (v ? 1 : 0) : Number(v)
    if (Number.isFinite(n)) out.push([ts, n])
  }
  return out
}

export interface Segment {
  start: number
  end: number
  value: unknown
}

/**
 * A state timeline row: consecutive equal values merged into one segment.
 * Each segment ends where the next value begins; the LAST one runs to `now`,
 * because a state that has not changed in ten minutes is the normal case, not
 * missing data.
 *
 * Nothing is drawn before the first point. A row with no data in the window
 * is blank -- unknown -- and deliberately not filled in with a guess: filling
 * it is the source's job (republish state periodically, or have a rule-router
 * rule publish a snapshot).
 *
 * Equality is on String(value), so `true` and "true" are one state -- the same
 * way the threshold rules colouring them compare.
 */
export function buildSegments(points: readonly Point[], now: number): Segment[] {
  const segments: Segment[] = []
  for (const [ts, value] of points) {
    // null is "no reading", not a state named "null"; it neither starts nor
    // ends a segment.
    if (value === null) continue
    const last = segments[segments.length - 1]
    if (last && String(last.value) === String(value)) continue
    if (last) last.end = ts
    segments.push({ start: ts, end: ts, value })
  }
  const last = segments[segments.length - 1]
  if (last) last.end = Math.max(last.end, now)
  return segments
}

/**
 * A palette index for a value no rule matched. Hashed from the value rather
 * than assigned in order of appearance, so "idle" keeps its colour as the
 * window slides and the first state it saw drops out of view.
 */
export function stableColorIndex(value: string, n: number): number {
  let h = 0
  for (let i = 0; i < value.length; i++) h = (h * 31 + value.charCodeAt(i)) | 0
  return Math.abs(h) % n
}
