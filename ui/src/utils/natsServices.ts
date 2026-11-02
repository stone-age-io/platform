// ui/src/utils/natsServices.ts
//
// NATS micro service discovery (ADR-32): every service subscribes to
// `$SRV.INFO` and `$SRV.STATS`, so one request to each, collecting every reply
// that arrives, lists what is running in this account right now. Nothing is
// stored. The edge agent registers as `stone-agent`, which is what makes this
// a list of live sites as well as of services.

import { NoRespondersError, PermissionViolationError } from '@nats-io/nats-core'
import type { NatsConnection } from '@nats-io/nats-core'
import type { ServiceInstance } from '@/types/services'

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

/**
 * Join INFO and STATS replies on instance id. INFO is the list; a STATS reply
 * with no INFO beside it is dropped rather than shown half-empty, since the two
 * travel the same path and missing one is the stall timer cutting it short.
 */
export function mergeServiceReplies(infos: any[], stats: any[]): ServiceInstance[] {
  const statsById = new Map(stats.filter(s => s?.id).map(s => [s.id, s]))
  return infos
    .filter(i => i?.id && i?.name)
    .map(i => {
      const s = statsById.get(i.id)
      const eps: any[] = Array.isArray(s?.endpoints) ? s.endpoints : []
      return {
        id: i.id,
        name: i.name,
        version: i.version ?? '',
        description: i.description ?? '',
        metadata: i.metadata ?? {},
        endpoints: (Array.isArray(i.endpoints) ? i.endpoints : []).map((e: any) => ({ name: e.name ?? '', subject: e.subject ?? '' })),
        started: s?.started ?? null,
        requests: s ? eps.reduce((n, e) => n + (e.num_requests ?? 0), 0) : null,
        errors: s ? eps.reduce((n, e) => n + (e.num_errors ?? 0), 0) : null,
        lastError: eps.map(e => e.last_error).find(Boolean) ?? '',
      }
    })
}

export async function discoverServices(nc: NatsConnection): Promise<ServiceInstance[]> {
  const [infos, stats] = await Promise.all([collect(nc, '$SRV.INFO'), collect(nc, '$SRV.STATS')])
  return mergeServiceReplies(infos, stats)
}
