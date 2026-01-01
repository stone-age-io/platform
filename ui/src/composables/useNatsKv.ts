import { ref, onUnmounted } from 'vue'
import { Kvm, type KV } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'

export interface KvEntry {
  key: string
  value: any
  revision: number
  created: Date
  operation: 'PUT' | 'DEL' | 'PURGE'
}

/**
 * useNatsKv handles interactions with a NATS Key-Value bucket.
 * @param bucketName Defaults to 'twin' for the global organization state.
 * @param baseKey Optional prefix (e.g., 'thing.S01'). If provided, the watcher 
 *                and writers will be scoped to this prefix.
 */
export function useNatsKv(bucketName: string = 'twin', baseKey?: string) {
  const natsStore = useNatsStore()
  const toast = useToast()
  
  const encoder = new TextEncoder()
  const decoder = new TextDecoder()

  // State
  const kv = ref<KV | null>(null)
  const entries = ref<Map<string, KvEntry>>(new Map())
  const loading = ref(false)
  const exists = ref(false)
  const error = ref<string | null>(null)
  
  let watcher: any = null

  async function init() {
    if (!natsStore.nc) return
    
    if (watcher) {
      try { watcher.stop() } catch (e) { console.error('Watcher stop error:', e) }
      watcher = null
    }
    
    loading.value = true
    entries.value.clear()
    error.value = null

    try {
      const kvm = new Kvm(natsStore.nc)
      
      // Check if bucket exists
      let found = false
      for await (const b of await kvm.list()) { 
        if (b.bucket === bucketName) {
          found = true
          break
        }
      }
      
      if (!found) {
        exists.value = false
        loading.value = false
        return
      }

      kv.value = await kvm.open(bucketName)
      exists.value = true
      await startWatch()
    } catch (e: any) {
      console.error('KV Init Error:', e)
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function startWatch() {
    if (!kv.value) return
    
    try {
      // If baseKey is provided, watch "baseKey.>"
      // Otherwise watch the whole bucket ">"
      const filter = baseKey ? `${baseKey}.>` : '>'
      const iter = await kv.value.watch({ key: filter })
      watcher = iter
      
      ;(async () => {
        for await (const e of iter) {
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
        }
      })()
    } catch (e) {
      console.error('KV Watch Error', e)
    }
  }

  /**
   * Puts a value into KV.
   * If baseKey is 'thing.S01' and propName is 'temp', key will be 'thing.S01.temp'
   */
  async function put(propName: string, value: any) {
    if (!kv.value) return
    try {
      // Prevent double prefixing if the full key was passed
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
      const kvm = new Kvm(natsStore.nc)
      await kvm.create(bucketName, {
        description: description || 'Digital Twin Store',
        history: 10,
        storage: 'file'
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
    if (watcher) watcher.stop()
  })

  return { entries, loading, exists, error, init, createBucket, put, del, getHistory }
}
