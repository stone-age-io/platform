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

const STORAGE_URLS = 'stone_age_nats_urls'
const STORAGE_AUTOCONNECT = 'stone_age_nats_autoconnect'

// Used only when the deployment configured nothing (local development). ws://
// matches the no_tls listener `nats export` generates.
const FALLBACK_URL = 'ws://localhost:9222'

export const useNatsStore = defineStore('nats', () => {
  const authStore = useAuthStore()
  const toast = useToast()

  const nc =ref<NatsConnection | null>(null)
  const status = ref<'disconnected' | 'connecting' | 'connected' | 'reconnecting'>('disconnected')
  const lastError = ref<string | null>(null)
  const autoConnect = ref(false)

  // Server URLs, in priority order: userUrls (this device's override, in
  // localStorage), defaultUrls (config.yaml via /api/client-config), FALLBACK_URL.
  // The override REPLACES the defaults, never merges: nats-core shuffles the
  // server list, and an override usually points at a leaf node, a different
  // JetStream domain from the hub, so a merged list picks a dataset at random.
  const defaultUrls = ref<string[]>([])
  const userUrls = ref<string[]>([])
  let defaultsLoaded = false

  const serverUrls = computed<string[]>(() => {
    if (userUrls.value.length) return userUrls.value
    if (defaultUrls.value.length) return defaultUrls.value
    return [FALLBACK_URL]
  })

  const usingOverride = computed(() => userUrls.value.length > 0)

  const rtt = ref<number | null>(null)
  const reconnectCount = ref(0)
  let statsInterval: number | null = null

  // Bumped on every connect/disconnect. Async work captures it and aborts if a
  // newer op superseded it, so stale creds never bind after a quick org switch.
  let opGen = 0

  const isConnected = computed(() => status.value === 'connected')

  // Once per session, from loadSettings rather than app boot, because the route
  // requires auth. A failure leaves defaultsLoaded false so a later call retries.
  async function loadDefaults() {
    if (defaultsLoaded) return
    try {
      const res = await pb.send<{ natsWebsocketUrls?: string[] }>('/api/client-config', { method: 'GET' })
      defaultUrls.value = Array.isArray(res?.natsWebsocketUrls)
        ? res.natsWebsocketUrls.filter(u => typeof u === 'string' && u)
        : []
      defaultsLoaded = true
    } catch (err) {
      console.debug('Client config unavailable; using device/fallback NATS URLs.', err)
    }
  }

  async function loadSettings() {
    // An absent key means no override.
    const savedUrls = localStorage.getItem(STORAGE_URLS)
    let parsed: unknown = null
    if (savedUrls) {
      try {
        parsed = JSON.parse(savedUrls)
      } catch {
        parsed = null
      }
    }
    userUrls.value = Array.isArray(parsed)
      ? parsed.filter((u): u is string => typeof u === 'string' && !!u)
      : []

    const savedAuto = localStorage.getItem(STORAGE_AUTOCONNECT)
    autoConnect.value = savedAuto === 'true'

    await loadDefaults()
  }

  function saveSettings() {
    // No override is an absent key, never an empty array.
    if (userUrls.value.length) {
      localStorage.setItem(STORAGE_URLS, JSON.stringify(userUrls.value))
    } else {
      localStorage.removeItem(STORAGE_URLS)
    }
    localStorage.setItem(STORAGE_AUTOCONNECT, String(autoConnect.value))
  }

  // Starts or extends this device's override, never the defaults (see
  // serverUrls). Multiple entries mean peers of one cluster.
  function addUrl(url: string) {
    if (!url) return
    if (!userUrls.value.includes(url)) {
      userUrls.value.push(url)
      saveSettings()
    }
  }

  // Device URLs only; removing the last one restores the defaults.
  function removeUrl(url: string) {
    userUrls.value = userUrls.value.filter(u => u !== url)
    saveSettings()
  }

  function resetToDefaults() {
    userUrls.value = []
    saveSettings()
  }

  // Separate from disconnect() so connect() can tear down and set up under one
  // generation.
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

  async function connect(specificUrl?: string) {
    const myGen = ++opGen

    await teardownExisting()
    if (myGen !== opGen) return  // superseded during teardown

    // A backstop (serverUrls always has the fallback): wsconnect() with an
    // empty list silently dials its own localhost default.
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

      console.debug(`Connecting to NATS at ${servers.join(', ')} as ${natsUserRecord.nats_username}...`)

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
        case 'close':
          // The client gave up (e.g. credential revoked, org suspended). A local
          // disconnect() bumps opGen first, so it never lands here.
          status.value = 'disconnected'
          window.dispatchEvent(new Event('nats:closed'))
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
    await loadSettings()
    if (autoConnect.value && authStore.currentMembership?.nats_user) {
      await connect()
    }
  }

  return {
    nc, status, lastError, serverUrls, defaultUrls, userUrls, usingOverride,
    autoConnect, rtt, isConnected, reconnectCount,
    loadSettings, saveSettings, addUrl, removeUrl, resetToDefaults,
    connect, disconnect, tryAutoConnect
  }
})
