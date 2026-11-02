// ui/src/types/services.ts

/**
 * One endpoint of a NATS micro service: its `$SRV.INFO` description, with its
 * numbers from `$SRV.STATS` when that reply arrived. The numbers are null
 * rather than 0 when it did not: "no reply" and "no requests" are different
 * answers.
 */
export interface ServiceEndpoint {
  name: string
  subject: string
  queueGroup: string
  requests: number | null
  errors: number | null
  lastError: string
  averageMs: number | null
}

/** One running instance of a NATS micro service. */
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

/** Every instance answering under one service name. */
export interface ServiceGroup {
  id: string              // the name, for ResponsiveList
  name: string
  description: string
  instances: ServiceInstance[]
  versions: string[]      // distinct, most-run first
  requests: number | null // null only when no instance's STATS arrived
  errors: number | null
}
