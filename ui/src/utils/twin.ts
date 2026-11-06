/**
 * The digital twin ("Live State") KV convention, in one place.
 *
 * Two buckets per organization, split by who owns the data:
 *
 *   twin           reported state. The device writes it; it flows edge -> hub.
 *   twin_desired   desired state. The operator writes it; it flows hub -> edge.
 *
 * One writer and one direction per bucket is the whole safety property: a
 * bucket written from both ends oscillates, the two values swapping across the
 * link indefinitely. Encoding the owner in the key instead
 * (`thing.S01.state.temp`) was tried and reverted: it taxes every key and
 * leaves a mistyped segment silently unsynced.
 *
 * So **reported state is read-only in the UI**: the edge overwrites any edit on
 * the next sync. Desired state is the operator's writable control.
 *
 * Keys are `<kind>.<code>.<prop>` (`thing.S01.temp`), and the two buckets pair
 * on the SAME key. Keys carry no sync bookkeeping; direction is the bucket.
 *
 * What goes where:
 *
 *   reported state         `twin` KV, edge -> hub (relayed up).
 *   setpoints and config   `twin_desired` KV, hub -> edge. Durable: a device
 *                          booting after days offline reads it from its local
 *                          mirror.
 *   commands ("reboot")    a NATS message on `cmd.>`, NOT a durable KV value.
 *   ranges, thresholds,    a rule-router rule reading `twin`. Not here.
 *   alarms, hysteresis
 *
 * PAIR DESIRED WITH AN ECHO, NEVER WITH A MEASUREMENT. A desired key should
 * pair with a reported key the device echoes back to acknowledge it
 * (`setpoint`, `mode`), so "differs" means the device has not accepted it. Desired
 * `temp = 20` against a measured `20.3` never converges, and no tolerance fixes
 * that in general; an alarm range is a rule over `twin`.
 *
 * The twin view is `KvDashboard` with `desiredBucket` set, the same browser used
 * for every bucket.
 *
 * Keep TWIN_BUCKET_CONFIG in step with bucketConfig() (formerly
 * twinBucketConfig()) in the agent repo (internal/edge/twin.go): both create
 * these buckets, and whoever gets there first defines them.
 */

/** Reported state, written by devices. Read-only in the console. */
export const TWIN_BUCKET = 'twin'

/** Desired state, written by operators. The writable half. */
export const TWIN_DESIRED_BUCKET = 'twin_desired'

/** Retention for both twin buckets. Mirrors the agent's internal/edge/twin.go. */
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
 * The paths where `reported` does not match `desired`; empty means it holds.
 *
 * A desired value is a **partial assertion**: on objects only the keys in
 * `desired` are checked, so a device reporting an extra field does not flip
 * every old desired value to differing. Subset semantics apply to OBJECTS ONLY;
 * arrays and scalars compare exactly. A top-level scalar mismatch has path ''.
 *
 * DO NOT ADD OPERATORS (`$gt`, ranges, tolerances): that is a rules engine in a
 * KV browser, and rule-router already is one. If equality feels wrong for a
 * key, it is paired with a measurement instead of an echo; fix the pairing.
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

/** The value at a `twinDrift` path ('' is the whole value), for a before/after. */
export function valueAtPath(obj: any, path: string): any {
  if (!path) return obj
  return path.split('.').reduce((acc, k) => (acc == null ? undefined : acc[k]), obj)
}
