// Shared types + schema↔fields conversion for SchemaBuilder + SchemaFieldEditor.

export type FieldType = 'string' | 'integer' | 'number' | 'boolean' | 'object' | 'array'

export interface Field {
  name: string
  /**
   * JSON Schema `title` — the human label. Distinct from `description`, which
   * renders as fine print under the input: this one REPLACES the key as the
   * label, which is what makes a member-facing form readable (`dock_doors` →
   * "Dock doors"). Every seeded type in internal/demoseed/data.go uses it and
   * none uses `description`, so a builder that could not round-trip it stripped
   * the labels off the demo data on the first edit.
   */
  title: string
  type: FieldType
  required: boolean
  description: string
  format: string
  enumText: string
  minimum: string
  maximum: string
  children: Field[]
  itemType: FieldType
  itemFormat: string
  itemChildren: Field[]
}

export const TYPES: FieldType[] = ['string', 'integer', 'number', 'boolean', 'object', 'array']
// Array items can't themselves be arrays via the form — keep the builder bounded.
export const ITEM_TYPES: FieldType[] = ['string', 'integer', 'number', 'boolean', 'object']
export const STRING_FORMATS = ['', 'date-time', 'date', 'time', 'email', 'uri', 'uuid']

export function emptyField(): Field {
  return {
    name: '',
    title: '',
    type: 'string',
    required: false,
    description: '',
    format: '',
    enumText: '',
    minimum: '',
    maximum: '',
    children: [],
    itemType: 'string',
    itemFormat: '',
    itemChildren: [],
  }
}

// Sibling property names that appear more than once. `fieldsToSchema` writes
// into a plain object so a duplicate silently overwrites its twin and the field
// the user typed first disappears on save — same failure MetadataEditor already
// warns about for free-form keys, so it gets the same warning here.
export function duplicateNames(list: Field[]): Set<string> {
  const seen = new Set<string>()
  const dupes = new Set<string>()
  for (const f of list) {
    const n = f.name.trim()
    if (!n) continue
    if (seen.has(n)) dupes.add(n)
    seen.add(n)
  }
  return dupes
}

// A schema node is form-compatible when every branch uses only the structures
// the builder can round-trip. Anything else falls through to the JSON view.
//
// There is deliberately NO nesting cap here. One used to live alongside these
// checks and disagreed with the editor by exactly one level, so a schema built
// in the form could not be reopened in the form. The rest of this predicate
// states facts about what the builder can REPRESENT ($ref and the combinators
// have no form equivalent); a depth limit is a rendering preference, and mixing
// the two in one predicate is what produced the off-by-one. The recursion
// terminates on the data either way.
export function isFormCompatible(schema: any): boolean {
  if (!schema || typeof schema !== 'object') return false
  if (schema.type !== 'object') return false
  if (schema.$ref || schema.anyOf || schema.oneOf || schema.allOf) return false
  const props = schema.properties
  if (props == null) return true
  if (typeof props !== 'object') return false
  return Object.values(props).every((raw: any) => isPropCompatible(raw))
}

function isPropCompatible(raw: any): boolean {
  if (!raw || typeof raw !== 'object') return false
  if (raw.$ref || raw.anyOf || raw.oneOf || raw.allOf) return false
  if (!TYPES.includes(raw.type)) return false
  if (raw.type === 'object') {
    if (raw.properties == null) return true
    return isFormCompatible(raw)
  }
  if (raw.type === 'array') {
    const items = raw.items
    if (items == null) return true
    if (typeof items !== 'object') return false
    if (items.$ref || items.anyOf || items.oneOf || items.allOf) return false
    if (!TYPES.includes(items.type)) return false
    if (items.type === 'array') return false
    if (items.type === 'object') {
      if (items.properties == null) return true
      return isFormCompatible(items)
    }
    return true
  }
  return true
}

export function schemaToFields(schema: any): Field[] {
  if (!schema?.properties) return []
  const required = new Set<string>(Array.isArray(schema.required) ? schema.required : [])
  return Object.entries(schema.properties).map(([name, raw]) =>
    schemaNodeToField(name, raw, required.has(name)),
  )
}

function schemaNodeToField(name: string, raw: any, isRequired: boolean): Field {
  const f = emptyField()
  f.name = name
  f.title = raw?.title || ''
  f.type = (TYPES.includes(raw?.type) ? raw.type : 'string') as FieldType
  f.required = isRequired
  f.description = raw?.description || ''
  f.format = raw?.format || ''
  f.enumText = Array.isArray(raw?.enum) ? raw.enum.join(', ') : ''
  f.minimum = raw?.minimum != null ? String(raw.minimum) : ''
  f.maximum = raw?.maximum != null ? String(raw.maximum) : ''

  if (f.type === 'object' && raw?.properties) {
    const req = new Set<string>(Array.isArray(raw.required) ? raw.required : [])
    f.children = Object.entries(raw.properties).map(([n, r]) =>
      schemaNodeToField(n, r, req.has(n)),
    )
  }
  if (f.type === 'array' && raw?.items && typeof raw.items === 'object') {
    const items = raw.items
    f.itemType = (ITEM_TYPES.includes(items.type) ? items.type : 'string') as FieldType
    if (f.itemType === 'string') f.itemFormat = items.format || ''
    if (f.itemType === 'object' && items.properties) {
      const req = new Set<string>(Array.isArray(items.required) ? items.required : [])
      f.itemChildren = Object.entries(items.properties).map(([n, r]) =>
        schemaNodeToField(n, r, req.has(n)),
      )
    }
  }
  return f
}

export function fieldsToSchema(list: Field[]): Record<string, any> {
  const properties: Record<string, any> = {}
  const required: string[] = []
  for (const f of list) {
    const name = f.name.trim()
    if (!name) continue
    properties[name] = fieldToSchemaNode(f)
    if (f.required) required.push(name)
  }
  const out: Record<string, any> = { type: 'object', properties }
  if (required.length > 0) out.required = required
  return out
}

function fieldToSchemaNode(f: Field): Record<string, any> {
  const entry: Record<string, any> = { type: f.type }
  if (f.title) entry.title = f.title
  if (f.description) entry.description = f.description

  if (f.type === 'string' && f.format) entry.format = f.format

  if (f.type === 'string' || f.type === 'integer' || f.type === 'number') {
    const enumValues = f.enumText.split(',').map(s => s.trim()).filter(Boolean)
    if (enumValues.length > 0) {
      entry.enum = (f.type === 'integer' || f.type === 'number')
        ? enumValues.map(v => Number(v)).filter(v => !Number.isNaN(v))
        : enumValues
    }
  }

  if (f.type === 'integer' || f.type === 'number') {
    if (f.minimum !== '') {
      const n = Number(f.minimum)
      if (!Number.isNaN(n)) entry.minimum = n
    }
    if (f.maximum !== '') {
      const n = Number(f.maximum)
      if (!Number.isNaN(n)) entry.maximum = n
    }
  }

  if (f.type === 'object') {
    entry.properties = {}
    const required: string[] = []
    for (const child of f.children) {
      const cname = child.name.trim()
      if (!cname) continue
      entry.properties[cname] = fieldToSchemaNode(child)
      if (child.required) required.push(cname)
    }
    if (required.length > 0) entry.required = required
  }

  if (f.type === 'array') {
    const items: Record<string, any> = { type: f.itemType }
    if (f.itemType === 'string' && f.itemFormat) items.format = f.itemFormat
    if (f.itemType === 'object') {
      items.properties = {}
      const required: string[] = []
      for (const child of f.itemChildren) {
        const cname = child.name.trim()
        if (!cname) continue
        items.properties[cname] = fieldToSchemaNode(child)
        if (child.required) required.push(cname)
      }
      if (required.length > 0) items.required = required
    }
    entry.items = items
  }

  return entry
}
