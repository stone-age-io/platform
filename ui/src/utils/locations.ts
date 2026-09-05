// ui/src/utils/locations.ts
import type { Location } from '@/types/pocketbase'

/**
 * A location flattened out of the parent/child tree, in display order.
 *
 * `depth` and `path` say the same thing two ways on purpose. A picker indents by
 * `depth` while you browse, and swaps to `path` while you filter -- indenting a
 * filtered subset is meaningless, because the parents that gave the indent its
 * meaning are not in the result. The path is also what tells three "Room 12"s
 * apart.
 */
export interface LocationNode {
  id: string
  name: string
  depth: number
  /** Ancestor names, outermost first: "Building A > Floor 2". Empty for roots. */
  path: string
  /**
   * Ancestor ids, outermost first. This is what lets a parent picker refuse the
   * subtree below the record being edited: making a location a child of its own
   * descendant creates a cycle, and the walk below then cannot reach either of
   * them, so both drop out of the tree and reappear as orphans.
   */
  ancestorIds: string[]
  /** Parent is missing from the input set, or the tree has a cycle. */
  orphan: boolean
}

const NAME_FALLBACK = 'Unnamed'
const PATH_SEPARATOR = ' › ' // ›

/**
 * Flatten locations into display order: roots alphabetically, each followed by
 * its subtree.
 *
 * Anything the walk cannot reach -- a record whose parent was filtered out of
 * `items`, or a cycle -- is appended flat and flagged `orphan`. Dropping those
 * would make a real location unpickable, which is worse than showing it without
 * its place in the tree.
 */
export function flattenLocationTree(items: Location[]): LocationNode[] {
  const childrenOf = new Map<string, Location[]>()
  const roots: Location[] = []
  const known = new Set(items.map(i => i.id))

  for (const item of items) {
    if (!item.parent) {
      roots.push(item)
    } else if (known.has(item.parent)) {
      const siblings = childrenOf.get(item.parent) || []
      siblings.push(item)
      childrenOf.set(item.parent, siblings)
    }
    // A parent outside this set means the orphan pass below picks it up.
  }

  const byName = (a: Location, b: Location) => (a.name || '').localeCompare(b.name || '')
  const result: LocationNode[] = []
  const seen = new Set<string>()

  function walk(node: Location, depth: number, ancestors: string[], ancestorIds: string[]) {
    if (seen.has(node.id)) return // cheap insurance against a cycle among roots
    seen.add(node.id)

    const name = node.name || NAME_FALLBACK
    result.push({
      id: node.id,
      name,
      depth,
      path: ancestors.join(PATH_SEPARATOR),
      ancestorIds,
      orphan: false,
    })

    for (const child of [...(childrenOf.get(node.id) || [])].sort(byName)) {
      walk(child, depth + 1, [...ancestors, name], [...ancestorIds, node.id])
    }
  }

  for (const root of [...roots].sort(byName)) walk(root, 0, [], [])

  for (const item of items) {
    if (seen.has(item.id)) continue
    seen.add(item.id)
    result.push({
      id: item.id,
      name: item.name || NAME_FALLBACK,
      depth: 0,
      path: '',
      ancestorIds: [],
      orphan: true,
    })
  }

  return result
}
