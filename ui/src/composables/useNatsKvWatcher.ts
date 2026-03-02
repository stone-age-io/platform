// ui/src/composables/useNatsKvWatcher.ts
//
// Generalized KV prefix watcher — extracted from the patterns in useOccupancy.ts.
// Watches a NATS KV bucket for keys matching a pattern and maintains a reactive Map of rows.

import { ref, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { Kvm } from '@nats-io/kv'
import { decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'

export interface KvRow {
  key: string
  keySuffix: string
  revision: number
  timestamp: Date
  data: Record<string, any>
}

export function useNatsKvWatcher(
  bucketName: Ref<string> | (() => string),
  keyPattern: Ref<string> | (() => string)
) {
  const natsStore = useNatsStore()
  const dashboardStore = useDashboardStore()

  const rows = ref<Map<string, KvRow>>(new Map())
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Internal buffer for RAF batching
  const buffer = new Map<string, KvRow>()
  const deletedKeys = new Set<string>()
  let flushHandle: number | null = null
  let kvWatcher: any = null
  let initialLoad = false
  let loadingTimeout: ReturnType<typeof setTimeout> | null = null

  function getBucket(): string {
    const raw = typeof bucketName === 'function' ? bucketName() : bucketName.value
    return resolveTemplate(raw, dashboardStore.currentVariableValues)
  }

  function getPattern(): string {
    const raw = typeof keyPattern === 'function' ? keyPattern() : keyPattern.value
    return resolveTemplate(raw, dashboardStore.currentVariableValues)
  }

  // Derive the key prefix from the pattern (strip trailing .> or >)
  function getPrefix(pattern: string): string {
    if (pattern.endsWith('.>')) return pattern.slice(0, -2)
    if (pattern === '>') return ''
    return pattern
  }

  function flushBuffer() {
    // Apply deletes
    for (const key of deletedKeys) {
      buffer.delete(key)
    }
    deletedKeys.clear()

    // Replace the entire Map to trigger Vue reactivity
    rows.value = new Map(buffer)
    flushHandle = null
  }

  function scheduleUpdate() {
    if (!flushHandle) {
      flushHandle = requestAnimationFrame(flushBuffer)
    }
  }

  async function startWatching() {
    const bucket = getBucket()
    const pattern = getPattern()
    if (!bucket || !pattern || !natsStore.isConnected || !natsStore.nc) return

    loading.value = true
    initialLoad = true
    error.value = null
    buffer.clear()
    deletedKeys.clear()
    rows.value = new Map()

    const prefix = getPrefix(pattern)

    try {
      const kvm = new Kvm(natsStore.nc)

      let kv
      try {
        kv = await kvm.open(bucket)
      } catch (e: any) {
        // Bucket might not exist yet
        loading.value = false
        if (e.message?.includes('stream not found')) {
          error.value = `Bucket "${bucket}" not found`
        }
        return
      }

      const iter = await kv.watch({ key: pattern })
      kvWatcher = iter

      // Fallback: if no keys match, the iterator blocks forever
      loadingTimeout = setTimeout(() => {
        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      }, 500)

      ;(async () => {
        for await (const e of iter) {
          // Clear loading after initial values are delivered
          if (initialLoad && e.delta === 0) {
            initialLoad = false
            loading.value = false
            if (loadingTimeout) { clearTimeout(loadingTimeout); loadingTimeout = null }
          }

          // Extract key suffix by stripping the watched prefix
          const keySuffix = prefix && e.key.startsWith(prefix + '.')
            ? e.key.slice(prefix.length + 1)
            : e.key

          if (e.operation === 'DEL' || e.operation === 'PURGE') {
            deletedKeys.add(e.key)
            buffer.delete(e.key)
          } else {
            deletedKeys.delete(e.key)
            try {
              const raw = decodeBytes(e.value!)
              let data: Record<string, any>
              try {
                data = JSON.parse(raw)
              } catch {
                // Non-JSON value — wrap in a simple object
                data = { __value__: raw }
              }

              buffer.set(e.key, {
                key: e.key,
                keySuffix,
                revision: e.revision,
                timestamp: e.created,
                data
              })
            } catch (err) {
              // Ignore malformed entries
            }
          }
          scheduleUpdate()
        }

        // Iterator ended without hitting delta === 0 (empty bucket)
        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      })()
    } catch (err: any) {
      console.error('[KvWatcher] Watch error:', err)
      error.value = err.message
      loading.value = false
    }
  }

  function stopWatching() {
    if (kvWatcher) {
      try { kvWatcher.stop() } catch {}
      kvWatcher = null
    }
    if (flushHandle) {
      cancelAnimationFrame(flushHandle)
      flushHandle = null
    }
    if (loadingTimeout) {
      clearTimeout(loadingTimeout)
      loadingTimeout = null
    }
    initialLoad = false
    buffer.clear()
    deletedKeys.clear()
    rows.value = new Map()
  }

  onMounted(() => {
    if (natsStore.isConnected) startWatching()
  })

  onUnmounted(() => {
    stopWatching()
  })

  // Restart when connection, bucket, or pattern changes
  watch(
    [
      () => natsStore.isConnected,
      () => getBucket(),
      () => getPattern()
    ],
    ([conn], [_oldConn]) => {
      stopWatching()
      if (conn && getBucket() && getPattern()) startWatching()
    }
  )

  return { rows, loading, error }
}
