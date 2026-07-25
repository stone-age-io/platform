/**
 * The digital twin ("Live State") KV convention, in one place.
 *
 * Two buckets per organization, split by who owns the data:
 *
 *   twin           reported state. The device writes it; it flows edge -> hub.
 *   twin_desired   desired state. The operator writes it; it flows hub -> edge.
 *
 * One writer per bucket, one direction per bucket. That is the whole safety
 * property, and it is structural rather than something anyone has to remember.
 * A single bucket written from both ends does not merely pick a loser on a
 * conflict — it oscillates, with the two values swapping across the link
 * indefinitely. Encoding the owner in the key instead (`thing.S01.state.temp`)
 * buys the same property but taxes every key in the system and leaves a mistyped
 * segment silently unsynced.
 *
 * The practical consequence for the UI: **reported state is read-only here.**
 * The edge overwrites it. Offering an edit button on a reported key is a lie —
 * the value comes back on the next sync. Desired state is the writable surface,
 * and it is the operator's actual control.
 *
 * Keys are `<kind>.<code>.<prop>` by convention (`thing.S01.temp`). Nothing
 * parses them except to build a watch prefix — direction lives in the bucket, so
 * keys carry no sync bookkeeping.
 *
 * Keep TWIN_BUCKET_CONFIG in step with twinBucketConfig() in
 * internal/leafsync/twin.go — both the console and leaf-sync create these
 * buckets, and whoever gets there first defines them.
 */

/** Reported state, written by devices. Read-only in the console. */
export const TWIN_BUCKET = 'twin'

/** Desired state, written by operators. The writable half. */
export const TWIN_DESIRED_BUCKET = 'twin_desired'

/** Retention for both twin buckets. Mirrors internal/leafsync/twin.go. */
export const TWIN_BUCKET_CONFIG = {
  history: 10,
  storage: 'file',
} as const

export const TWIN_BUCKET_DESCRIPTIONS: Record<string, string> = {
  [TWIN_BUCKET]: 'Digital twin: reported state (written at the edge)',
  [TWIN_DESIRED_BUCKET]: 'Digital twin: desired state (written by operators)',
}

export type TwinKind = 'thing' | 'location'

/** Key prefix for one entity, e.g. `thing.S01`. Used to scope a watch. */
export function twinPrefix(kind: TwinKind, code: string): string {
  return `${kind}.${code}`
}

/**
 * Strip the entity prefix for display: `thing.S01.temp` -> `temp`. The card
 * already says which Thing it is; repeating the prefix on every row is noise.
 */
export function twinPropName(key: string, prefix: string): string {
  return key.startsWith(`${prefix}.`) ? key.slice(prefix.length + 1) : key
}
