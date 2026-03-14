// ui/src/types/jetstream.ts

/** Stream summary for list view */
export interface StreamSummary {
  id: string            // same as name, needed for ResponsiveList
  name: string
  description: string
  subjects: string[]
  retention: string     // 'limits' | 'interest' | 'workqueue'
  storage: string       // 'file' | 'memory'
  messages: number
  bytes: number
  consumers: number
  created: string       // ISO date
}

/** Stream create/edit form data */
export interface StreamFormData {
  name: string
  description: string
  subjects: string           // comma/newline separated, converted to array on submit
  retention: 'limits' | 'interest' | 'workqueue'
  storage: 'file' | 'memory'
  maxMsgs: string
  maxBytes: string
  maxMsgSize: string
  maxAge: string             // human-readable: "24h", "7d"
  maxConsumers: string
  maxMsgsPerSubject: string
  numReplicas: string
  discard: 'old' | 'new'
  duplicateWindow: string    // human-readable: "2m"
  allowDirect: boolean
}

/** Consumer summary for stream detail sub-table */
export interface ConsumerSummary {
  id: string             // same as name, for keying
  name: string
  durableName: string
  description: string
  deliverPolicy: string
  ackPolicy: string
  filterSubjects: string[]
  numPending: number
  numAckPending: number
  numRedelivered: number
  created: string
  paused: boolean
}

/** KV bucket summary for list view */
export interface KvBucketSummary {
  id: string             // same as bucket name
  name: string
  description: string
  storage: string
  history: number
  maxBytes: number
  ttl: number            // nanoseconds
  replicas: number
  values: number         // key count
  bytes: number
  created: string
}

/** KV bucket create form data */
export interface KvBucketFormData {
  name: string
  description: string
  storage: 'file' | 'memory'
  history: string
  maxBytes: string
  maxValueSize: string
  ttl: string            // human-readable: "24h", "7d", "0" = no TTL
  replicas: string
}

/** Account-level JetStream stats */
export interface JetStreamAccountSummary {
  memoryUsed: number
  memoryLimit: number    // -1 = unlimited
  storageUsed: number
  storageLimit: number   // -1 = unlimited
  streamCount: number
  streamLimit: number
  consumerCount: number
  consumerLimit: number
}
