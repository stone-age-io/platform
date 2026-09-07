/**
 * Infer a starting JSON Schema from a sample document.
 *
 * Authoring a schema by hand is the tedious half of describing a record type,
 * and the sample is almost always to hand — a device's payload, a row exported
 * from the system being replaced. This turns one into a draft to edit rather
 * than a blank form.
 *
 * It came from MessageSchemaFormView, which was removed with the
 * `message_schemas` collection. The function itself was never specific to that
 * collection: a metadata_schema is the same kind of document, authored on the
 * thing type and location type forms, so the helper moved here instead of going
 * with its old home.
 *
 * DRAFT, not truth. Every key present in the sample is marked required, because
 * a sample cannot show which keys are optional — one sample is one observation,
 * not a population. Review is part of the flow, which is why the callers say so
 * when they apply it.
 */
export function inferSchema(value: any): Record<string, any> {
  if (value === null) return { type: 'null' }

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

  // Recognised string formats, so a date lands as a date picker in the form
  // renderer rather than a text box.
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
