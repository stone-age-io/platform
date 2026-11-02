// ui/src/types/services.ts

/** One endpoint of a NATS micro service, as `$SRV.INFO` describes it. */
export interface ServiceEndpoint {
  name: string
  subject: string
}

/**
 * One running instance of a NATS micro service: its `$SRV.INFO` reply, with the
 * numbers from its `$SRV.STATS` reply folded in when that arrived too. The
 * stats are null rather than 0 when it did not: "no reply" and "no requests"
 * are different answers.
 */
export interface ServiceInstance {
  id: string
  name: string
  version: string
  description: string
  metadata: Record<string, string>
  endpoints: ServiceEndpoint[]
  started: string | null  // ISO date, from STATS
  requests: number | null // summed over the endpoints, from STATS
  errors: number | null
  lastError: string
}
