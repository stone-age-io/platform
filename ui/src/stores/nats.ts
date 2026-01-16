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
  const reconnectCount = ref(0) // <--- ADDED THIS
  let statsInterval: number | null = null

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

  // --- CONNECT LOGIC ---
  async function connect(specificUrl?: string) {
    if (nc.value) await disconnect()

    // 1. Validations
    const url = specificUrl || serverUrls.value[0]
    if (!url) {
      toast.error('No NATS URL configured')
      return
    }

    if (!authStore.user) {
      toast.error('You must be logged in to connect')
      return
    }

    const natsUserId = (authStore.user as any).nats_user
    if (!natsUserId) {
      toast.error('No NATS Identity linked to user account')
      return
    }

    status.value = 'connecting'
    lastError.value = null
    reconnectCount.value = 0 // <--- RESET COUNT

    try {
      const natsUserRecord = await pb.collection('nats_users').getOne<NatsUser>(natsUserId)
      
      if (!natsUserRecord.creds_file) {
        throw new Error('Linked NATS identity has no credentials file')
      }

      const encoder = new TextEncoder()
      const credsBytes = encoder.encode(natsUserRecord.creds_file)
      
      console.log(`Connecting to NATS at ${url} as ${natsUserRecord.nats_username}...`)
      
      nc.value = await wsconnect({ 
        servers: [url],
        authenticator: credsAuthenticator(credsBytes),
        name: `stone-age-ui-${authStore.user.id}`,
      })

      status.value = 'connected'
      toast.success(`Connected to NATS`)

      monitorConnection()
      startStatsLoop()

    } catch (err: any) {
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

  async function monitorConnection() {
    if (!nc.value) return
    for await (const s of nc.value.status()) {
      switch (s.type) {
        case 'disconnect':
          status.value = 'reconnecting'
          break
        case 'reconnect':
          status.value = 'connected'
          reconnectCount.value++ // <--- INCREMENT COUNT
          toast.success('Reconnected to NATS')
          break
        case 'error':
          console.error('NATS Error:', s)
          break
      }
    }
  }

  function startStatsLoop() {
    statsInterval = setInterval(async () => {
      if (nc.value && !nc.value.isClosed()) {
        try {
          rtt.value = await nc.value.rtt()
        } catch { /* ignore */ }
      }
    }, 10000) as unknown as number
  }

  function tryAutoConnect() {
    loadSettings()
    if (autoConnect.value && (authStore.user as any)?.nats_user) {
      connect()
    }
  }

  return {
    nc, status, lastError, serverUrls, autoConnect, rtt, isConnected, reconnectCount, // <--- EXPORTED
    loadSettings, saveSettings, addUrl, removeUrl, connect, disconnect, tryAutoConnect
  }
})
