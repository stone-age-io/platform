import { describe, expect, it } from 'vitest'

import {
  TWIN_BUCKET,
  TWIN_BUCKET_CONFIG,
  TWIN_DESIRED_BUCKET,
  twinDrift,
  valueAtPath,
} from './twin'

// twinDrift is 20 lines and the platform's documentation spends several hundred
// words on its exact semantics: subset for objects, exact for arrays and
// scalars, and no operators, ever. None of that was pinned by anything, and the
// function was typed (any, any), so a "helpful" change to make a range work
// would have compiled, passed the build, and quietly redefined what every
// desired value on every deployment means.
describe('twinDrift', () => {
  it('reports nothing when the reported value matches', () => {
    expect(twinDrift({ mode: 'auto' }, { mode: 'auto' })).toEqual([])
    expect(twinDrift('auto', 'auto')).toEqual([])
    expect(twinDrift(20, 20)).toEqual([])
    expect(twinDrift(true, true)).toEqual([])
  })

  // A desired value is a PARTIAL assertion: only the keys it names are checked.
  // Full equality rots -- the day a device reports one new field, every
  // assertion set months ago flips to "differs".
  it('ignores reported keys the desired value does not mention', () => {
    const desired = { setpoint: 20 }
    const reported = { setpoint: 20, temp: 20.3, rssi: -61, firmware: '2.1.0' }
    expect(twinDrift(desired, reported)).toEqual([])
  })

  it('names the path of a key that differs', () => {
    expect(twinDrift({ mode: 'manual' }, { mode: 'auto' })).toEqual(['mode'])
  })

  it('names a key the device has not reported at all', () => {
    expect(twinDrift({ setpoint: 20 }, {})).toEqual(['setpoint'])
  })

  it('recurses into nested objects and reports the dotted path', () => {
    const desired = { hvac: { setpoint: 20, mode: 'auto' } }
    const reported = { hvac: { setpoint: 21, mode: 'auto' } }
    expect(twinDrift(desired, reported)).toEqual(['hvac.setpoint'])
  })

  it('reports every differing key, not just the first', () => {
    const desired = { a: 1, b: 2, c: 3 }
    const reported = { a: 9, b: 2, c: 9 }
    expect(twinDrift(desired, reported)).toEqual(['a', 'c'])
  })

  // Subset semantics are objects-only. An array is a value, so a desired array
  // must match exactly -- otherwise "the first two elements are right" would
  // silently count as agreement.
  it('compares arrays exactly, not as a subset', () => {
    expect(twinDrift({ zones: [1, 2] }, { zones: [1, 2] })).toEqual([])
    expect(twinDrift({ zones: [1, 2] }, { zones: [1, 2, 3] })).toEqual(['zones'])
    expect(twinDrift({ zones: [1, 2] }, { zones: [2, 1] })).toEqual(['zones'])
  })

  // The degenerate case: a top-level scalar has no keys to recurse into, so its
  // path is the empty string -- the whole value.
  it('uses the empty path for a top-level scalar', () => {
    expect(twinDrift('manual', 'auto')).toEqual([''])
    expect(twinDrift(20, 21)).toEqual([''])
  })

  it('does not treat a type change as agreement', () => {
    expect(twinDrift({ on: true }, { on: 'true' })).toEqual(['on'])
    expect(twinDrift({ n: 1 }, { n: '1' })).toEqual(['n'])
  })

  // The rule the documentation is most emphatic about. An operator-looking
  // object is just an object: {$gt: 30} is compared as a value, so it differs
  // from a number rather than being interpreted. If this ever starts passing,
  // someone has begun building a rules engine inside a KV browser.
  it('treats an operator-shaped desired value as a plain value, not an operator', () => {
    expect(twinDrift({ temp: { $gt: 30 } }, { temp: 35 })).toEqual(['temp'])
    expect(twinDrift({ temp: { $between: [18, 22] } }, { temp: 20 })).toEqual(['temp'])
  })

  it('handles null without throwing or matching loosely', () => {
    expect(twinDrift({ x: null }, { x: null })).toEqual([])
    expect(twinDrift({ x: null }, { x: 0 })).toEqual(['x'])
    expect(twinDrift({ x: null }, {})).toEqual(['x'])
  })
})

// The drift indicator renders the VALUES, not a word for the difference, so
// turning a path back into a before/after is part of the contract.
describe('valueAtPath', () => {
  it('returns the whole value for the empty path', () => {
    expect(valueAtPath({ a: 1 }, '')).toEqual({ a: 1 })
    expect(valueAtPath('auto', '')).toBe('auto')
  })

  it('reads a dotted path', () => {
    expect(valueAtPath({ hvac: { setpoint: 20 } }, 'hvac.setpoint')).toBe(20)
  })

  it('returns undefined rather than throwing on a missing branch', () => {
    expect(valueAtPath({}, 'hvac.setpoint')).toBeUndefined()
    expect(valueAtPath({ hvac: null }, 'hvac.setpoint')).toBeUndefined()
  })

  it('round-trips every path twinDrift reports', () => {
    const desired = { hvac: { setpoint: 20 }, mode: 'manual' }
    const reported = { hvac: { setpoint: 21 }, mode: 'auto' }
    const paths = twinDrift(desired, reported)
    expect(paths).toEqual(['hvac.setpoint', 'mode'])
    for (const p of paths) {
      expect(valueAtPath(desired, p)).not.toEqual(valueAtPath(reported, p))
    }
  })
})

// The two bucket names and their retention are duplicated in
// internal/leafsync/twin.go, and whichever side creates a bucket first defines
// it. Nothing enforces that they agree, so pin this side's values here; the Go
// side has its own test.
describe('twin bucket definitions', () => {
  it('names the two buckets by owner, not by what they describe', () => {
    expect(TWIN_BUCKET).toBe('twin')
    expect(TWIN_DESIRED_BUCKET).toBe('twin_desired')
  })

  it('keeps history, so a revision list has something to show', () => {
    expect(TWIN_BUCKET_CONFIG.history).toBe(10)
  })
})
