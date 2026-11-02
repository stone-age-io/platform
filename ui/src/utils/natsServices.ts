// ui/src/utils/natsServices.ts
//
// NATS micro service discovery (ADR-32): every service subscribes to
// `$SRV.INFO` and `$SRV.STATS`, and to `$SRV.INFO.<name>` for its own name, so
// one request to each, collecting every reply that arrives, lists what is
// running in this account right now. Nothing is stored. The edge agent
// registers as `stone-agent`, which is what makes this a list of live sites as
// well as of services.

import { NoRespondersError, PermissionViolationError } from '@nats-io/nats-core'
import type { NatsConnection } from '@nats-io/nats-core'
import type { ServiceEndpoint, ServiceGroup, ServiceInstance } from '@/types/services'

/** Asked of nats-core per request. Waits up to this long for a first reply,
    then stops 300ms after the last one. */
const MAX_WAIT_MS = 3000

/** The request was refused rather than unanswered: the console's own NATS
    user cannot publish on `$SRV.>`. */
export class DiscoveryDeniedError extends Error {}

async function collect(nc: NatsConnection, subject: string): Promise<any[]> {
  const replies: any[] = []
  try {
    const iter = await nc.requestMany(subject, '', { strategy: 'stall', maxWait: MAX_WAIT_MS })
    for await (const m of iter) {
      try { replies.push(m.json()) } catch { /* not JSON, so not a micro service */ }
    }
  } catch (e) {
    const cause = (e as { cause?: unknown })?.cause ?? e
    if (cause instanceof NoRespondersError) return []
    if (cause instanceof PermissionViolationError) throw new DiscoveryDeniedError(subject)
    throw e
  }
  return replies
}

const count = (n: unknown): number => (typeof n === 'number' ? n : 0)

/**
 * Join INFO and STATS replies on instance id, and each endpoint on its name.
 * INFO is the list; a STATS reply with no INFO beside it is dropped rather than
 * shown half-empty, since the two travel the same path and missing one is the
 * stall timer cutting it short.
 */
export function mergeServiceReplies(infos: any[], stats: any[]): ServiceInstance[] {
  const statsById = new Map(stats.filter(s => s?.id).map(s => [s.id, s]))
  return infos
    .filter(i => i?.id && i?.name)
    .map(i => {
      const s = statsById.get(i.id)
      const statEps: any[] = Array.isArray(s?.endpoints) ? s.endpoints : []
      const endpoints: ServiceEndpoint[] = (Array.isArray(i.endpoints) ? i.endpoints : []).map((e: any) => {
        const es = statEps.find(x => x?.name === e.name)
        return {
          name: e.name ?? '',
          subject: e.subject ?? '',
          queueGroup: e.queue_group ?? '',
          requests: es ? count(es.num_requests) : null,
          errors: es ? count(es.num_errors) : null,
          lastError: es?.last_error ?? '',
          // Nanoseconds on the wire.
          averageMs: es ? count(es.average_processing_time) / 1e6 : null,
        }
      })
      return {
        id: i.id,
        name: i.name,
        version: i.version ?? '',
        description: i.description ?? '',
        metadata: i.metadata ?? {},
        endpoints,
        started: s?.started ?? null,
        requests: s ? statEps.reduce((n, e) => n + count(e?.num_requests), 0) : null,
        errors: s ? statEps.reduce((n, e) => n + count(e?.num_errors), 0) : null,
        lastError: statEps.map(e => e?.last_error).find(Boolean) ?? '',
      }
    })
}

/** One group per service name, its versions ordered by how many instances run them. */
export function groupServices(instances: ServiceInstance[]): ServiceGroup[] {
  const byName = new Map<string, ServiceInstance[]>()
  for (const i of instances) byName.set(i.name, [...(byName.get(i.name) ?? []), i])

  return [...byName].map(([name, list]) => {
    const runs = new Map<string, number>()
    for (const i of list) runs.set(i.version, (runs.get(i.version) ?? 0) + 1)
    const withStats = list.filter(i => i.requests !== null)
    return {
      id: name,
      name,
      // Instances of one service normally agree; the first non-empty one wins.
      description: list.find(i => i.description)?.description ?? '',
      instances: list,
      versions: [...runs].sort((a, b) => b[1] - a[1]).map(([v]) => v),
      requests: withStats.length ? withStats.reduce((n, i) => n + (i.requests ?? 0), 0) : null,
      errors: withStats.length ? withStats.reduce((n, i) => n + (i.errors ?? 0), 0) : null,
    }
  })
}

/**
 * The metadata keys a set of instances carry, in first-seen order: one
 * service's instances share a shape, so its keys can be columns.
 */
export function metadataKeys(instances: ServiceInstance[]): string[] {
  const keys = new Set<string>()
  for (const i of instances) for (const k of Object.keys(i.metadata)) keys.add(k)
  return [...keys]
}

/**
 * A version short enough for a column. A Go pseudo-version
 * (`0.1.1-0.20261013022711-733be73d0e58`) is a base version, a timestamp and a
 * commit; the timestamp is what wrapped it onto five lines, and the base and a
 * short commit are what anyone compares. Anything else is returned as is.
 */
export function shortVersion(v: string): string {
  const m = /^v?(\d+\.\d+\.\d+)-(?:0\.)?\d{14}-([0-9a-f]{12})(\+.*)?$/.exec(v)
  return m ? `${m[1]} · ${m[2].slice(0, 7)}${m[3] ?? ''}` : v
}

/** Every service in the account. */
export async function discoverServices(nc: NatsConnection): Promise<ServiceInstance[]> {
  const [infos, stats] = await Promise.all([collect(nc, '$SRV.INFO'), collect(nc, '$SRV.STATS')])
  return mergeServiceReplies(infos, stats)
}

/** The instances of one service, asked of that service alone. The name comes
    from the URL, so anything a micro service cannot be called (a dot, a
    wildcard) is answered with nothing rather than put into a subject. */
export async function discoverService(nc: NatsConnection, name: string): Promise<ServiceInstance[]> {
  if (!/^[A-Za-z0-9_-]+$/.test(name)) return []
  const [infos, stats] = await Promise.all([collect(nc, `$SRV.INFO.${name}`), collect(nc, `$SRV.STATS.${name}`)])
  return mergeServiceReplies(infos, stats)
}
