import { describe, it, expect } from 'vitest'
import { targetDimensions, MAX_DIMENSION } from './imageResize'

/**
 * Only the pure half. The canvas half of imageResize cannot be tested in this
 * suite at all -- vitest.config.ts runs node, and jsdom does not implement
 * canvas.toBlob -- which is precisely why every decision lives here instead of
 * inside the drawing code.
 */
describe('targetDimensions', () => {
  it('leaves an image that is already small enough alone', () => {
    // null, not the original size: the caller uses it to skip re-encoding
    // entirely, and a small PNG through a canvas comes out a larger JPEG.
    expect(targetDimensions(800, 600)).toBeNull()
    expect(targetDimensions(MAX_DIMENSION, MAX_DIMENSION)).toBeNull()
  })

  it('scales the long edge down to the cap', () => {
    expect(targetDimensions(3200, 2400)).toEqual({ width: 1600, height: 1200 })
    expect(targetDimensions(2400, 3200)).toEqual({ width: 1200, height: 1600 })
  })

  it('preserves aspect ratio within a pixel', () => {
    // 4032x3024 is an iPhone photo, the case this exists for.
    const t = targetDimensions(4032, 3024)!
    expect(Math.max(t.width, t.height)).toBe(1600)
    expect(Math.abs(t.width / t.height - 4032 / 3024)).toBeLessThan(0.01)
  })

  it('never rounds an extreme ratio down to zero', () => {
    // A panorama: 20000x50 scales the short edge to 4px, and Math.round of
    // anything under 0.5 would be 0 -- a zero-sized canvas throws.
    const t = targetDimensions(20000, 50)!
    expect(t.width).toBe(1600)
    expect(t.height).toBeGreaterThanOrEqual(1)
  })

  it('treats a degenerate size as nothing to do', () => {
    expect(targetDimensions(0, 0)).toBeNull()
    expect(targetDimensions(-10, 100)).toBeNull()
  })

  it('honours a caller-supplied cap', () => {
    expect(targetDimensions(1000, 500, 400)).toEqual({ width: 400, height: 200 })
  })
})
