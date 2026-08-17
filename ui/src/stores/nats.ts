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

const STORAGE_URLS = 'stone_age_nats_urls'
const STORAGE_AUTOCONNECT = 'stone_age_nats_autoconnect'

// Last resort, used only when the deployment configured nothing. ws:// rather
// than wss:// because it matches the listener `nats export` actually generates
// (websocket { port: 9222, no_tls: true }), and because this default only ever
// applies to local development, which is served over http where a browser
// permits ws://. Production sets nats.websocket_urls in config.yaml.
const FALLBACK_URL = 'ws://localhost:9222'

export const useNatsStore = defineStore('nats', () => {
  const authStore = useAuthStore()
  const toast = useToast()

  // State
  const nc = ref<NatsConnection | null>(null)
  const status = ref<'disconnected' | 'connecting' | 'connected' | 'reconnecting'>('disconnected')
  const lastError = ref<string | null>(null)
  const autoConnect = ref(false)

  // Server URLs come from three tiers, in strict priority:
  //
  //   1. userUrls    — this device's override (localStorage)
  //   2. defaultUrls — the deployment's setting (config.yaml, via /api/client-config)
  //   3. FALLBACK_URL — compiled in, development only
  //
  // The override REPLACES the defaults; the two are never merged. Two reasons,
  // both load-bearing. First, nats-core shuffles the server list by default
  // (noRandomize is false), so a merged list is a pool picked from at random,
  // not a priority order. Second, the reason a device overrides at all is to
  // talk to its local leaf node instead of the hub — and those are different
  // JetStream domains holding different data under the same bucket names. A
  // merged list would make *which dataset you are looking at* a coin flip per
  // connect and per reconnect.
  const defaultUrls = ref<string[]>([])
  const userUrls = ref<string[]>([])
  let defaultsLoaded = false

  const serverUrls = computed<string[]>(() => {
    if (userUrls.value.length) return userUrls.value
    if (defaultUrls.value.length) return defaultUrls.value
    return [FALLBACK_URL]
  })

  const usingOverride = computed(() => userUrls.value.length > 0)

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

  // Fetch the deployment's URLs once per session. Authenticated (the route is
  // bound to RequireAuth), so it is called from loadSettings rather than at app
  // boot — by the time anything connects, auth is hydrated.
  //
  // A failure is not fatal and deliberately does NOT mark the defaults as
  // loaded: an unreachable route at login should not leave the session stuck on
  // the fallback when opening Settings would have retried successfully.
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
    // An absent key means "no override" — the deployment default applies. Note
    // that an existing key from before this setting existed is read as an
    // override, which is the correct reading: somebody typed it deliberately.
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
    // No override is stored as an ABSENT key rather than an empty array, so
    // "has this device been overridden?" stays a single unambiguous check.
    if (userUrls.value.length) {
      localStorage.setItem(STORAGE_URLS, JSON.stringify(userUrls.value))
    } else {
      localStorage.removeItem(STORAGE_URLS)
    }
    localStorage.setItem(STORAGE_AUTOCONNECT, String(autoConnect.value))
  }

  // Adding a URL starts (or extends) this device's override. It never appends
  // to the deployment defaults — see the comment on defaultUrls for why the two
  // must not be merged. Multiple entries here mean "peers of one cluster".
  function addUrl(url: string) {
    if (!url) return
    if (!userUrls.value.includes(url)) {
      userUrls.value.push(url)
      saveSettings()
    }
  }

  // Only ever removes a device URL; the deployment defaults are read-only in
  // the UI. Removing the last one drops the override and the defaults apply
  // again, which is the same end state as resetToDefaults().
  function removeUrl(url: string) {
    userUrls.value = userUrls.value.filter(u => u !== url)
    saveSettings()
  }

  function resetToDefaults() {
    userUrls.value = []
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

    // serverUrls always yields at least the compiled-in fallback, so this guard
    // is a backstop rather than a normal path. Kept because wsconnect() with an
    // empty list silently dials its own localhost default instead of failing,
    // which is a far worse thing to debug than an explicit toast.
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
