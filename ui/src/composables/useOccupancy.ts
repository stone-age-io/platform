import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { Kvm } from '@nats-io/kv'
import { decodeBytes } from '@/utils/encoding'

export interface Occupant {
  user_id: string
  timestamp: string
  lat?: number
  lon?: number
}

export function useOccupancy(locationCode: () => string | undefined) {
  const natsStore = useNatsStore()
  
  const occupants = ref<Occupant[]>([])
  const loading = ref(false)
  
  // Internal buffer
  const occupantBuffer = new Map<string, Occupant>()
  
  let kvWatcher: any = null
  let flushHandle: number | null = null

  const flushBuffer = () => {
    occupants.value = Array.from(occupantBuffer.values())
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    
    flushHandle = null
  }

  const scheduleUpdate = () => {
    if (!flushHandle) {
      flushHandle = requestAnimationFrame(flushBuffer)
    }
  }

  // Helper: Normalize incoming timestamp to ISO String
  function normalizeTimestamp(val: any): string {
    if (!val) return new Date().toISOString()
    
    // If it's a number (OwnTracks sends Unix Seconds: 1772232560)
    if (typeof val === 'number') {
      // Check if it's seconds (small) or milliseconds (big)
      // 10000000000 is roughly the year 2286. If smaller, it's seconds.
      const ms = val < 10000000000 ? val * 1000 : val
      return new Date(ms).toISOString()
    }
    
    // If it's already a string (legacy data), return as is
    return String(val)
  }

  async function startWatching() {
    const code = locationCode()
    if (!code || !natsStore.isConnected || !natsStore.nc) return

    loading.value = true
    occupantBuffer.clear()
    occupants.value = []

    try {
      const kvm = new Kvm(natsStore.nc)
      
      try {
        await kvm.open('occupancy')
      } catch (e) {
        // Bucket might not exist yet
        loading.value = false
        return
      }

      const kv = await kvm.open('occupancy')
      
      // Watch keys starting with the location code
      const iter = await kv.watch({ key: `${code}.*` })
      kvWatcher = iter

      ;(async () => {
        for await (const e of iter) {
          const parts = e.key.split('.')
          const userId = parts[parts.length - 1]

          if (e.operation === 'DEL' || e.operation === 'PURGE') {
            occupantBuffer.delete(userId)
          } else {
            try {
              const data = JSON.parse(decodeBytes(e.value!))
              
              occupantBuffer.set(userId, { 
                user_id: userId,
                timestamp: normalizeTimestamp(data.timestamp),
                lat: data.lat,
                lon: data.lon
              })
            } catch (err) { 
              // Ignore malformed
            }
          }
          scheduleUpdate()
        }
      })()
    } catch (err) {
      console.error('[Occupancy] Watch error:', err)
    } finally {
      loading.value = false
    }
  }

  function stopWatching() {
    if (kvWatcher) { 
      // FIX: kvWatcher.stop() returns void in some versions/contexts.
      // Removed .catch() to prevent "Cannot read properties of undefined" error.
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
