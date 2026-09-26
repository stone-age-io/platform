// The largest point size at which a label's code fits its text column on one
// line, clamped to the stock's range.
//
// A fixed size has to be chosen for the longest code a tenant might use, which
// leaves every ordinary code printing at a fraction of the size the sticker
// could carry. The code is the fallback for the label that will not scan, read
// aloud from across a plant room, so it gets whatever width is going.
//
// This is arithmetic rather than a DOM measurement because the code is set in a
// monospace face: every glyph has the same advance, so width is length times a
// constant. Codes are ASCII by the schema's own pattern, so length is glyphs.
//
// Below `minPt` the code wraps, at a hyphen where it has one, rather than
// shrinking into something nobody can read.

/** 1pt = 1/72 in. */
const MM_PER_PT = 25.4 / 72

/**
 * Advance width of one glyph as a fraction of the font size. The faces in
 * Tailwind's `font-mono` stack sit between 0.55 (Consolas) and 0.602 (Menlo,
 * DejaVu Sans Mono), bold included. Rounding UP means an estimate that is off
 * errs toward a code slightly smaller than it could be, never one that wraps.
 */
const MONO_ADVANCE = 0.62

export function fitCodePt(code: string, widthMm: number, minPt: number, maxPt: number): number {
  if (!code.length) return maxPt
  const pt = widthMm / (code.length * MONO_ADVANCE * MM_PER_PT)
  // Half-point steps: finer than any print engine renders differently.
  return Math.min(maxPt, Math.max(minPt, Math.floor(pt * 2) / 2))
}
