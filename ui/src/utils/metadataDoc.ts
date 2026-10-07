// A type's `metadata_schema` is a FORM HINT, not a contract. Nothing on the
// server validates `metadata` against it, a record may carry keys the schema
// does not name, and the form never blocks a save on it — a rule one client
// enforces and the API does not is half a rule, and half a rule invites
// someone to finish it in a hook.
//
// So MetadataEditor edits one document through two controls: typed inputs for
// the keys the schema describes, and plain key/value rows for everything else.
// These functions are the split and the merge, kept pure so the round trip is
// testable — any key that falls between the two halves is deleted by the next
// save, silently.

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
 * Rebuild the document from its two halves: the described keys as they stand
 * in `doc`, then the rows in order. A blank row key is a row in progress, not
 * an instruction to write "", so it is skipped; on a repeated key the later one
 * wins, which is what the JSON view would produce from the same text. An empty
 * result is `null`, the same as clearing the JSON view.
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
