import { describe, expect, it } from 'vitest'

import { fitCodePt } from './labelFit'

// The 2″ × 1″ label's text column, the tighter of the two.
const WIDTH = 26.8
const MIN = 8
const MAX = 14

/** Width the code would occupy at `pt`, using the widest face in the stack. */
const widestMm = (code: string, pt: number) => code.length * 0.602 * pt * (25.4 / 72)

describe('fitCodePt', () => {
  it('gives a short code the full maximum', () => {
    expect(fitCodePt('AHU-1', WIDTH, MIN, MAX)).toBe(MAX)
  })

  it('shrinks a longer code until it fits the column', () => {
    const code = 'S01-AHU-0003'
    const pt = fitCodePt(code, WIDTH, MIN, MAX)

    expect(pt).toBeLessThan(MAX)
    expect(pt).toBeGreaterThan(MIN)
    expect(widestMm(code, pt)).toBeLessThanOrEqual(WIDTH)
  })

  it('never goes below the minimum, even for a code that cannot fit', () => {
    // 63 characters is the schema maximum. It wraps rather than shrinking.
    expect(fitCodePt('X'.repeat(63), WIDTH, MIN, MAX)).toBe(MIN)
  })

  it('fits every length that can fit, in the widest face', () => {
    for (let n = 1; n <= 63; n++) {
      const code = 'W'.repeat(n)
      const pt = fitCodePt(code, WIDTH, MIN, MAX)
      if (pt > MIN) expect(widestMm(code, pt)).toBeLessThanOrEqual(WIDTH)
    }
  })

  it('steps in half points', () => {
    const pt = fitCodePt('S01-AHU-0003', WIDTH, MIN, MAX)
    expect(pt * 2).toBe(Math.round(pt * 2))
  })

  it('treats an empty code as fitting', () => {
    expect(fitCodePt('', WIDTH, MIN, MAX)).toBe(MAX)
  })
})
