import { describe, it, expect } from 'vitest'
import {
  type Field,
  emptyField,
  duplicateNames,
  isFormCompatible,
  schemaToFields,
  fieldsToSchema,
} from './schemaFields'

// These functions are the whole contract between the JSON tab and the Form tab
// of MetadataSchemaCard: whatever `schemaToFields` cannot read, `fieldsToSchema`
// cannot write back, and the user loses it on their next edit — silently, since
// `isFormCompatible` decides the warning banner and knows nothing about which
// keywords survive the trip.
//
// That is exactly how `title` went missing: it is the label every seeded type in
// internal/demoseed/data.go uses, JsonSchemaForm and MetadataEditor both render
// it, the builder round-tripped it to nothing, and the banner said the schema
// was fine. Nothing in `vue-tsc && vite build` can see that, hence a spec.

/** Round-trip a schema through the form's internal representation. */
function roundTrip(schema: Record<string, any>): Record<string, any> {
  return fieldsToSchema(schemaToFields(schema))
}

describe('round-trip fidelity', () => {
  it('preserves title — the label, not the key', () => {
    const schema = {
      type: 'object',
      properties: { dock_doors: { type: 'integer', title: 'Dock doors' } },
    }
    expect(roundTrip(schema)).toEqual(schema)
  })

  it('preserves a seeded type verbatim', () => {
    // Copied in shape from the `warehouse` fixture in internal/demoseed/data.go:
    // title on every property, a format, an enum, and a required list.
    const schema = {
      type: 'object',
      properties: {
        dock_doors: { type: 'integer', title: 'Dock doors' },
        sqft: { type: 'integer', title: 'Square feet' },
        temp_class: {
          type: 'string',
          title: 'Temperature class',
          enum: ['ambient', 'chilled', 'frozen'],
        },
        sprinklered: { type: 'boolean', title: 'Sprinklered' },
        last_inspected: { type: 'string', format: 'date', title: 'Last inspected' },
      },
      required: ['dock_doors'],
    }
    expect(roundTrip(schema)).toEqual(schema)
  })

  it('keeps title and description apart', () => {
    // They render in different places — title replaces the label, description is
    // fine print under the input — so collapsing one into the other is a
    // regression the type system cannot see.
    const schema = {
      type: 'object',
      properties: {
        vin: { type: 'string', title: 'VIN', description: 'Vehicle identification number' },
      },
    }
    const out = roundTrip(schema)
    expect(out.properties.vin.title).toBe('VIN')
    expect(out.properties.vin.description).toBe('Vehicle identification number')
  })

  it('preserves numeric bounds and a numeric enum', () => {
    const schema = {
      type: 'object',
      properties: {
        setpoint_c: { type: 'number', title: 'Setpoint (C)', minimum: -30, maximum: 40 },
        lines: { type: 'integer', title: 'Lines', enum: [1, 2, 3] },
      },
    }
    expect(roundTrip(schema)).toEqual(schema)
  })

  it('preserves nested objects, including their titles and required lists', () => {
    const schema = {
      type: 'object',
      properties: {
        contact: {
          type: 'object',
          title: 'Site contact',
          properties: {
            name: { type: 'string', title: 'Name' },
            email: { type: 'string', format: 'email', title: 'Email' },
          },
          required: ['name'],
        },
      },
    }
    expect(roundTrip(schema)).toEqual(schema)
  })

  it('preserves array-of-object item properties', () => {
    const schema = {
      type: 'object',
      properties: {
        racks: {
          type: 'array',
          title: 'Racks',
          items: {
            type: 'object',
            properties: {
              label: { type: 'string', title: 'Label' },
              positions: { type: 'integer', title: 'Positions' },
            },
            required: ['label'],
          },
        },
      },
    }
    expect(roundTrip(schema)).toEqual(schema)
  })

  it('omits empty optional keywords rather than writing blanks', () => {
    const schema = { type: 'object', properties: { note: { type: 'string' } } }
    const out = roundTrip(schema)
    expect(out.properties.note).toEqual({ type: 'string' })
    expect('title' in out.properties.note).toBe(false)
    expect('required' in out).toBe(false)
  })

  it('drops a field whose name is blank', () => {
    const f = emptyField()
    f.type = 'string'
    expect(fieldsToSchema([f])).toEqual({ type: 'object', properties: {} })
  })
})

describe('isFormCompatible', () => {
  it('accepts nesting at any depth', () => {
    // There is no depth cap. There used to be one, and it disagreed with
    // SchemaFieldEditor by exactly one level, so a schema the form had just
    // built could not be reopened in the form.
    let node: Record<string, any> = { type: 'string', title: 'Leaf' }
    for (let i = 8; i >= 1; i--) {
      node = { type: 'object', properties: { [`level_${i}`]: node } }
    }
    expect(isFormCompatible(node)).toBe(true)
    expect(roundTrip(node)).toEqual(node)
  })

  it('rejects the keywords the builder genuinely cannot represent', () => {
    for (const bad of ['$ref', 'anyOf', 'oneOf', 'allOf']) {
      expect(isFormCompatible({ type: 'object', properties: { x: { [bad]: 1 } } })).toBe(false)
    }
  })

  it('rejects a node that is not an object schema', () => {
    expect(isFormCompatible(null)).toBe(false)
    expect(isFormCompatible({ type: 'string' })).toBe(false)
    expect(isFormCompatible({ type: 'object', properties: { x: { type: 'null' } } })).toBe(false)
  })

  it('rejects an array of arrays, which the item editor cannot express', () => {
    expect(
      isFormCompatible({
        type: 'object',
        properties: { x: { type: 'array', items: { type: 'array' } } },
      }),
    ).toBe(false)
  })

  it('accepts a schema with no properties yet', () => {
    expect(isFormCompatible({ type: 'object' })).toBe(true)
    expect(isFormCompatible({ type: 'object', properties: {} })).toBe(true)
  })
})

describe('duplicateNames', () => {
  const named = (...names: string[]): Field[] =>
    names.map(n => ({ ...emptyField(), name: n }))

  it('reports a name used twice', () => {
    expect([...duplicateNames(named('a', 'b', 'a'))]).toEqual(['a'])
  })

  it('ignores blank and whitespace-only names', () => {
    expect(duplicateNames(named('', '  ', '')).size).toBe(0)
  })

  it('compares trimmed names, since fieldsToSchema keys on the trimmed value', () => {
    expect([...duplicateNames(named('a', ' a '))]).toEqual(['a'])
  })

  it('is silent when every name is distinct', () => {
    expect(duplicateNames(named('a', 'b', 'c')).size).toBe(0)
  })

  it('matches what fieldsToSchema actually does with a collision', () => {
    const [first, second] = named('a', 'a')
    first.title = 'First'
    second.title = 'Second'
    const out = fieldsToSchema([first, second])
    expect(Object.keys(out.properties)).toEqual(['a'])
    expect(out.properties.a.title).toBe('Second')
  })
})
