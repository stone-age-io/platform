import { describe, it, expect } from 'vitest'
import {
  DEFAULT_PREFIX,
  VAR_LOCATION,
  VAR_THING,
  VAR_THING_TYPE_CODE,
  join,
  resolveThing,
  resolveRolePattern,
} from './subjectResolver'

describe('DEFAULT_PREFIX', () => {
  // This constant is a published contract: docs/thing-types.md and
  // docs/connectivity.md in platform-docs both state it, and it shipped
  // disagreeing with them (location-first) for as long as it existed. Nothing
  // type-checks a docs page, so assert the literal.
  it('is the documented family-first default', () => {
    expect(DEFAULT_PREFIX).toBe('{thing_type_code}.{location}.{thing}')
  })

  // The real invariant, and the reason the order is not cosmetic.
  // resolveRolePattern substitutes {location} -> "*" when no location is given,
  // so a template leading with {location} produces a wildcard-leading subject.
  // JetStream refuses two streams whose subject filters overlap, and a
  // wildcard-leading filter intersects every literal-rooted one — so a single
  // such role pattern makes per-thing-type streams impossible.
  it('resolves to a role pattern whose first token is a literal', () => {
    const firstToken = resolveRolePattern(DEFAULT_PREFIX).split('.')[0]
    expect(firstToken).not.toBe('*')
    expect(firstToken).not.toBe('>')
    expect(firstToken).toBe(VAR_THING_TYPE_CODE)
  })
})

describe('join', () => {
  it('falls back to the default prefix when none is set', () => {
    expect(join('', 'evt.motion')).toBe(`${DEFAULT_PREFIX}.evt.motion`)
    expect(join(null, 'evt.motion')).toBe(`${DEFAULT_PREFIX}.evt.motion`)
    expect(join(undefined, undefined)).toBe(DEFAULT_PREFIX)
  })

  it('appends the operation suffix to a bespoke prefix', () => {
    expect(join('camera.{location}.{thing}', 'cmd.ptz')).toBe('camera.{location}.{thing}.cmd.ptz')
  })
})

describe('resolveThing', () => {
  it('substitutes every supplied variable', () => {
    expect(resolveThing(DEFAULT_PREFIX, { location: 'KC-DC1', thing: 'CAM-042', thingTypeCode: 'camera' }))
      .toBe('camera.KC-DC1.CAM-042')
  })

  // Callers detect incomplete input by looking for a leftover token, so an
  // unset field must NOT silently collapse to an empty string — that would
  // produce `camera..CAM-042`, a subject with an empty token that reads as
  // valid and matches nothing.
  it('leaves unset fields as literal template tokens', () => {
    const out = resolveThing(DEFAULT_PREFIX, { thingTypeCode: 'camera' })
    expect(out).toBe(`camera.${VAR_LOCATION}.${VAR_THING}`)
    expect(out).toContain(VAR_LOCATION)
  })
})

describe('resolveRolePattern', () => {
  it('always wildcards the thing, and the location only when absent', () => {
    expect(resolveRolePattern(DEFAULT_PREFIX, { thingTypeCode: 'camera', location: 'KC-DC1' }))
      .toBe('camera.KC-DC1.*')
    expect(resolveRolePattern(DEFAULT_PREFIX, { thingTypeCode: 'camera' }))
      .toBe('camera.*.*')
  })
})
