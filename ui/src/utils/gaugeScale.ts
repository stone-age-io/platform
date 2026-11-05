// ui/src/utils/gaugeScale.ts
//
// The arithmetic behind a gauge: where a value sits in its range, which zone it
// lands in, and how to write it down.
//
// Extracted from GaugeWidget when the marker panel gained a gauge row. It is
// pure, so unlike the widget it can actually be tested here -- and every one of
// these is a quiet wrong-answer rather than a crash: a zone list whose bands do
// not reach the value, a reversed range that divides by a negative, a value
// outside its range running the fill past the end of the track. None of that
// throws, and nothing in CI renders a gauge.
//
// The widget and the marker row both draw from this, so the bar in a 280px
// panel and the arc on a dashboard cannot disagree about what amber means.

export interface GaugeZone {
  min: number
  max: number
  color: string
}

export interface GaugeRange {
  min: number
  max: number
}

/**
 * A range has to span something. `max <= min` is not a style choice to be
 * rendered oddly -- every fraction below would be negative or infinite -- so
 * callers check this and say so rather than drawing a lie.
 */
export function rangeIsValid(range: GaugeRange): boolean {
  const span = range.max - range.min
  return Number.isFinite(span) && span > 0
}

/** The value as drawn: inside the range, whatever arrived. */
export function clampToRange(value: number | null, range: GaugeRange): number {
  if (value === null || !Number.isFinite(value)) return range.min
  return Math.max(range.min, Math.min(range.max, value))
}

/** Position in the range, 0..1. Safe to multiply by a width. */
export function valueFraction(value: number | null, range: GaugeRange): number {
  if (!rangeIsValid(range)) return 0
  return (clampToRange(value, range) - range.min) / (range.max - range.min)
}

/**
 * Each zone as a 0..1 span of the track.
 *
 * Clamped at both ends so a zone list that overruns the range -- 0-120 on a
 * 0-100 gauge, which is what happens when somebody widens the range and
 * forgets the zones -- paints to the end of the track instead of past it.
 */
export function zoneSpans(
  zones: GaugeZone[] | undefined,
  range: GaugeRange,
): Array<{ color: string; start: number; end: number }> {
  if (!zones || !rangeIsValid(range)) return []
  const frac = (v: number) => Math.max(0, Math.min(1, (v - range.min) / (range.max - range.min)))
  return zones.map(z => ({ color: z.color, start: frac(z.min), end: frac(z.max) }))
}

/**
 * The colour for a value: the first zone that contains it, else the fallback.
 *
 * FIRST match, not best or narrowest. Overlapping zones are a configuration
 * mistake and picking between them is a judgement this cannot make; taking the
 * first makes the list an order the author controls, the same rule the status
 * widget's mappings follow.
 *
 * Bounds are inclusive at both ends, so adjacent zones written the obvious way
 * (0-60, 60-80) leave no value uncoloured. The overlap at the boundary resolves
 * by the first-match rule above.
 */
export function zoneColor(
  value: number | null,
  zones: GaugeZone[] | undefined,
  range: GaugeRange,
  fallback: string,
): string {
  if (!zones || zones.length === 0) return fallback
  const v = clampToRange(value, range)
  for (const zone of zones) {
    if (v >= zone.min && v <= zone.max) return zone.color
  }
  return fallback
}

/**
 * Decimals when the author has not said.
 *
 * Scaled to the magnitude: a temperature reads better as 21.53 than 22, and a
 * packet count worse as 18403.00 than 18403. The widget's own behaviour, kept
 * because changing it would silently reformat every gauge already on a
 * dashboard.
 */
export function defaultDecimals(value: number): number {
  const abs = Math.abs(value)
  if (abs >= 100) return 0
  if (abs >= 10) return 1
  return 2
}

/** The value as text, or an em dash when there is nothing to show. */
export function formatValue(value: number | null, decimals?: number): string {
  if (value === null || !Number.isFinite(value)) return '—'
  return value.toFixed(decimals ?? defaultDecimals(value))
}

/**
 * Pull a number out of whatever arrived.
 *
 * A payload may be a bare number, a numeric string, or an object a JSON path
 * has already been applied to. A boolean is deliberately NOT converted: `true`
 * becoming 1 would draw a full gauge for a device reporting a flag, which reads
 * as a measurement rather than as the misconfiguration it is.
 */
export function toNumber(value: unknown): number | null {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return null
    const n = Number(trimmed)
    return Number.isFinite(n) ? n : null
  }
  return null
}
