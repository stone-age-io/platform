import { ref, onUnmounted } from 'vue'
import { useNatsStore } from '@/stores/nats'

/**
 * Which of this organization's things currently have a NATS leaf node attached
 * to the hub, asked of NATS itself.
 *
 * There is no separate screen for this and no separate record behind it: a site
 * IS a Thing, so its page is the Thing's page and this is one more fact shown
 * there, beside its NATS identity.
 *
 * There is deliberately no heartbeat behind this and no platform route. A
 * heartbeat travels over the very link that breaks, so its absence cannot tell
 * "edge box down" from "WAN down" from "agent crashed" — and the platform
 * server could not read one anyway, since it holds the NATS operator and no
 * credential inside any organization's account. The hub always knows which
 * leaves are attached to it, so the browser asks the hub, over the connection
 * the console already has as the logged-in user.
 *
 * `$SYS.REQ.ACCOUNT.PING.CONNZ` is the account-scoped endpoint: every account
 * carries its own $SYS subject space, so this answers for THIS organization and
 * nothing else. The operator-only `$SYS.REQ.SERVER.PING.*` endpoints, which
 * would span every tenant, are not reachable from here — the server enforces
 * that, and internal/health/leaf_visibility_test.go pins both halves against a
 * real hub with a real leaf attached.
 *
 * THE TRAP, if this ever starts timing out for one org and not another: in NATS
 * a publish DENY beats a publish ALLOW. A nats_roles entry carrying `$SYS.>` in
 * its publish deny list cannot reach these endpoints no matter what its allow
 * list says, and the symptom is a plain request timeout — the actual reason
 * arrives asynchronously on the connection's error handler and never on the
 * request. Deny `$SYS.REQ.SERVER.>` instead. See internal/demoseed/contract.go.
 */

/** A leaf connection as CONNZ reports it, trimmed to what the console shows. */
export interface LeafConnection {
  /** The leaf server's `server_name`, which the agent sets to the Thing's code. */
  name: string
  /** RFC3339; when this leaf attached to the hub. */
  start?: string
  /** Human-readable uptime, e.g. "3d2h11m". */
  uptime?: string
  rtt?: string
}

/** The request is cheap, but not free: a whole account's connection list. */
const REFRESH_MS = 15_000

/**
 * More than one server can answer (a clustered hub), so replies are collected
 * for a short window rather than taking the first. Long enough for a LAN or
 * WAN round trip, short enough that a dead bus does not hold a view blank.
 */
const GATHER_MS = 1_500

export function useLeafConnections() {
  const natsStore = useNatsStore()

  /** Keyed by leaf server name, which is the Thing's code. */
  const connections = ref<Map<string, LeafConnection>>(new Map())
  const loading = ref(false)

  /**
   * Null until a request has actually completed, which is what separates "no
   * leaf attached" from "we could not look". Without it a disconnected console
   * would report every site as having nothing attached, which is the failure
   * this whole approach exists to avoid.
   */
  const lastAnswer = ref<Date | null>(null)

  let timer: number | null = null
  let generation = 0

  async function refresh() {
    const nc = natsStore.nc
    if (!nc || nc.isClosed()) {
      connections.value = new Map()
      lastAnswer.value = null
      return
    }

    const mine = ++generation
    loading.value = true

    const found = new Map<string, LeafConnection>()
    try {
      const replies = await nc.requestMany('$SYS.REQ.ACCOUNT.PING.CONNZ', JSON.stringify({}), {
        maxWait: GATHER_MS,
      })
      for await (const m of replies) {
        let body: any
        try {
          body = m.json()
        } catch {
          continue // a malformed reply from one server must not blank the view
        }
        for (const c of body?.data?.connections ?? []) {
          // Every other kind on this list is a client: the org's own devices and
          // browser sessions. They have their own screens; this one is sites.
          if (c?.kind !== 'Leafnode' || !c?.name) continue
          found.set(c.name, { name: c.name, start: c.start, uptime: c.uptime, rtt: c.rtt })
        }
      }
    } catch {
      // A timeout means "could not look", not "nothing attached". Leave the
      // previous answer in place rather than flipping every site to offline.
      if (mine === generation) loading.value = false
      return
    }

    if (mine !== generation) return // a later refresh already answered
    connections.value = found
    lastAnswer.value = new Date()
    loading.value = false
  }

  function start() {
    void refresh()
    if (timer === null) timer = window.setInterval(() => void refresh(), REFRESH_MS)
  }

  function stop() {
    if (timer !== null) {
      window.clearInterval(timer)
      timer = null
    }
    generation++
  }

  onUnmounted(stop)

  return { connections, loading, lastAnswer, refresh, start, stop }
}

export type LeafConnectionState = 'attached' | 'none' | 'unknown'

/**
 * Derive one thing's state. `code` is the Thing's code, which the agent writes
 * into the leaf's `server_name` and its JetStream domain — the same value, by
 * construction, because GET /api/me/leaf-config computes the domain from the
 * code rather than storing a second copy that could disagree.
 */
export function leafConnectionState(
  code: string,
  connections: Map<string, LeafConnection>,
  lastAnswer: Date | null,
): LeafConnectionState {
  if (!code || lastAnswer === null) return 'unknown'
  return connections.has(code) ? 'attached' : 'none'
}
