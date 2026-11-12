import { describe, it, expect } from 'vitest'
import { fillTemplate, hasToken } from './templates'
import { resolveTemplate } from './variables'
import { resolveThing } from './subjectResolver'

describe('fillTemplate', () => {
  // Thing Type prefixes are stored with single braces and dashboards with
  // double, so one function has to read both.
  it('fills both spellings of a token', () => {
    expect(fillTemplate('a.{x}.b', { x: '1' })).toBe('a.1.b')
    expect(fillTemplate('a.{{x}}.b', { x: '1' })).toBe('a.1.b')
    expect(fillTemplate('a.{{ x }}.b', { x: '1' })).toBe('a.1.b')
  })

  it('keeps a token it has no value for', () => {
    expect(fillTemplate('a.{x}.{{y}}', {})).toBe('a.{x}.{{y}}')
    expect(fillTemplate('a.{x}', { x: undefined })).toBe('a.{x}')
  })

  it('writes `missing` for an unknown token when asked', () => {
    expect(fillTemplate('scans.{purpose}.{nope}', { purpose: 'door' }, '')).toBe('scans.door.')
  })

  // An empty value is still a value: a dashboard variable set to '' fills in.
  it('fills an empty string', () => {
    expect(fillTemplate('a.{{x}}.b', { x: '' })).toBe('a..b')
  })

  it('leaves JSON alone', () => {
    const json = '{"state":"on","n":{"a":1},"e":{}}'
    expect(fillTemplate(json, { state: 'x', a: 'y' })).toBe(json)
    expect(fillTemplate('{"level": {{value}}}', { value: '7' })).toBe('{"level": 7}')
  })

  it('accepts a modifier in the name', () => {
    expect(fillTemplate('code:lower = "{value:lower}"', { 'value:lower': 'ab' }))
      .toBe('code:lower = "ab"')
  })

  it('does not read inherited object keys', () => {
    expect(fillTemplate('{constructor}.{toString}', {})).toBe('{constructor}.{toString}')
  })
})

describe('hasToken', () => {
  it('finds a token in either spelling', () => {
    expect(hasToken('thing.{code}.temp')).toBe(true)
    expect(hasToken('thing.{{code}}.temp')).toBe(true)
    // Called twice on purpose: a global regex remembers where it stopped.
    expect(hasToken('thing.{code}.temp')).toBe(true)
  })

  // Forms skip their checks when this is true, so JSON must not count.
  it('does not mistake JSON for a token', () => {
    expect(hasToken('{"state":"on","n":{},"l":[1]}')).toBe(false)
    expect(hasToken('plain.subject.>')).toBe(false)
    expect(hasToken('{"level": {value}}')).toBe(true)
  })
})

// A floor-plan item is filled twice: per Thing first, then with the dashboard's
// variables. The first pass used to turn `{{region}}` into `{<value>}` whenever
// the names collided, and must leave a variable it does not own intact.
describe('two passes over one template', () => {
  it('keeps a dashboard variable through the Thing pass', () => {
    const afterThing = resolveThing('{{region}}.{thing}.state', { thing: 'D-1' })
    expect(afterThing).toBe('{{region}}.D-1.state')
    expect(resolveTemplate(afterThing, { region: 'east' })).toBe('east.D-1.state')
  })
})

describe('resolveTemplate', () => {
  it('returns an empty string for no template', () => {
    expect(resolveTemplate(undefined, { a: '1' })).toBe('')
  })
})
