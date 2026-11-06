import type { Location } from '@/types/pocketbase'

/**
 * A location flattened out of the parent/child tree, in display order.
 *
 * A picker indents by `depth` while browsing and shows `path` while filtering,
 * where the parents are not in the result.
 */
export interface LocationNode {
  id: string
  name: string
  depth: number
  /** Ancestor names, outermost first: "Building A › Floor 2". Empty for roots. */
  path: string
  /**
   * Ancestor ids, outermost first. Lets a parent picker refuse the record's own
   * subtree, which would create a cycle.
   */
  ancestorIds: string[]
  /** Parent is missing from the input set, or the tree has a cycle. */
  orphan: boolean
}

const NAME_FALLBACK = 'Unnamed'
const PATH_SEPARATOR = ' › '

/**
 * Flatten locations into display order: roots alphabetically, each followed by
 * its subtree. Anything unreachable (parent not in `items`, or a cycle) is
 * appended flat and flagged `orphan` rather than dropped, so it stays pickable.
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
    // A parent outside this set: the orphan pass below picks it up.
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
