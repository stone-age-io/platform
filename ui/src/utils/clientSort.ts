/**
 * Order an array by a PocketBase-style sort term (`name`, `-bytes`).
 *
 * Only for lists that hold the whole set, like the JetStream lists
 * (`listStreams()`, `listKvBuckets()` return everything; paging is a slice).
 * Sort the FILTERED set, then slice; sorting an already-sliced page gives the
 * wrong rows, which is invisible while everything fits on one page.
 *
 * A view backed by `usePagination` must sort on the server: add `sortable:` to
 * the column instead.
 */
export function sortBy<T>(items: readonly T[], term: string): T[] {
  const list = [...items]
  if (!term) return list

  const desc = term.startsWith('-')
  const field = desc ? term.slice(1) : term
  const dir = desc ? -1 : 1

  // Direction lives in the comparator, not a `.reverse()` afterwards, so the
  // stable sort keeps ties in place when the column is flipped.
  list.sort((a, b) => dir * compare((a as Record<string, unknown>)[field], (b as Record<string, unknown>)[field]))
  return list
}

function compare(a: unknown, b: unknown): number {
  // Not left to the `numeric: true` collation below, which gets decimals and
  // negative numbers wrong.
  if (typeof a === 'number' && typeof b === 'number') return a - b

  // Arrays compare as String(), the same text the cell renders. `numeric` so
  // `stream_2` sorts before `stream_10`.
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}
