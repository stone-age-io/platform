// ui/src/utils/clientSort.ts

/**
 * Order an array by a PocketBase-style sort term (`name`, `-bytes`).
 *
 * ResponsiveList emits that syntax because every other list in this console
 * sorts on the SERVER, and the difference is not stylistic. A PocketBase-backed
 * list holds one page, so ordering it in the browser reorders twenty rows and
 * answers a different question than the reader asked -- silently, and most
 * convincingly on page one. The two JetStream lists are the exception that makes
 * this safe: `listStreams()` and `listKvBuckets()` return everything the account
 * has, and the paging is a slice taken afterwards. Ordering the whole set in
 * memory is the same answer the server would have given.
 *
 * So the rule for a caller is the one thing worth getting right: sort the
 * FILTERED set and slice the result. Sorting the already-sliced page compiles,
 * renders, and orders a page that was already the wrong twenty rows -- page one
 * included, not just page two. What hides it is a set small enough to fit on one
 * page, where the slice IS the whole set and the two are identical: that is
 * every dev environment and most real accounts, so the mistake ships looking
 * perfect and misbehaves only for whoever has the most data.
 *
 * Do not reach for this from a view backed by `usePagination` -- add
 * `sortable:` to the column and let the query do it.
 */
export function sortBy<T>(items: readonly T[], term: string): T[] {
  const list = [...items]
  if (!term) return list

  const desc = term.startsWith('-')
  const field = desc ? term.slice(1) : term
  const dir = desc ? -1 : 1

  // The direction is folded into the comparator rather than applied as a
  // `.reverse()` afterwards. Array.prototype.sort is stable, and reversing
  // undoes that for ties: rows holding the same value would swap places every
  // time the column was flipped, which reads as data moving on its own. Both
  // JetStream lists have columns full of ties -- retention and storage are
  // three- and two-value enums.
  list.sort((a, b) => dir * compare((a as Record<string, unknown>)[field], (b as Record<string, unknown>)[field]))
  return list
}

function compare(a: unknown, b: unknown): number {
  // Numbers numerically. The `numeric: true` collation below is NOT a substitute
  // for this, though it looks like one: it reads digit runs as numbers, so it
  // gets non-negative integers right on its own and a test written against
  // message counts alone cannot tell the two paths apart (this one was, and
  // could not). Where it parts company is decimals and signs -- it compares
  // '1.5' against '1.25' run by run, finds 5 against 25, and reports 1.5 as the
  // smaller; it reads the '-' in '-5' as punctuation and drops it. Today's
  // callers only ever pass non-negative integers, so this line changes no
  // column currently on screen. It is here so the function is right about
  // numbers rather than right about the numbers it happens to be handed.
  if (typeof a === 'number' && typeof b === 'number') return a - b

  // Everything else as text, arrays included -- String(['a.>', 'b.>']) joins on
  // ',' which is what the Subjects column already renders. `numeric` so
  // `stream_2` comes before `stream_10`, since NATS names are routinely
  // numbered and lexical order puts 10 first.
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}
