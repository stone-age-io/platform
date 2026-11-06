// A type's `metadata_schema` is a FORM HINT, not a contract: nothing on the
// server validates `metadata` against it, records may carry other keys, and the
// form never blocks a save (see ui/CLAUDE.md before adding validation).
//
// MetadataEditor edits one document through typed inputs for the described keys
// and key/value rows for the rest. These are the split and the merge; any key
// that falls between the two halves is silently deleted on the next save.

export interface MetadataRow {
  key: string
  value: any
}

/**
 * The keys a schema describes. Empty for no schema, a non-object schema, or
 * one with no properties — in which case every key belongs to the rows.
 */
export function describedKeys(schema: any): Set<string> {
  if (!schema || schema.type !== 'object' || !schema.properties) return new Set()
  return new Set(Object.keys(schema.properties))
}

/** The entries the row editor owns: every key the schema does not describe. */
export function undescribedEntries(
  doc: Record<string, any>,
  described: Set<string>,
): [string, any][] {
  return Object.entries(doc).filter(([k]) => !described.has(k))
}

/**
 * Rebuild the document: the described keys from `doc`, then the rows in order.
 * A blank row key (a row in progress) is skipped; a repeated key's later value
 * wins, as in the JSON view. An empty result is `null`.
 */
export function mergeRows(
  doc: Record<string, any>,
  described: Set<string>,
  rows: MetadataRow[],
): Record<string, any> | null {
  const next: Record<string, any> = {}
  for (const k of Object.keys(doc)) {
    if (described.has(k)) next[k] = doc[k]
  }
  for (const r of rows) {
    const k = r.key.trim()
    if (!k) continue
    next[k] = r.value
  }
  return Object.keys(next).length ? next : null
}
