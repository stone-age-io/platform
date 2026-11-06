/**
 * Infer a draft JSON Schema from a sample document, for the user to review.
 * Every key in the sample is marked required, since one sample cannot show
 * which are optional.
 *
 * The output must stay editable in SchemaBuilder, so a null infers `string`
 * (the builder has no null type). Mixed-type arrays still yield `anyOf`, which
 * the form genuinely cannot represent.
 */
export function inferSchema(value: any): Record<string, any> {
  if (value === null) return { type: 'string' }

  if (Array.isArray(value)) {
    if (value.length === 0) return { type: 'array', items: {} }
    const itemSchemas = value.map(inferSchema)
    const firstKey = JSON.stringify(itemSchemas[0])
    const allSame = itemSchemas.every((s) => JSON.stringify(s) === firstKey)
    return { type: 'array', items: allSame ? itemSchemas[0] : { anyOf: itemSchemas } }
  }

  if (typeof value === 'object') {
    const properties: Record<string, any> = {}
    const required: string[] = []
    for (const [k, v] of Object.entries(value)) {
      properties[k] = inferSchema(v)
      required.push(k)
    }
    return required.length
      ? { type: 'object', properties, required }
      : { type: 'object', properties }
  }

  // Formats, so e.g. a date renders as a date picker.
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value)) return { type: 'string', format: 'date-time' }
    if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return { type: 'string', format: 'date' }
    if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) return { type: 'string', format: 'email' }
    if (/^https?:\/\//.test(value)) return { type: 'string', format: 'uri' }
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) {
      return { type: 'string', format: 'uuid' }
    }
    return { type: 'string' }
  }

  if (typeof value === 'number') {
    return Number.isInteger(value) ? { type: 'integer' } : { type: 'number' }
  }
  if (typeof value === 'boolean') return { type: 'boolean' }

  return {}
}
