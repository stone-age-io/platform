// Watches a NATS KV bucket for keys matching a pattern and keeps a reactive Map of rows.

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

  // Batched into rows once per animation frame.
  const buffer = new Map<string, KvRow>()
  const deletedKeys = new Set<string>()
  let flushHandle: number | null = null
  let kvWatcher: any = null
  let initialLoad = false
  let loadingTimeout: ReturnType<typeof setTimeout> | null = null
  let startGeneration = 0

  function getBucket(): string {
    const raw = typeof bucketName === 'function' ? bucketName() : bucketName.value
    return resolveTemplate(raw, dashboardStore.currentVariableValues)
  }

  function getPattern(): string {
    const raw = typeof keyPattern === 'function' ? keyPattern() : keyPattern.value
    return resolveTemplate(raw, dashboardStore.currentVariableValues)
  }

  function getPrefix(pattern: string): string {
    if (pattern.endsWith('.>')) return pattern.slice(0, -2)
    if (pattern === '>') return ''
    return pattern
  }

  function flushBuffer() {
    for (const key of deletedKeys) {
      buffer.delete(key)
    }
    deletedKeys.clear()

    // A new Map, so Vue sees the change.
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

    // A start or stop during the awaits below makes this generation stale.
    const thisGeneration = ++startGeneration

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
        if (thisGeneration !== startGeneration) return
        loading.value = false
        error.value = e.message?.includes('stream not found')
          ? `Bucket "${bucket}" not found`
          : e.message || `Could not open bucket "${bucket}"`
        return
      }

      if (thisGeneration !== startGeneration) return

      const iter = await kv.watch({ key: pattern })

      if (thisGeneration !== startGeneration) {
        try { iter.stop() } catch {}
        return
      }

      kvWatcher = iter

      // With no matching keys the iterator never delivers delta 0, so time out the spinner.
      loadingTimeout = setTimeout(() => {
        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      }, 500)

      ;(async () => {
        for await (const e of iter) {
          if (thisGeneration !== startGeneration) break

          // delta 0 marks the last of the initial values.
          if (initialLoad && e.delta === 0) {
            initialLoad = false
            loading.value = false
            if (loadingTimeout) { clearTimeout(loadingTimeout); loadingTimeout = null }
          }

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
              // Skip a malformed entry.
            }
          }
          scheduleUpdate()
        }

        // Ended without a delta 0 (empty bucket).
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
    startGeneration++ // invalidates an in-flight startWatching
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

  function refresh() {
    stopWatching()
    if (natsStore.isConnected) startWatching()
  }

  onMounted(() => {
    if (natsStore.isConnected) startWatching()
    window.addEventListener('dashboard:refresh', refresh)
  })

  onUnmounted(() => {
    stopWatching()
    window.removeEventListener('dashboard:refresh', refresh)
  })

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

  return { rows, loading, error, refresh }
}
