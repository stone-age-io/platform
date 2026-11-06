// Everything a chart decides about its data, kept out of ChartWidget.vue so it
// can be tested (nothing renders a component in CI).
import type { BufferedMessage } from '@/stores/widgetData'
import type { ChartSeries } from '@/types/dashboard'
import { extractJsonPath } from './jsonPath'

export type Point = [timestamp: number, value: unknown]

/**
 * The points one series draws, sorted by time (arrival order is not time
 * order when timestamps come from the payload or JetStream).
 *
 * `null` series (a chart saved before multi-series) draws `m.value`, which the
 * widget's own jsonPath already extracted at ingest.
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
 * Booleans become 1/0. Anything else non-numeric is dropped, not drawn as zero
 * (a reading the device never sent).
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
 * A state timeline row: consecutive equal values merged into one segment. The
 * last segment runs to `now` (an unchanged state is not missing data).
 *
 * Nothing is drawn before the first point, deliberately: a row with no data is
 * unknown, and filling it is the source's job (republish state periodically).
 *
 * Equality is on String(value), so `true` and "true" are one state, matching
 * the threshold colour rules.
 */
export function buildSegments(points: readonly Point[], now: number): Segment[] {
  const segments: Segment[] = []
  for (const [ts, value] of points) {
    // null is "no reading": it neither starts nor ends a segment.
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
 * A palette index for a value no rule matched. Hashed rather than assigned in
 * order of appearance, so a value keeps its colour as the window slides.
 */
export function stableColorIndex(value: string, n: number): number {
  let h = 0
  for (let i = 0; i < value.length; i++) h = (h * 31 + value.charCodeAt(i)) | 0
  return Math.abs(h) % n
}
