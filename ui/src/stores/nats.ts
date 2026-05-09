// ui/src/stores/nats.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { 
  wsconnect, 
  credsAuthenticator, 
  type NatsConnection
} from '@nats-io/nats-core'
import { useAuthStore } from './auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { NatsUser } from '@/types/pocketbase'

export const useNatsStore = defineStore('nats', () => {
  const authStore = useAuthStore()
  const toast = useToast()

  // State
  const nc = ref<NatsConnection | null>(null)
  const status = ref<'disconnected' | 'connecting' | 'connected' | 'reconnecting'>('disconnected')
  const lastError = ref<string | null>(null)
  const serverUrls = ref<string[]>([])
  const autoConnect = ref(false)

  // Stats
  const rtt = ref<number | null>(null)
  const reconnectCount = ref(0)
  let statsInterval: number | null = null

  // Monotonically incremented on every connect/disconnect. Async work captures
  // the generation it started in and aborts (or throws away its result) if a
  // newer op has superseded it. Prevents stale creds from binding when the user
  // rapidly switches orgs or edits identity.
  let opGen = 0

  const isConnected = computed(() => status.value === 'connected')

  function loadSettings() {
    const savedUrls = localStorage.getItem('stone_age_nats_urls')
    if (savedUrls) {
      try {
        serverUrls.value = JSON.parse(savedUrls)
      } catch {
        serverUrls.value = ['wss://localhost:9222']
      }
    } else {
      serverUrls.value = ['wss://localhost:9222']
    }

    const savedAuto = localStorage.getItem('stone_age_nats_autoconnect')
    autoConnect.value = savedAuto === 'true'
  }

  function saveSettings() {
    localStorage.setItem('stone_age_nats_urls', JSON.stringify(serverUrls.value))
    localStorage.setItem('stone_age_nats_autoconnect', String(autoConnect.value))
  }

  function addUrl(url: string) {
    if (!url) return
    if (!serverUrls.value.includes(url)) {
      serverUrls.value.push(url)
      saveSettings()
    }
  }

  function removeUrl(url: string) {
    serverUrls.value = serverUrls.value.filter(u => u !== url)
    saveSettings()
  }

  // Tear down any existing connection. Inline (not via disconnect()) so callers
  // like connect() can sequence teardown→setup under a single generation.
  async function teardownExisting() {
    if (statsInterval) {
      clearInterval(statsInterval)
      statsInterval = null
    }
    const oldNc = nc.value
    nc.value = null
    rtt.value = null
    if (oldNc) {
      try {
        await oldNc.drain()
      } catch {
        try { await oldNc.close() } catch { /* ignore */ }
      }
    }
  }

  // --- CONNECT LOGIC ---
  async function connect(specificUrl?: string) {
    const myGen = ++opGen

    await teardownExisting()
    if (myGen !== opGen) return  // superseded during teardown

    const servers = specificUrl ? [specificUrl] : serverUrls.value
    if (!servers.length) {
      if (myGen === opGen) {
        status.value = 'disconnected'
        toast.error('No NATS URL configured')
      }
      return
    }

    if (!authStore.isAuthenticated) {
      if (myGen === opGen) {
        status.value = 'disconnected'
        toast.error('You must be logged in to connect')
      }
      return
    }

    const natsUserId = authStore.currentMembership?.nats_user
    if (!natsUserId) {
      if (myGen === opGen) {
        status.value = 'disconnected'
        toast.error('No NATS Identity linked to this organization context')
      }
      return
    }

    status.value = 'connecting'
    lastError.value = null
    reconnectCount.value = 0

    try {
      const natsUserRecord = await pb.collection('nats_users').getOne<NatsUser>(natsUserId)
      if (myGen !== opGen) return  // superseded while fetching creds

      if (!natsUserRecord.creds_file) {
        throw new Error('Linked NATS identity has no credentials file')
      }

      const encoder = new TextEncoder()
      const credsBytes = encoder.encode(natsUserRecord.creds_file)

      console.log(`Connecting to NATS at ${servers.join(', ')} as ${natsUserRecord.nats_username}...`)

      const newNc = await wsconnect({
        servers,
        authenticator: credsAuthenticator(credsBytes),
        name: `stone-age-ui-${authStore.user?.id}`,
        maxReconnectAttempts: -1,
        reconnectTimeWait: 2_000,
        reconnectJitter: 1_000,
        pingInterval: 30_000,
        maxPingOut: 3,
      })

      if (myGen !== opGen) {
        // Superseded while wsconnect was resolving — close this orphan.
        try { await newNc.close() } catch { /* ignore */ }
        return
      }

      nc.value = newNc
      status.value = 'connected'

      monitorConnection(myGen)
      startStatsLoop()

    } catch (err: any) {
      if (myGen !== opGen) return
      console.error('NATS Connection Error:', err)
      status.value = 'disconnected'
      lastError.value = err.message
      if (err.status === 404) {
        toast.error('Linked NATS Identity no longer exists')
      } else {
        toast.error(`Connection failed: ${err.message}`)
      }
    }
  }

  async function disconnect() {
    const myGen = ++opGen
    await teardownExisting()
    if (myGen !== opGen) return  // superseded by a connect()
    status.value = 'disconnected'
    window.dispatchEvent(new Event('nats:closed'))
  }

  async function monitorConnection(forGen: number) {
    const ncRef = nc.value
    if (!ncRef) return
    for await (const s of ncRef.status()) {
      if (forGen !== opGen) return  // superseded; let the new monitor own status
      switch (s.type) {
        case 'disconnect':
          status.value = 'reconnecting'
          window.dispatchEvent(new Event('nats:disconnected'))
          break
        case 'reconnect':
          status.value = 'connected'
          reconnectCount.value++
          window.dispatchEvent(new Event('nats:reconnected'))
          break
        case 'error':
          lastError.value = String((s as any).data ?? s.type)
          console.error('NATS Error:', s)
          break
      }
    }
  }

  function startStatsLoop() {
    statsInterval = window.setInterval(async () => {
      if (nc.value && !nc.value.isClosed()) {
        try {
          rtt.value = await nc.value.rtt()
        } catch { /* ignore */ }
      }
    }, 10_000)
  }

  async function tryAutoConnect() {
    loadSettings()
    if (autoConnect.value && authStore.currentMembership?.nats_user) {
      await connect()
    }
  }

  return {
    nc, status, lastError, serverUrls, autoConnect, rtt, isConnected, reconnectCount,
    loadSettings, saveSettings, addUrl, removeUrl, connect, disconnect, tryAutoConnect
  }
})
