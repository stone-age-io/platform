// Liveness derivation for leaf nodes. leaf-sync writes a heartbeat into the
// `leaf_status` NATS KV bucket once per sync cycle; the UI reads it and decides
// whether a node looks alive. There is no PocketBase mirror — the absence of a
// recent beat is the offline signal.

export interface LeafHeartbeat {
  code: string
  version: string
  ts: string // RFC3339 timestamp of the beat
  interval: string // Go duration string, e.g. "30s" — the expected cadence
  synced: Record<string, number> // per-collection record counts
  errors: string[] // per-collection sync errors, empty when healthy
}

export type LeafStatusState = 'online' | 'offline' | 'unknown'

// A node is considered offline once it misses this many heartbeat intervals.
const MISSED_INTERVALS_BEFORE_OFFLINE = 3
const DEFAULT_INTERVAL_MS = 30_000

const UNIT_MS: Record<string, number> = {
  ns: 1e-6, us: 1e-3, 'µs': 1e-3, ms: 1, s: 1000, m: 60_000, h: 3_600_000,
}

/**
 * Parse a Go `time.Duration.String()` value (e.g. "30s", "1m30s", "500ms") to
 * milliseconds. Returns 0 for anything unparseable so callers can fall back.
 */
export function parseGoDuration(d: string): number {
  if (!d) return 0
  let ms = 0
  let matched = false
  const re = /([0-9]*\.?[0-9]+)(ns|us|µs|ms|s|m|h)/g
  let m: RegExpExecArray | null
  while ((m = re.exec(d)) !== null) {
    matched = true
    ms += parseFloat(m[1]) * (UNIT_MS[m[2]] ?? 0)
  }
  return matched ? ms : 0
}

/**
 * Derive a leaf node's liveness from its latest heartbeat.
 * - `unknown` — not connected to NATS, or no beat seen yet (can't tell).
 * - `offline` — last beat older than 3× its sync interval.
 * - `online`  — a recent beat.
 */
export function leafStatus(
  hb: LeafHeartbeat | undefined,
  connected: boolean,
  now: number = Date.now(),
): LeafStatusState {
  if (!connected || !hb?.ts) return 'unknown'
  const beat = Date.parse(hb.ts)
  if (isNaN(beat)) return 'unknown'
  const interval = parseGoDuration(hb.interval) || DEFAULT_INTERVAL_MS
  return now - beat > MISSED_INTERVALS_BEFORE_OFFLINE * interval ? 'offline' : 'online'
}
