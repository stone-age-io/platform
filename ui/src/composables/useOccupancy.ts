import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { Kvm } from '@nats-io/kv'
import { decodeBytes } from '@/utils/encoding'

export interface Occupant {
  user_id: string
  timestamp: string
  sortKey: number // cached ms for sort performance
  lat?: number
  lon?: number
  rawData: Record<string, any>
}

export function useOccupancy(locationCode: () => string | undefined) {
  const natsStore = useNatsStore()

  const occupants = ref<Occupant[]>([])
  const loading = ref(false)

  // Internal buffer
  const occupantBuffer = new Map<string, Occupant>()

  let kvWatcher: any = null
  let flushHandle: number | null = null
  let initialLoad = false
  let loadingTimeout: ReturnType<typeof setTimeout> | null = null

  const flushBuffer = () => {
    occupants.value = Array.from(occupantBuffer.values())
      .sort((a, b) => b.sortKey - a.sortKey)

    flushHandle = null
  }

  const scheduleUpdate = () => {
    if (!flushHandle) {
      flushHandle = requestAnimationFrame(flushBuffer)
    }
  }

  // Helper: Normalize incoming timestamp to ms number + ISO string
  function normalizeTimestamp(val: any): { iso: string; ms: number } {
    if (!val) {
      const now = Date.now()
      return { iso: new Date(now).toISOString(), ms: now }
    }

    // If it's a number (OwnTracks sends Unix Seconds: 1772232560)
    if (typeof val === 'number') {
      // Check if it's seconds (small) or milliseconds (big)
      // 10000000000 is roughly the year 2286. If smaller, it's seconds.
      const ms = val < 10000000000 ? val * 1000 : val
      return { iso: new Date(ms).toISOString(), ms }
    }

    // If it's already a string (legacy data), parse it
    const ms = new Date(String(val)).getTime() || Date.now()
    return { iso: String(val), ms }
  }

  async function startWatching() {
    const code = locationCode()
    if (!code || !natsStore.isConnected || !natsStore.nc) return

    loading.value = true
    initialLoad = true
    occupantBuffer.clear()
    occupants.value = []

    try {
      const kvm = new Kvm(natsStore.nc)

      let kv
      try {
        kv = await kvm.open('occupancy')
      } catch (e) {
        // Bucket might not exist yet
        loading.value = false
        return
      }

      // Watch keys matching the location code prefix.
      // Using .> to match one or more tokens after the code,
      // guarding against user IDs that might contain dots.
      const iter = await kv.watch({ key: `${code}.>` })
      kvWatcher = iter

      // Fallback: if no keys match this prefix the iterator blocks
      // forever — clear loading after a short grace period.
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

          const codePrefix = code + '.'
          const userId = e.key.startsWith(codePrefix)
            ? e.key.slice(codePrefix.length)
            : e.key.split('.').pop()!

          if (e.operation === 'DEL' || e.operation === 'PURGE') {
            occupantBuffer.delete(userId)
          } else {
            try {
              const data = JSON.parse(decodeBytes(e.value!))
              const ts = normalizeTimestamp(data.timestamp)

              occupantBuffer.set(userId, {
                user_id: userId,
                timestamp: ts.iso,
                sortKey: ts.ms,
                lat: data.lat,
                lon: data.lon,
                rawData: data
              })
            } catch (err) {
              // Ignore malformed
            }
          }
          scheduleUpdate()
        }

        // If the iterator ends without hitting delta === 0
        // (empty bucket), clear loading
        if (initialLoad) {
          initialLoad = false
          loading.value = false
        }
      })()
    } catch (err) {
      console.error('[Occupancy] Watch error:', err)
      loading.value = false
    }
  }

  function stopWatching() {
    if (kvWatcher) {
      try {
        kvWatcher.stop()
      } catch (err) {
        // Ignore errors if already stopped
      }
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
    occupantBuffer.clear()
    occupants.value = []
  }

  onMounted(() => {
    if (natsStore.isConnected) startWatching()
  })

  onUnmounted(() => {
    stopWatching()
  })

  watch([() => natsStore.isConnected, locationCode], ([conn, loc], [oldConn, oldLoc]) => {
    if (conn !== oldConn || loc !== oldLoc) {
      stopWatching()
      if (conn && loc) startWatching()
    }
  })

  return { occupants, loading }
}
