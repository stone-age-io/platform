import { describe, it, expect } from 'vitest'
import { inferSchema } from './inferSchema'
import { isFormCompatible } from '@/components/things/schemaFields'

// The invariant worth pinning is not the shape of any one inference — it is that
// what comes out stays editable in SchemaBuilder. `applyInferred` toasts "review
// before saving" and drops the user straight into the Form tab, so an inferred
// schema the form refuses is a contradiction in a single click.

describe('inferSchema', () => {
  it('infers a flat sample the builder can edit', () => {
    const schema = inferSchema({
      asset_tag: 'NW-0142',
      last_service: '2026-03-14',
      warranty_months: 36,
      setpoint_c: 4.5,
      sprinklered: true,
    })
    expect(isFormCompatible(schema)).toBe(true)
    expect(schema.properties).toEqual({
      asset_tag: { type: 'string' },
      last_service: { type: 'string', format: 'date' },
      warranty_months: { type: 'integer' },
      setpoint_c: { type: 'number' },
      sprinklered: { type: 'boolean' },
    })
  })

  it('marks every observed key required, because one sample cannot say otherwise', () => {
    expect(inferSchema({ a: 1, b: 2 }).required).toEqual(['a', 'b'])
  })

  it('infers string for a null, so the sample stays editable in the form', () => {
    // The literal truth is {type: 'null'}, which the builder has no type for —
    // it would fail isFormCompatible and raise the banner on a fresh inference.
    // A null in a sample means "present and empty when I looked", not "only ever
    // null".
    const schema = inferSchema({ note: null })
    expect(schema.properties.note).toEqual({ type: 'string' })
    expect(isFormCompatible(schema)).toBe(true)
  })

  it('recognises the string formats the form renders as pickers', () => {
    const schema = inferSchema({
      at: '2026-03-14T09:30:00Z',
      on: '2026-03-14',
      who: 'ops@northwind.test',
      where: 'https://example.test/a',
      id: '0f8fad5b-d9cb-469f-a165-70867728950e',
    })
    expect(Object.values(schema.properties).map((p: any) => p.format)).toEqual([
      'date-time', 'date', 'email', 'uri', 'uuid',
    ])
    expect(isFormCompatible(schema)).toBe(true)
  })

  it('still yields anyOf for a mixed array, which the form correctly refuses', () => {
    // Not a gap to close: the builder has no way to say "one of these", so the
    // banner pointing at the JSON tab is the honest answer.
    const schema = inferSchema({ readings: [1, 'high'] })
    expect(schema.properties.readings.items.anyOf).toHaveLength(2)
    expect(isFormCompatible(schema)).toBe(false)
  })
})
