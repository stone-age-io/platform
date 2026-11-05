import { describe, it, expect } from 'vitest'
import {
  clampToRange,
  defaultDecimals,
  formatValue,
  rangeIsValid,
  toNumber,
  valueFraction,
  zoneColor,
  zoneSpans,
} from './gaugeScale'

// Every failure here is a quiet wrong answer rather than a crash -- a fill that
// runs past the end of its track, a value that falls between two zones and
// takes the fallback colour, a boolean drawn as a full gauge. Nothing in CI
// renders a gauge, so this is the only place any of it is checked.

const R = { min: 0, max: 100 }

describe('rangeIsValid', () => {
  it('accepts a range that spans something', () => {
    expect(rangeIsValid(R)).toBe(true)
    expect(rangeIsValid({ min: -40, max: 40 })).toBe(true)
  })

  it('rejects an empty or reversed range, which would divide by <= 0', () => {
    expect(rangeIsValid({ min: 10, max: 10 })).toBe(false)
    expect(rangeIsValid({ min: 100, max: 0 })).toBe(false)
  })

  it('rejects a range that is not finite', () => {
    expect(rangeIsValid({ min: 0, max: Infinity })).toBe(false)
    expect(rangeIsValid({ min: NaN, max: 10 })).toBe(false)
  })
})

describe('clampToRange', () => {
  it('leaves a value inside the range alone', () => {
    expect(clampToRange(42, R)).toBe(42)
  })

  it('pins a value outside the range to the nearer end', () => {
    expect(clampToRange(140, R)).toBe(100)
    expect(clampToRange(-20, R)).toBe(0)
  })

  it('reads nothing as the bottom of the range', () => {
    expect(clampToRange(null, R)).toBe(0)
    expect(clampToRange(NaN, R)).toBe(0)
  })

  it('works on a range that does not start at zero', () => {
    expect(clampToRange(-50, { min: -40, max: 40 })).toBe(-40)
  })
})

describe('valueFraction', () => {
  it('maps the range onto 0..1', () => {
    expect(valueFraction(0, R)).toBe(0)
    expect(valueFraction(50, R)).toBe(0.5)
    expect(valueFraction(100, R)).toBe(1)
  })

  it('never leaves 0..1, however far outside the value is', () => {
    expect(valueFraction(500, R)).toBe(1)
    expect(valueFraction(-500, R)).toBe(0)
  })

  it('handles a range below zero', () => {
    expect(valueFraction(0, { min: -40, max: 40 })).toBe(0.5)
  })

  it('is 0 rather than Infinity for an invalid range', () => {
    // The caller shows an error instead of a bar, but this must not be NaN:
    // NaN reaches a CSS width and the fill silently disappears.
    expect(valueFraction(50, { min: 10, max: 10 })).toBe(0)
  })
})

describe('zoneSpans', () => {
  it('turns zones into fractions of the track', () => {
    expect(zoneSpans([{ min: 0, max: 60, color: 'g' }, { min: 60, max: 100, color: 'r' }], R))
      .toEqual([
        { color: 'g', start: 0, end: 0.6 },
        { color: 'r', start: 0.6, end: 1 },
      ])
  })

  it('clamps a zone that overruns the range', () => {
    // What happens when someone narrows the range and forgets the zones.
    expect(zoneSpans([{ min: -20, max: 140, color: 'g' }], R))
      .toEqual([{ color: 'g', start: 0, end: 1 }])
  })

  it('is empty for no zones or an invalid range', () => {
    expect(zoneSpans(undefined, R)).toEqual([])
    expect(zoneSpans([{ min: 0, max: 10, color: 'g' }], { min: 5, max: 5 })).toEqual([])
  })
})

describe('zoneColor', () => {
  const zones = [
    { min: 0, max: 60, color: 'green' },
    { min: 60, max: 80, color: 'amber' },
    { min: 80, max: 100, color: 'red' },
  ]

  it('picks the zone containing the value', () => {
    expect(zoneColor(30, zones, R, 'fallback')).toBe('green')
    expect(zoneColor(70, zones, R, 'fallback')).toBe('amber')
    expect(zoneColor(90, zones, R, 'fallback')).toBe('red')
  })

  it('takes the FIRST match at a shared boundary rather than guessing', () => {
    // 60 is in both the 0-60 and 60-80 bands, which is how everyone writes
    // adjacent zones. Order is the author's, so the earlier zone wins.
    expect(zoneColor(60, zones, R, 'fallback')).toBe('green')
  })

  it('leaves no gap between adjacent zones', () => {
    for (let v = 0; v <= 100; v += 1) {
      expect(zoneColor(v, zones, R, 'fallback')).not.toBe('fallback')
    }
  })

  it('falls back when no zone contains the value', () => {
    expect(zoneColor(50, [{ min: 80, max: 100, color: 'red' }], R, 'fallback')).toBe('fallback')
  })

  it('falls back with no zones at all', () => {
    expect(zoneColor(50, [], R, 'fallback')).toBe('fallback')
    expect(zoneColor(50, undefined, R, 'fallback')).toBe('fallback')
  })

  it('colours an out-of-range value by the end it is pinned to', () => {
    expect(zoneColor(500, zones, R, 'fallback')).toBe('red')
  })
})

describe('formatValue / defaultDecimals', () => {
  it('scales decimals to the magnitude when none is given', () => {
    expect(defaultDecimals(0.5)).toBe(2)
    expect(defaultDecimals(21.5)).toBe(1)
    expect(defaultDecimals(18403)).toBe(0)
    // Negatives read off the magnitude, not the sign.
    expect(defaultDecimals(-21.5)).toBe(1)
  })

  it('honours an explicit decimals, including zero', () => {
    expect(formatValue(21.55, 0)).toBe('22')
    expect(formatValue(21.55, 3)).toBe('21.550')
  })

  it('shows an em dash for nothing, rather than 0', () => {
    // A gauge reading 0 and a gauge with no data are different facts.
    expect(formatValue(null)).toBe('—')
    expect(formatValue(NaN)).toBe('—')
    expect(formatValue(0)).toBe('0.00')
  })
})

describe('toNumber', () => {
  it('takes a number or a numeric string', () => {
    expect(toNumber(42)).toBe(42)
    expect(toNumber('42.5')).toBe(42.5)
    expect(toNumber('  7 ')).toBe(7)
    expect(toNumber(-3)).toBe(-3)
  })

  it('refuses a boolean, which would draw a flag as a measurement', () => {
    expect(toNumber(true)).toBeNull()
    expect(toNumber(false)).toBeNull()
  })

  it('refuses anything that is not a number', () => {
    expect(toNumber('warm')).toBeNull()
    expect(toNumber('')).toBeNull()
    expect(toNumber('   ')).toBeNull()
    expect(toNumber(null)).toBeNull()
    expect(toNumber(undefined)).toBeNull()
    expect(toNumber({ value: 1 })).toBeNull()
    expect(toNumber([1])).toBeNull()
    expect(toNumber(Infinity)).toBeNull()
  })
})
