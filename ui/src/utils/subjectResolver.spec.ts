import { describe, it, expect } from 'vitest'
import {
  DEFAULT_PREFIX,
  VAR_LOCATION,
  VAR_THING,
  VAR_THING_TYPE_CODE,
  join,
  resolveThing,
  resolveRolePattern,
  resolveOperationSubjects,
  hasUnresolved,
} from './subjectResolver'

describe('DEFAULT_PREFIX', () => {
  // This constant is a published contract: ADR 0003 and docs/thing-types.md in
  // platform-docs state it, and it once shipped disagreeing with the docs
  // (location-first) for as long as it existed. Nothing type-checks a docs page,
  // so assert the literal.
  it('is the documented family-first, location-free default', () => {
    expect(DEFAULT_PREFIX).toBe('{thing_type_code}.{thing}')
  })

  // Things move, and a subject resolved from the current location would move
  // with them. The default must not reintroduce it.
  it('does not contain the location', () => {
    expect(DEFAULT_PREFIX).not.toContain(VAR_LOCATION)
  })

  // The real invariant, and the reason the order is not cosmetic. A template
  // leading with a variable that resolves to "*" produces a wildcard-leading
  // subject. JetStream refuses two streams whose subject filters overlap, and a
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
    expect(join('{thing_type_code}.{thing}', 'motion')).toBe('{thing_type_code}.{thing}.motion')
    expect(join('camera.{location}.{thing}', 'cmd.ptz')).toBe('camera.{location}.{thing}.cmd.ptz')
  })
})

describe('resolveThing', () => {
  it('substitutes every supplied variable', () => {
    expect(resolveThing(DEFAULT_PREFIX, { location: 'KC-DC1', thing: 'CA-9KD-4PX', thingTypeCode: 'camera' }))
      .toBe('camera.CA-9KD-4PX')
    // A prefix that opts into the location still gets it.
    expect(resolveThing('freezer.{location}.{thing}', { location: 'KC-DC1', thing: 'FZ-1' }))
      .toBe('freezer.KC-DC1.FZ-1')
  })

  // Callers detect incomplete input by looking for a leftover token, so an
  // unset field must NOT silently collapse to an empty string — that would
  // produce `camera..CAM-042`, a subject with an empty token that reads as
  // valid and matches nothing.
  it('leaves unset fields as literal template tokens', () => {
    const out = resolveThing(DEFAULT_PREFIX, { thingTypeCode: 'camera' })
    expect(out).toBe(`camera.${VAR_THING}`)
    expect(out).toContain(VAR_THING)
    expect(resolveThing('freezer.{location}.{thing}', { thing: 'FZ-1' })).toContain(VAR_LOCATION)
  })
})

describe('resolveRolePattern', () => {
  it('always wildcards the thing, and the location only when absent', () => {
    expect(resolveRolePattern(DEFAULT_PREFIX, { thingTypeCode: 'camera', location: 'KC-DC1' }))
      .toBe('camera.*')
    expect(resolveRolePattern('freezer.{location}.{thing}', { location: 'KC-DC1' }))
      .toBe('freezer.KC-DC1.*')
    expect(resolveRolePattern('freezer.{location}.{thing}'))
      .toBe('freezer.*.*')
  })
})

describe('resolveOperationSubjects', () => {
  const ops = [
    { id: '1', name: 'subscribe_grant', capability: 'subscribe', subject_suffix: 'cmd.grant' },
    { id: '2', name: 'publish_state', capability: 'publish', subject_suffix: 'evt.state' },
    { id: '3', name: 'reply_diag', capability: 'reply', subject_suffix: 'diag' },
    { id: '4', name: 'publish_alarm', capability: 'publish', subject_suffix: 'evt.alarm' },
  ]

  // The Thing page and the type form both render this list; the full subject is
  // the type's prefix (or the default) joined to the operation's suffix.
  it('joins each suffix to the prefix and resolves it for the thing', () => {
    const out = resolveOperationSubjects('acc.{location}.door.{thing}', ops, { location: 'KC', thing: 'D-1' })
    expect(out.find(o => o.id === '4')?.subject).toBe('acc.KC.door.D-1.evt.alarm')
    expect(resolveOperationSubjects('', [ops[1]], { thingTypeCode: 'door', thing: 'D-1' })[0].subject)
      .toBe('door.D-1.evt.state')
  })

  // What the device sends first, then what it listens for; by name within one.
  it('orders by capability, then name', () => {
    expect(resolveOperationSubjects('', ops, {}).map(o => o.name))
      .toEqual(['publish_alarm', 'publish_state', 'subscribe_grant', 'reply_diag'])
  })

  it('does not reorder the array it was given', () => {
    const copy = [...ops]
    resolveOperationSubjects('', ops, {})
    expect(ops).toEqual(copy)
  })
})

describe('hasUnresolved', () => {
  it('spots a variable the context could not fill', () => {
    expect(hasUnresolved('acc.{location}.door.D-1')).toBe(true)
    expect(hasUnresolved('acc.KC.door.D-1')).toBe(false)
  })

  // Braces that are not one of the four variables are just characters.
  it('ignores braces that are not a variable', () => {
    expect(hasUnresolved('weird.{other}.x')).toBe(false)
  })
})
