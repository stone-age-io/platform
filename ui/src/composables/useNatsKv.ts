import { ref, onUnmounted } from 'vue'
import { Kvm, type KV } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { TWIN_BUCKET, TWIN_BUCKET_CONFIG, TWIN_BUCKET_DESCRIPTIONS } from '@/utils/twin'

export interface KvEntry {
  key: string
  value: any
  revision: number
  created: Date
  operation: 'PUT' | 'DEL' | 'PURGE'
}

/**
 * Watches and writes a NATS KV bucket, optionally scoped to a key prefix.
 * @param bucketName Defaults to the org's `twin` bucket (key convention in utils/twin.ts).
 * @param baseKey Optional prefix (e.g. 'thing.S01') for the watcher and writers.
 */
export function useNatsKv(bucketName: string = TWIN_BUCKET, baseKey?: string) {
  const natsStore = useNatsStore()
  const toast = useToast()
  
  const encoder = new TextEncoder()
  const decoder = new TextDecoder()

  const kv = ref<KV | null>(null)
  const entries = ref<Map<string, KvEntry>>(new Map())
  const loading = ref(false)
  const exists = ref(false)
  const error = ref<string | null>(null)
  
  let watcher: any = null
  let initGeneration = 0

  async function init() {
    if (!natsStore.nc) return

    stopWatcher()
    const thisGeneration = ++initGeneration

    loading.value = true
    entries.value.clear()
    error.value = null

    try {
      const kvm = new Kvm(natsStore.nc)

      try {
        kv.value = await kvm.open(bucketName)
      } catch (e: any) {
        if (e.message?.includes('stream not found')) {
          exists.value = false
          loading.value = false
          return
        }
        throw e
      }

      // Stale: another init() ran while we awaited open().
      if (thisGeneration !== initGeneration) return

      exists.value = true
      await startWatch(thisGeneration)
    } catch (e: any) {
      console.error('KV Init Error:', e)
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  function stopWatcher() {
    initGeneration++
    if (watcher) {
      try { watcher.stop() } catch {}
      watcher = null
    }
  }

  async function startWatch(generation: number) {
    if (!kv.value) return

    try {
      const filter = baseKey ? `${baseKey}.>` : '>'
      const iter = await kv.value.watch({ key: filter })

      if (generation !== initGeneration) {
        try { iter.stop() } catch {}
        return
      }

      watcher = iter

      ;(async () => {
        for await (const e of iter) {
          if (generation !== initGeneration) break
          try {
            if (e.operation === 'DEL' || e.operation === 'PURGE') {
              entries.value.delete(e.key)
            } else {
              entries.value.set(e.key, {
                key: e.key,
                value: decodeValue(e.value),
                revision: e.revision,
                created: e.created,
                operation: e.operation
              })
            }
          } catch (err) {
            console.error('KV entry processing error:', err)
          }
        }
      })()
    } catch (e) {
      console.error('KV Watch Error', e)
    }
  }

  /** Writes JSON to `baseKey.propName`, or to `propName` if it is already a full key. */
  async function put(propName: string, value: any) {
    if (!kv.value) return
    try {
      const fullKey = (baseKey && !propName.startsWith(baseKey)) 
        ? `${baseKey}.${propName}` 
        : propName

      const encoded = encoder.encode(JSON.stringify(value))
      await kv.value.put(fullKey, encoded)
    } catch (e: any) {
      toast.error(`Update failed: ${e.message}`)
      throw e
    }
  }

  async function del(key: string) {
    if (!kv.value) return
    try {
      await kv.value.delete(key)
      toast.success('Key deleted')
    } catch (e: any) {
      toast.error(`Failed to delete: ${e.message}`)
    }
  }

  async function getHistory(key: string): Promise<KvEntry[]> {
    if (!kv.value) return []
    const history: KvEntry[] = []
    try {
      const iter = await kv.value.history({ key })
      for await (const e of iter) {
        history.push({
          key: e.key,
          value: e.value ? decodeValue(e.value) : null,
          revision: e.revision,
          created: e.created,
          operation: e.operation
        })
      }
    } catch (e) {
      console.error('History fetch error', e)
    }
    return history.reverse()
  }

  async function createBucket(description?: string) {
    if (!natsStore.nc) return
    loading.value = true
    try {
      // Shared config, so this and the edge agent create the same bucket,
      // whichever gets there first. See utils/twin.ts.
      const kvm = new Kvm(natsStore.nc)
      await kvm.create(bucketName, {
        ...TWIN_BUCKET_CONFIG,
        description: description || TWIN_BUCKET_DESCRIPTIONS[bucketName] || bucketName,
      })
      toast.success(`Bucket ${bucketName} initialized`)
      await init()
    } catch (e: any) {
      toast.error(e.message)
    } finally {
      loading.value = false
    }
  }

  function decodeValue(bytes: Uint8Array): any {
    try {
      const str = decoder.decode(bytes)
      try { return JSON.parse(str) } catch { return str }
    } catch { return '[Binary]' }
  }

  onUnmounted(() => {
    stopWatcher()
  })

  return { entries, loading, exists, error, init, createBucket, put, del, getHistory }
}
