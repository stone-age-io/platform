import { ref, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { Kvm } from '@nats-io/kv'

/**
 * Watches the entire `occupancy` KV bucket and provides
 * live per-location occupant counts.
 *
 * @param knownCodes Reactive list of location codes from PocketBase.
 *   Used to correctly parse keys like "building.floor1.brian-smith"
 *   where "building.floor1" is the location code and "brian-smith"
 *   is the user ID. Codes are matched longest-first so hierarchical
 *   codes resolve correctly.
 */
export function useOccupancySummary(knownCodes: Ref<string[]>) {
  const natsStore = useNatsStore()

  const counts = ref<Map<string, number>>(new Map())
  const loading = ref(false)

  // Internal: location code -> set of user IDs
  const occupantsByLocation = new Map<string, Set<string>>()

  let kvWatcher: any = null
  let flushHandle: number | null = null
  let initialLoad = false
  let loadingTimeout: ReturnType<typeof setTimeout> | null = null

  // Sorted longest-first for greedy prefix matching
  let sortedCodes: string[] = []

  function updateSortedCodes() {
    sortedCodes = [...knownCodes.value].sort((a, b) => b.length - a.length)
  }

  function extractCode(key: string): string | null {
    for (const code of sortedCodes) {
      if (key.startsWith(code + '.')) return code
    }
    return null
  }

  const flushCounts = () => {
    const map = new Map<string, number>()
    for (const [code, users] of occupantsByLocation) {
      map.set(code, users.size)
    }
    counts.value = map
    flushHandle = null
  }

  const scheduleUpdate = () => {
    if (!flushHandle) {
      flushHandle = requestAnimationFrame(flushCounts)
    }
  }

  async function startWatching() {
    if (!natsStore.isConnected || !natsStore.nc || knownCodes.value.length === 0) return

    loading.value = true
    initialLoad = true
    occupantsByLocation.clear()
    counts.value = new Map()
    updateSortedCodes()

    try {
      const kvm = new Kvm(natsStore.nc)

      let kv
      try {
        kv = await kvm.open('occupancy')
      } catch (e) {
        loading.value = false
        return
      }

      const iter = await kv.watch({ key: '>' })
      kvWatcher = iter

      // Fallback for empty bucket
      loadingTimeout = setTimeout(() => {
        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      }, 500)

      ;(async () => {
        for await (const e of iter) {
          if (initialLoad && e.delta === 0) {
            initialLoad = false
            loading.value = false
            if (loadingTimeout) { clearTimeout(loadingTimeout); loadingTimeout = null }
          }

          const code = extractCode(e.key)
          if (!code) continue

          const userId = e.key.slice(code.length + 1)

          if (e.operation === 'DEL' || e.operation === 'PURGE') {
            const users = occupantsByLocation.get(code)
            if (users) {
              users.delete(userId)
              if (users.size === 0) occupantsByLocation.delete(code)
            }
          } else {
            if (!occupantsByLocation.has(code)) {
              occupantsByLocation.set(code, new Set())
            }
            occupantsByLocation.get(code)!.add(userId)
          }
          scheduleUpdate()
        }

        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      })()
    } catch (err) {
      console.error('[OccupancySummary] Watch error:', err)
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
    occupantsByLocation.clear()
    counts.value = new Map()
  }

  onMounted(() => {
    if (natsStore.isConnected && knownCodes.value.length > 0) startWatching()
  })

  onUnmounted(() => stopWatching())

  watch([() => natsStore.isConnected, knownCodes], () => {
    stopWatching()
    if (natsStore.isConnected && knownCodes.value.length > 0) startWatching()
  })

  return { counts, loading }
}
