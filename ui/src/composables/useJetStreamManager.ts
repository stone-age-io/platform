// ui/src/composables/useJetStreamManager.ts
import { jetstreamManager } from '@nats-io/jetstream'
import type { StreamInfo, ConsumerInfo, JetStreamAccountStats, PurgeOpts, PurgeResponse } from '@nats-io/jetstream'
import { Kvm, type KV, type KvStatus } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import type {
  StreamSummary,
  StreamFormData,
  ConsumerSummary,
  KvBucketSummary,
  KvBucketFormData,
  JetStreamAccountSummary,
} from '@/types/jetstream'

// --- Duration helpers ---

const NANOS_PER_MS = 1_000_000
const NANOS_PER_SEC = 1_000_000_000
const NANOS_PER_MIN = 60 * NANOS_PER_SEC
const NANOS_PER_HOUR = 60 * NANOS_PER_MIN
const NANOS_PER_DAY = 24 * NANOS_PER_HOUR

/**
 * Parse human-readable duration to nanoseconds.
 * Supports: "30s", "5m", "2h", "7d", "1h30m", or plain number (treated as nanos).
 */
export function parseHumanDuration(str: string): number {
  if (!str || str === '0') return 0
  const trimmed = str.trim()

  // Plain number → treat as nanoseconds
  if (/^\d+$/.test(trimmed)) return parseInt(trimmed, 10)

  let total = 0
  const regex = /(\d+(?:\.\d+)?)\s*(d|h|m|s|ms|us|ns)/gi
  let match
  while ((match = regex.exec(trimmed)) !== null) {
    const val = parseFloat(match[1])
    switch (match[2].toLowerCase()) {
      case 'd': total += val * NANOS_PER_DAY; break
      case 'h': total += val * NANOS_PER_HOUR; break
      case 'm': total += val * NANOS_PER_MIN; break
      case 's': total += val * NANOS_PER_SEC; break
      case 'ms': total += val * NANOS_PER_MS; break
      case 'us': total += val * 1_000; break
      case 'ns': total += val; break
    }
  }
  return Math.round(total)
}

/**
 * Format nanoseconds to human-readable duration.
 */
export function formatNanos(nanos: number): string {
  if (!nanos || nanos === 0) return '0'
  if (nanos < 0) return 'unlimited'

  const parts: string[] = []
  let remaining = nanos

  if (remaining >= NANOS_PER_DAY) {
    const days = Math.floor(remaining / NANOS_PER_DAY)
    parts.push(`${days}d`)
    remaining %= NANOS_PER_DAY
  }
  if (remaining >= NANOS_PER_HOUR) {
    const hours = Math.floor(remaining / NANOS_PER_HOUR)
    parts.push(`${hours}h`)
    remaining %= NANOS_PER_HOUR
  }
  if (remaining >= NANOS_PER_MIN) {
    const mins = Math.floor(remaining / NANOS_PER_MIN)
    parts.push(`${mins}m`)
    remaining %= NANOS_PER_MIN
  }
  if (remaining >= NANOS_PER_SEC) {
    const secs = Math.floor(remaining / NANOS_PER_SEC)
    parts.push(`${secs}s`)
    remaining %= NANOS_PER_SEC
  }
  if (remaining > 0 && parts.length === 0) {
    if (remaining >= NANOS_PER_MS) {
      parts.push(`${Math.floor(remaining / NANOS_PER_MS)}ms`)
    } else {
      parts.push(`${remaining}ns`)
    }
  }

  return parts.join(' ') || '0'
}

/**
 * Format nanoseconds to human form suitable for re-editing in forms.
 * Returns the most compact single-unit representation.
 */
export function nanosToFormValue(nanos: number): string {
  if (!nanos || nanos === 0) return '0'
  if (nanos % NANOS_PER_DAY === 0) return `${nanos / NANOS_PER_DAY}d`
  if (nanos % NANOS_PER_HOUR === 0) return `${nanos / NANOS_PER_HOUR}h`
  if (nanos % NANOS_PER_MIN === 0) return `${nanos / NANOS_PER_MIN}m`
  if (nanos % NANOS_PER_SEC === 0) return `${nanos / NANOS_PER_SEC}s`
  return formatNanos(nanos)
}

/**
 * Format millis to human form suitable for forms.
 */
export function millisToFormValue(ms: number): string {
  if (!ms || ms === 0) return '0'
  return nanosToFormValue(ms * NANOS_PER_MS)
}

// --- Composable ---

export function useJetStreamManager() {
  const natsStore = useNatsStore()
  const toast = useToast()

  function requireConnection() {
    if (!natsStore.nc) throw new Error('Not connected to NATS')
    return natsStore.nc
  }

  async function getJsm() {
    return await jetstreamManager(requireConnection())
  }

  function getKvm() {
    return new Kvm(requireConnection())
  }

  // --- Account Info ---

  async function getAccountInfo(): Promise<JetStreamAccountSummary> {
    try {
      const jsm = await getJsm()
      const info: JetStreamAccountStats = await jsm.getAccountInfo()
      return {
        memoryUsed: info.memory,
        memoryLimit: info.limits.max_memory,
        storageUsed: info.storage,
        storageLimit: info.limits.max_storage,
        streamCount: info.streams,
        streamLimit: info.limits.max_streams,
        consumerCount: info.consumers,
        consumerLimit: info.limits.max_consumers,
      }
    } catch (e: any) {
      toast.error(`Failed to get account info: ${e.message}`)
      throw e
    }
  }

  // --- Streams ---

  async function listStreams(): Promise<StreamSummary[]> {
    try {
      const jsm = await getJsm()
      const lister = jsm.streams.list()
      const streams: StreamSummary[] = []
      for await (const si of lister) {
        streams.push(mapStreamInfo(si))
      }
      return streams
    } catch (e: any) {
      toast.error(`Failed to list streams: ${e.message}`)
      throw e
    }
  }

  async function getStreamInfo(name: string): Promise<StreamInfo> {
    try {
      const jsm = await getJsm()
      return await jsm.streams.info(name)
    } catch (e: any) {
      toast.error(`Failed to get stream info: ${e.message}`)
      throw e
    }
  }

  async function createStream(formData: StreamFormData): Promise<StreamInfo> {
    try {
      const jsm = await getJsm()
      const config = formDataToStreamConfig(formData)
      const info = await jsm.streams.add(config)
      toast.success(`Stream "${formData.name}" created`)
      return info
    } catch (e: any) {
      toast.error(`Failed to create stream: ${e.message}`)
      throw e
    }
  }

  async function updateStream(name: string, formData: StreamFormData): Promise<StreamInfo> {
    try {
      const jsm = await getJsm()
      const config = formDataToStreamUpdateConfig(formData)
      const info = await jsm.streams.update(name, config)
      toast.success(`Stream "${name}" updated`)
      return info
    } catch (e: any) {
      toast.error(`Failed to update stream: ${e.message}`)
      throw e
    }
  }

  async function deleteStream(name: string): Promise<boolean> {
    try {
      const jsm = await getJsm()
      const result = await jsm.streams.delete(name)
      toast.success(`Stream "${name}" deleted`)
      return result
    } catch (e: any) {
      toast.error(`Failed to delete stream: ${e.message}`)
      throw e
    }
  }

  async function purgeStream(name: string, opts?: PurgeOpts): Promise<PurgeResponse> {
    try {
      const jsm = await getJsm()
      const result = await jsm.streams.purge(name, opts)
      toast.success(`Stream "${name}" purged (${result.purged} messages removed)`)
      return result
    } catch (e: any) {
      toast.error(`Failed to purge stream: ${e.message}`)
      throw e
    }
  }

  // --- Consumers ---

  async function listConsumers(stream: string): Promise<ConsumerSummary[]> {
    try {
      const jsm = await getJsm()
      const lister = jsm.consumers.list(stream)
      const consumers: ConsumerSummary[] = []
      for await (const ci of lister) {
        consumers.push(mapConsumerInfo(ci))
      }
      return consumers
    } catch (e: any) {
      toast.error(`Failed to list consumers: ${e.message}`)
      throw e
    }
  }

  async function deleteConsumer(stream: string, consumer: string): Promise<boolean> {
    try {
      const jsm = await getJsm()
      const result = await jsm.consumers.delete(stream, consumer)
      toast.success(`Consumer "${consumer}" deleted`)
      return result
    } catch (e: any) {
      toast.error(`Failed to delete consumer: ${e.message}`)
      throw e
    }
  }

  async function pauseConsumer(stream: string, consumer: string, until?: Date): Promise<void> {
    try {
      const jsm = await getJsm()
      const pauseUntil = until || new Date(Date.now() + 365 * 24 * 60 * 60 * 1000) // 1 year default
      await jsm.consumers.pause(stream, consumer, pauseUntil)
      toast.success(`Consumer "${consumer}" paused`)
    } catch (e: any) {
      toast.error(`Failed to pause consumer: ${e.message}`)
      throw e
    }
  }

  async function resumeConsumer(stream: string, consumer: string): Promise<void> {
    try {
      const jsm = await getJsm()
      await jsm.consumers.resume(stream, consumer)
      toast.success(`Consumer "${consumer}" resumed`)
    } catch (e: any) {
      toast.error(`Failed to resume consumer: ${e.message}`)
      throw e
    }
  }

  // --- KV Buckets ---

  async function listKvBuckets(): Promise<KvBucketSummary[]> {
    try {
      const jsm = await getJsm()
      const kvm = getKvm()
      const lister = jsm.streams.list()
      const buckets: KvBucketSummary[] = []

      for await (const si of lister) {
        // KV buckets are stored as streams with "KV_" prefix
        if (!si.config.name.startsWith('KV_')) continue
        const bucketName = si.config.name.substring(3) // remove "KV_"
        try {
          const kv = await kvm.open(bucketName)
          const status = await kv.status()
          buckets.push(mapKvStatus(status))
        } catch {
          // If we can't open the bucket, build summary from stream info
          buckets.push({
            id: bucketName,
            name: bucketName,
            description: si.config.description || '',
            storage: si.config.storage,
            history: si.config.max_msgs_per_subject,
            maxBytes: si.config.max_bytes,
            ttl: si.config.max_age,
            replicas: si.config.num_replicas,
            values: si.state.messages,
            bytes: si.state.bytes,
            created: si.created,
          })
        }
      }
      return buckets
    } catch (e: any) {
      toast.error(`Failed to list KV buckets: ${e.message}`)
      throw e
    }
  }

  async function getKvBucketStatus(name: string): Promise<KvStatus> {
    try {
      const kvm = getKvm()
      const kv = await kvm.open(name)
      return await kv.status()
    } catch (e: any) {
      toast.error(`Failed to get bucket status: ${e.message}`)
      throw e
    }
  }

  async function createKvBucket(formData: KvBucketFormData): Promise<KV> {
    try {
      const kvm = getKvm()
      const opts: any = {
        description: formData.description || undefined,
        history: parseInt(formData.history) || 1,
        storage: formData.storage,
        replicas: parseInt(formData.replicas) || 1,
      }

      const maxBytes = parseInt(formData.maxBytes)
      if (!isNaN(maxBytes) && maxBytes !== 0) opts.max_bytes = maxBytes

      const maxValueSize = parseInt(formData.maxValueSize)
      if (!isNaN(maxValueSize) && maxValueSize !== 0) opts.maxValueSize = maxValueSize

      const ttlMs = parseHumanDuration(formData.ttl) / NANOS_PER_MS
      if (ttlMs > 0) opts.ttl = ttlMs

      const kv = await kvm.create(formData.name, opts)
      toast.success(`KV bucket "${formData.name}" created`)
      return kv
    } catch (e: any) {
      toast.error(`Failed to create KV bucket: ${e.message}`)
      throw e
    }
  }

  async function deleteKvBucket(name: string): Promise<boolean> {
    try {
      const kvm = getKvm()
      const kv = await kvm.open(name)
      const result = await kv.destroy()
      toast.success(`KV bucket "${name}" deleted`)
      return result
    } catch (e: any) {
      toast.error(`Failed to delete KV bucket: ${e.message}`)
      throw e
    }
  }

  // --- Mapping helpers ---

  function mapStreamInfo(si: StreamInfo): StreamSummary {
    return {
      id: si.config.name,
      name: si.config.name,
      description: si.config.description || '',
      subjects: si.config.subjects || [],
      retention: si.config.retention,
      storage: si.config.storage,
      messages: si.state.messages,
      bytes: si.state.bytes,
      consumers: si.state.consumer_count,
      created: si.created,
    }
  }

  function mapConsumerInfo(ci: ConsumerInfo): ConsumerSummary {
    return {
      id: ci.name,
      name: ci.name,
      durableName: ci.config.durable_name || '',
      description: ci.config.description || '',
      deliverPolicy: ci.config.deliver_policy,
      ackPolicy: ci.config.ack_policy,
      filterSubjects: ci.config.filter_subjects || (ci.config.filter_subject ? [ci.config.filter_subject] : []),
      numPending: ci.num_pending,
      numAckPending: ci.num_ack_pending,
      numRedelivered: ci.num_redelivered,
      created: ci.created,
      paused: ci.paused || false,
    }
  }

  function mapKvStatus(status: KvStatus): KvBucketSummary {
    return {
      id: status.bucket,
      name: status.bucket,
      description: status.description || '',
      storage: status.storage,
      history: status.history,
      maxBytes: status.max_bytes,
      ttl: status.ttl,
      replicas: status.replicas,
      values: status.values,
      bytes: status.size,
      created: status.streamInfo.created,
    }
  }

  function parseSubjects(str: string): string[] {
    return str.split(/[,\n]/)
      .map(s => s.trim())
      .filter(s => s !== '')
  }

  function formDataToStreamConfig(fd: StreamFormData) {
    return {
      name: fd.name,
      description: fd.description || undefined,
      subjects: parseSubjects(fd.subjects),
      retention: fd.retention as any,
      storage: fd.storage as any,
      max_msgs: parseInt(fd.maxMsgs) || -1,
      max_bytes: parseInt(fd.maxBytes) || -1,
      max_msg_size: parseInt(fd.maxMsgSize) || -1,
      max_age: parseHumanDuration(fd.maxAge),
      max_consumers: parseInt(fd.maxConsumers) || -1,
      max_msgs_per_subject: parseInt(fd.maxMsgsPerSubject) || -1,
      num_replicas: parseInt(fd.numReplicas) || 1,
      discard: fd.discard as any,
      duplicate_window: parseHumanDuration(fd.duplicateWindow),
      allow_direct: fd.allowDirect,
    }
  }

  function formDataToStreamUpdateConfig(fd: StreamFormData) {
    return {
      description: fd.description || undefined,
      subjects: parseSubjects(fd.subjects),
      max_msgs: parseInt(fd.maxMsgs) || -1,
      max_bytes: parseInt(fd.maxBytes) || -1,
      max_msg_size: parseInt(fd.maxMsgSize) || -1,
      max_age: parseHumanDuration(fd.maxAge),
      max_consumers: parseInt(fd.maxConsumers) || -1,
      max_msgs_per_subject: parseInt(fd.maxMsgsPerSubject) || -1,
      num_replicas: parseInt(fd.numReplicas) || 1,
      discard: fd.discard as any,
      duplicate_window: parseHumanDuration(fd.duplicateWindow),
      allow_direct: fd.allowDirect,
    }
  }

  return {
    // Account
    getAccountInfo,
    // Streams
    listStreams,
    getStreamInfo,
    createStream,
    updateStream,
    deleteStream,
    purgeStream,
    // Consumers
    listConsumers,
    deleteConsumer,
    pauseConsumer,
    resumeConsumer,
    // KV Buckets
    listKvBuckets,
    getKvBucketStatus,
    createKvBucket,
    deleteKvBucket,
    // Helpers
    parseHumanDuration,
    formatNanos,
    nanosToFormValue,
    millisToFormValue,
  }
}
