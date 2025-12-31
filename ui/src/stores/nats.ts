// src/stores/nats.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { 
  wsconnect, 
  credsAuthenticator, 
  type NatsConnection,
  type Msg,
  type NatsError
} from '@nats-io/nats-core'
import { useAuthStore } from './auth'
import { useToast } from '@/composables/useToast'

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
  let statsInterval: number | null = null

  // Computed
  const isConnected = computed(() => status.value === 'connected')

  // Load Settings from LocalStorage
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

  // Save Settings
  function saveSettings() {
    localStorage.setItem('stone_age_nats_urls', JSON.stringify(serverUrls.value))
    localStorage.setItem('stone_age_nats_autoconnect', String(autoConnect.value))
  }

  // Add/Remove URLs
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

  // Connect Logic
  async function connect(specificUrl?: string) {
    if (nc.value) await disconnect()

    // 1. Determine URL
    // Use specific URL if provided, otherwise try the first one in the list
    const url = specificUrl || serverUrls.value[0]
    if (!url) {
      toast.error('No NATS URL configured')
      return
    }

    // 2. Get Credentials from User Identity
    const natsUser = authStore.user?.expand?.nats_user
    if (!natsUser || !natsUser.creds_file) {
      toast.error('No NATS Identity linked to user account')
      return
    }

    status.value = 'connecting'
    lastError.value = null

    try {
      // 3. Prepare Authenticator
      const encoder = new TextEncoder()
      const credsBytes = encoder.encode(natsUser.creds_file)
      
      // 4. Connect
      nc.value = await wsconnect({ 
        servers: [url],
        authenticator: credsAuthenticator(credsBytes),
        name: `stone-age-ui-${authStore.user?.id}`,
      })

      status.value = 'connected'
      toast.success(`Connected to ${url}`)

      // 5. Monitor Connection
      monitorConnection()
      startStatsLoop()

    } catch (err: any) {
      console.error('NATS Connection Error:', err)
      status.value = 'disconnected'
      lastError.value = err.message
      toast.error(`Connection failed: ${err.message}`)
    }
  }

  // Disconnect Logic
  async function disconnect() {
    if (statsInterval) {
      clearInterval(statsInterval)
      statsInterval = null
    }

    if (nc.value) {
      await nc.value.close()
      nc.value = null
    }
    status.value = 'disconnected'
    rtt.value = null
  }

  // Internal: Monitor Status
  async function monitorConnection() {
    if (!nc.value) return
    for await (const s of nc.value.status()) {
      switch (s.type) {
        case 'disconnect':
          status.value = 'reconnecting'
          break
        case 'reconnect':
          status.value = 'connected'
          toast.success('Reconnected to NATS')
          break
        case 'error':
          console.error('NATS Error:', s.data)
          break
      }
    }
  }

  // Internal: Stats Loop
  function startStatsLoop() {
    statsInterval = setInterval(async () => {
      if (nc.value && !nc.value.isClosed()) {
        try {
          rtt.value = await nc.value.rtt()
        } catch { /* ignore */ }
      }
    }, 2000) as unknown as number
  }

  // Auto-connect helper called by App.vue
  function tryAutoConnect() {
    loadSettings()
    if (autoConnect.value && authStore.user?.expand?.nats_user) {
      connect()
    }
  }

  return {
    // State
    nc,
    status,
    lastError,
    serverUrls,
    autoConnect,
    rtt,
    isConnected,
    
    // Actions
    loadSettings,
    saveSettings,
    addUrl,
    removeUrl,
    connect,
    disconnect,
    tryAutoConnect
  }
})
