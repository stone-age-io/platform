// The largest point size at which a label's code fits its text column on one
// line, clamped to the stock's range. Below `minPt` the code wraps instead.
//
// Arithmetic, not a DOM measurement: the code is monospace and ASCII (by the
// schema's pattern), so width is length times a constant.

/** 1pt = 1/72 in. */
const MM_PER_PT = 25.4 / 72

/**
 * Glyph advance as a fraction of font size. Tailwind's `font-mono` faces span
 * 0.55 (Consolas) to 0.602 (Menlo); rounded up so an error shrinks rather than
 * wraps.
 */
const MONO_ADVANCE = 0.62

export function fitCodePt(code: string, widthMm: number, minPt: number, maxPt: number): number {
  if (!code.length) return maxPt
  const pt = widthMm / (code.length * MONO_ADVANCE * MM_PER_PT)
  // Half-point steps: finer than any print engine renders differently.
  return Math.min(maxPt, Math.max(minPt, Math.floor(pt * 2) / 2))
}
