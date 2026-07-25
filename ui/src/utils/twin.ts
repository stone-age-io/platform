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
 * Keys are `<kind>.<code>.<prop>` by convention (`thing.S01.temp`), and the two
 * buckets pair on the SAME key. Nothing parses them except to build a watch
 * prefix — direction lives in the bucket, so keys carry no sync bookkeeping.
 *
 * Values may be primitives or whole JSON objects; `twinDrift` below handles
 * both with one rule. The twin view is `KvDashboard` with its `desiredBucket`
 * prop set — deliberately the same browser used for every other bucket, rather
 * than a second one that has to reinvent tree view, filtering, history and the
 * responsive detail drawer.
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

function isPlainObject(v: any): v is Record<string, any> {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

function deepEqual(a: any, b: any): boolean {
  if (a === b) return true
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((x, i) => deepEqual(x, b[i]))
  }
  if (isPlainObject(a) && isPlainObject(b)) {
    const ka = Object.keys(a)
    return (
      ka.length === Object.keys(b).length && ka.every((k) => k in b && deepEqual(a[k], b[k]))
    )
  }
  return false
}

/**
 * Compare a desired value against what the device reports, returning the paths
 * that do not match. Empty result means the assertion holds.
 *
 * A desired value is a **partial assertion**, not a replacement. On objects,
 * only the keys present in `desired` are checked — extra keys in `reported` are
 * nobody's business. So `{arm: "armed"}` against a twelve-field object says "I
 * care about `arm`" and stays quiet about the rest.
 *
 * The alternative, full-replacement equality, rots: the day a device starts
 * reporting one extra field, every desired value set months ago flips to
 * differing, and an indicator that cries wolf gets ignored within a week.
 *
 * Subset semantics apply to OBJECTS ONLY. Arrays and scalars compare exactly —
 * you can omit a field you don't care about, but not an array element, because
 * "this array contains at least these items, somewhere" is a different and much
 * more ambiguous claim.
 *
 * Primitives fall out as the degenerate case: `true` vs `true` is a comparison
 * with no keys to recurse into. One rule covers both shapes.
 */
export function twinDrift(desired: any, reported: any, path = ''): string[] {
  if (isPlainObject(desired) && isPlainObject(reported)) {
    const out: string[] = []
    for (const k of Object.keys(desired)) {
      const p = path ? `${path}.${k}` : k
      if (!(k in reported)) {
        out.push(p)
        continue
      }
      out.push(...twinDrift(desired[k], reported[k], p))
    }
    return out
  }
  return deepEqual(desired, reported) ? [] : [path || 'value']
}
