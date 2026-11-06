// The arithmetic behind a gauge: where a value sits in its range, which zone it
// lands in, and how to write it down. Shared by GaugeWidget and the marker
// panel's gauge row so they agree, and pure so it can be tested.

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
 * `max <= min` makes every fraction below negative or infinite, so callers
 * check this and show an error instead of drawing.
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
 * Each zone as a 0..1 span of the track, clamped so zones that overrun the
 * range stop at the end of the track.
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
 * The colour for a value: the FIRST zone that contains it (author's order, as
 * in the status widget's mappings), else the fallback. Bounds are inclusive, so
 * adjacent zones (0-60, 60-80) leave no gap.
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
 * Decimals when the author has not set them, scaled to the magnitude. Changing
 * this reformats every existing gauge.
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
 * A number or numeric string, else null. A boolean is deliberately NOT
 * converted: `true` as 1 would draw a misconfiguration as a measurement.
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
