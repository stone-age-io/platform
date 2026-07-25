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
 * ---------------------------------------------------------------------------
 * WHAT BELONGS HERE, AND WHAT DOES NOT
 *
 *   reported state         `twin` KV, edge -> hub.
 *   setpoints and config   `twin_desired` KV, hub -> edge. Durable: a device
 *                          that boots after three days offline reads the
 *                          current value out of its local mirror.
 *   commands ("reboot")    a NATS message on `cmd.>`, NOT a KV value. A durable
 *                          "reboot now" sitting in a bucket forever is a bug.
 *   ranges, thresholds,    a rule-router rule reading `twin`. Not this file.
 *   alarms, hysteresis
 *
 * The third and fourth rows are the ones people try to put in `twin_desired`.
 * Both make it worse; see the note on twinDrift below.
 *
 * PAIR DESIRED WITH AN ECHO, NEVER WITH A MEASUREMENT.
 *
 * A desired key should pair with a reported key the device *echoes back* to
 * acknowledge an instruction — `thing.S01.setpoint`, `thing.S01.mode`. Those
 * converge exactly, so equality is the right question and "differs" means "the
 * device has not accepted my instruction", which is actionable.
 *
 * A measurement is the wrong partner. Desired `temp = 20` against reported
 * `temp = 20.3` differs, and keeps differing, because it compares an
 * instruction to a continuous reading — two things that never converge. No
 * tolerance fixes that in general: the right tolerance is different per
 * property, per device, per season. If you want "alarm when temp leaves
 * 18-22", that is a rule over `twin`, and it does not involve `twin_desired`
 * at all.
 * ---------------------------------------------------------------------------
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
 * with no keys to recurse into. One rule covers both shapes. The path for that
 * case is the empty string — the whole value, no sub-path.
 *
 * DO NOT ADD OPERATORS. The next reasonable-sounding request is `{$gt: 30}`,
 * then `{$between: [18, 22]}`, then tolerances, and at that point this function
 * is a rules engine living inside a KV browser — and we already own a rules
 * engine that does it properly. Every one of those asks is a threshold over
 * REPORTED state, which is rule-router's job; none of them is a desired value.
 * If equality feels wrong for a key, the key is paired with a measurement
 * instead of an echo (see the header) — fix the pairing, not the comparison.
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
  return deepEqual(desired, reported) ? [] : [path]
}

/**
 * Read the value at a dotted path produced by `twinDrift`. The empty path means
 * the whole value, which is how the top-level-scalar case comes back.
 *
 * Lets a caller turn drift paths into an actual before/after — naming the paths
 * alone just tells someone where to go looking.
 */
export function valueAtPath(obj: any, path: string): any {
  if (!path) return obj
  return path.split('.').reduce((acc, k) => (acc == null ? undefined : acc[k]), obj)
}
