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

export function useNatsKv(bucketName: string) {
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

  // Initialize
  async function init() {
    if (!natsStore.nc) return
    
    loading.value = true
    entries.value.clear()
    error.value = null
    exists.value = false

    try {
      const kvm = new Kvm(natsStore.nc)
      
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

  // Create Bucket
  async function createBucket(description?: string) {
    if (!natsStore.nc) return
    loading.value = true
    try {
      const kvm = new Kvm(natsStore.nc)
      await kvm.create(bucketName, {
        description: description || 'Digital Twin Store',
        history: 10, // Increased history for the new feature
        storage: 'file'
      })
      toast.success(`Bucket ${bucketName} created`)
      await init()
    } catch (e: any) {
      toast.error(e.message)
    } finally {
      loading.value = false
    }
  }

  // Watcher
  async function startWatch() {
    if (!kv.value) return
    try {
      const iter = await kv.value.watch()
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

  // Helper: Decode value
  function decodeValue(bytes: Uint8Array): any {
    try {
      const str = decoder.decode(bytes)
      try { return JSON.parse(str) } catch { return str }
    } catch { return '[Binary]' }
  }

  // Put
  async function put(key: string, value: any) {
    if (!kv.value) return
    try {
      const encoded = encoder.encode(JSON.stringify(value))
      await kv.value.put(key, encoded)
      toast.success('Key updated')
    } catch (e: any) {
      toast.error(`Failed to put key: ${e.message}`)
      throw e
    }
  }

  // Delete
  async function del(key: string) {
    if (!kv.value) return
    try {
      await kv.value.delete(key)
      toast.success('Key deleted')
    } catch (e: any) {
      toast.error(`Failed to delete: ${e.message}`)
    }
  }

  // Get History
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
    // Return newest first
    return history.reverse()
  }

  onUnmounted(() => {
    if (watcher) watcher.stop()
  })

  return {
    entries,
    loading,
    exists,
    error,
    init,
    createBucket,
    put,
    del,
    getHistory
  }
}
