// ui/src/composables/useSubscriptionManager.ts
import { useNatsStore } from '@/stores/nats'
import { useWidgetDataStore } from '@/stores/widgetData'
import { JSONPath } from 'jsonpath-plus'
import type { Subscription } from '@nats-io/nats-core'
import { 
  jetstream, 
  jetstreamManager,
  type ConsumerMessages
} from '@nats-io/jetstream'
import { decodeBytes } from '@/utils/encoding'
import type { DataSourceConfig } from '@/types/dashboard'

/**
 * Subscription Manager (Modular v3 API + Ordered Consumers)
 * 
 * Grug say: 
 * 1. SDK v3 types use underscores: 'deliver_policy', 'filter_subjects'.
 * 2. No name in js.consumers.get() = Ordered Consumer.
 * 3. Ordered Consumers are stateless on server, perfect for 🔄 button.
 */

interface WidgetListener {
  widgetId: string
  jsonPath?: string
}

interface SubscriptionRef {
  key: string
  subject: string
  natsSubscription?: Subscription 
  iterator?: ConsumerMessages
  listeners: Map<string, WidgetListener>
  isActive: boolean
  isJetStream: boolean
  config: DataSourceConfig
  initPromise?: Promise<void>
  lastError?: string
}

interface QueuedMessage {
  widgetId: string
  value: any
  raw: any
  subject?: string
  timestamp: number
}

interface SubscriptionStats {
  messagesReceived: number
  messagesDropped: number
  lastDropTime: number | null
  subscriptionErrors: number
}

const MAX_QUEUE_SIZE = 5000 

export function useSubscriptionManager() {
  const natsStore = useNatsStore()
  const dataStore = useWidgetDataStore()
  
  const subscriptions = new Map<string, SubscriptionRef>()
  const stats: SubscriptionStats = {
    messagesReceived: 0,
    messagesDropped: 0,
    lastDropTime: null,
    subscriptionErrors: 0
  }
  
  const messageQueue: QueuedMessage[] = []
  let flushPending = false
  let reconnectListenerAttached = false
  let closeListenerAttached = false
  
  // --- Message Queue (Batching for Performance) ---
  function flushQueue() {
    if (messageQueue.length === 0) {
      flushPending = false
      return
    }
    const batch = [...messageQueue]
    messageQueue.length = 0
    dataStore.batchAddMessages(batch)
    requestAnimationFrame(flushQueue)
  }
  
  function queueMessage(widgetId: string, value: any, raw: any, subject?: string) {
    stats.messagesReceived++
    if (messageQueue.length >= MAX_QUEUE_SIZE) {
      const dropCount = 1000
      messageQueue.splice(0, dropCount)
      stats.messagesDropped += dropCount
      stats.lastDropTime = Date.now()
    }
    messageQueue.push({ widgetId, value, raw, subject, timestamp: Date.now() })
    if (!flushPending) {
      flushPending = true
      requestAnimationFrame(flushQueue)
    }
  }

  // --- Helpers ---
  function calculateStartTime(windowStr: string | undefined): string {
    const now = Date.now()
    let ms = 5 * 60 * 1000 // Default 5m
    if (windowStr) {
      const match = windowStr.match(/^(\d+)([smhd])$/)
      if (match) {
        const val = parseInt(match[1])
        const unit = match[2]
        switch (unit) {
          case 's': ms = val * 1000; break
          case 'm': ms = val * 60 * 1000; break
          case 'h': ms = val * 60 * 60 * 1000; break
          case 'd': ms = val * 24 * 60 * 60 * 1000; break
        }
      }
    }
    return new Date(now - ms).toISOString()
  }

  function getSubscriptionKey(widgetId: string, config: DataSourceConfig): string {
    if (config.useJetStream) return `js:${widgetId}:${config.subject}`
    return `core:${config.subject}`
  }

  // --- Event Listeners ---
  function ensureReconnectListener() {
    if (reconnectListenerAttached) return
    window.addEventListener('nats:reconnected', handleReconnect)
    window.addEventListener('nats:disconnected', handleDisconnect)
    reconnectListenerAttached = true
  }

  function ensureCloseListener() {
    if (closeListenerAttached) return
    window.addEventListener('nats:closed', handleClose)
    closeListenerAttached = true
  }

  function handleDisconnect() {
    for (const subRef of subscriptions.values()) subRef.isActive = false
  }

  function handleClose() {
    for (const subRef of subscriptions.values()) cleanupSubscription(subRef)
    subscriptions.clear()
  }

  async function handleReconnect() {
    const subsToRefresh = Array.from(subscriptions.entries())
    for (const [key, subRef] of subsToRefresh) {
      if (subRef.listeners.size === 0) {
        subscriptions.delete(key)
        continue
      }
      cleanupSubscription(subRef)
      try {
        await createSubscription(subRef)
      } catch (err) {
        console.error(`[SubMgr] Failed to refresh ${subRef.subject}:`, err)
        subRef.lastError = String(err)
        stats.subscriptionErrors++
      }
    }
  }

  // --- Lifecycle ---
  async function subscribe(widgetId: string, config: DataSourceConfig, jsonPath?: string) {
    if (!natsStore.nc) return
    const subject = config.subject
    if (!subject) return
    
    ensureReconnectListener()
    ensureCloseListener()
    
    const key = getSubscriptionKey(widgetId, config)
    let subRef = subscriptions.get(key)
    
    if (subRef) {
      if (subRef.initPromise) {
        try { await subRef.initPromise } catch { 
          subscriptions.delete(key)
          subRef = undefined
        }
      }
      subRef = subscriptions.get(key)
      if (subRef) {
        const isDead = !subRef.isActive || (!subRef.isJetStream && subRef.natsSubscription?.isClosed())
        if (isDead) {
          cleanupSubscription(subRef)
          subscriptions.delete(key)
        } else {
          subRef.listeners.set(widgetId, { widgetId, jsonPath })
          return
        }
      }
    }
    
    const newSubRef: SubscriptionRef = {
      key, subject,
      listeners: new Map([[widgetId, { widgetId, jsonPath }]]),
      isActive: true,
      isJetStream: !!config.useJetStream,
      config: { ...config },
    }
    
    subscriptions.set(key, newSubRef)
    newSubRef.initPromise = createSubscription(newSubRef)
    
    try {
      await newSubRef.initPromise
    } catch (err) {
      console.error(`[SubMgr] Subscription failed: ${subject}`, err)
      newSubRef.lastError = String(err)
      stats.subscriptionErrors++
    } finally {
      newSubRef.initPromise = undefined
    }
  }

  async function createSubscription(subRef: SubscriptionRef): Promise<void> {
    if (subRef.config.useJetStream) {
      await createJetStreamSubscription(subRef)
    } else {
      await createCoreSubscription(subRef)
    }
  }

  async function createCoreSubscription(subRef: SubscriptionRef): Promise<void> {
    if (!natsStore.nc) throw new Error('Not connected')
    const natsSub = natsStore.nc.subscribe(subRef.subject)
    subRef.natsSubscription = natsSub
    subRef.isActive = true
    processCoreMessages(subRef)
  }

  /**
   * Create JetStream Subscription using Ordered Consumer (v3 API)
   */
  async function createJetStreamSubscription(subRef: SubscriptionRef): Promise<void> {
    if (!natsStore.nc) throw new Error('Not connected')
    
    const js = jetstream(natsStore.nc)
    const jsm = await jetstreamManager(natsStore.nc)
    const config = subRef.config

    try {
      // 1. Find the stream name for the subject
      const streamName = await jsm.streams.find(subRef.subject)

      // 2. Build options using underscored keys from types.ts
      const consumerOpts: any = {
        filter_subjects: [subRef.subject],
      }

      switch (config.deliverPolicy) {
        case 'all': consumerOpts.deliver_policy = 'all'; break
        case 'last': consumerOpts.deliver_policy = 'last'; break
        case 'new': consumerOpts.deliver_policy = 'new'; break
        case 'last_per_subject': consumerOpts.deliver_policy = 'last_per_subject'; break
        case 'by_start_time':
          consumerOpts.deliver_policy = 'by_start_time'
          consumerOpts.opt_start_time = calculateStartTime(config.timeWindow)
          break
        default: consumerOpts.deliver_policy = 'last'
      }

      // 3. Get the consumer (No name provided = Ordered Consumer)
      const consumer = await js.consumers.get(streamName, consumerOpts)
      const messages = await consumer.consume()
      
      subRef.iterator = messages
      subRef.isActive = true
      
      processJetStreamMessages(subRef)
    } catch (err: any) {
      const errorMsg = `JetStream Error: ${err.message}`
      subRef.lastError = errorMsg
      for (const listener of subRef.listeners.values()) {
        queueMessage(listener.widgetId, null, { error: errorMsg }, subRef.subject)
      }
      throw err
    }
  }

  function unsubscribe(widgetId: string, config: DataSourceConfig) {
    if (!config.subject) return
    const key = getSubscriptionKey(widgetId, config)
    const subRef = subscriptions.get(key)
    if (!subRef) return
    subRef.listeners.delete(widgetId)
    if (subRef.listeners.size === 0) {
      cleanupSubscription(subRef)
      subscriptions.delete(key)
    }
  }

  // --- Processing ---
  async function processCoreMessages(subRef: SubscriptionRef) {
    if (!subRef.natsSubscription) return
    try {
      for await (const msg of subRef.natsSubscription) {
        if (!subRef.isActive) break
        dispatchMessage(subRef, msg.data, msg.subject)
      }
    } catch (err) {
      if (subRef.isActive) subRef.isActive = false
    }
  }

  async function processJetStreamMessages(subRef: SubscriptionRef) {
    if (!subRef.iterator) return
    try {
      for await (const msg of subRef.iterator) {
        if (!subRef.isActive) break
        dispatchMessage(subRef, msg.data, msg.subject)
      }
    } catch (err) {
      if (subRef.isActive) subRef.isActive = false
    }
  }

  function dispatchMessage(subRef: SubscriptionRef, rawData: Uint8Array, subject: string) {
    let data: any
    try {
      const text = decodeBytes(rawData)
      try { data = JSON.parse(text) } catch { data = text }
    } catch { return }
    
    for (const listener of subRef.listeners.values()) {
      try {
        let value = data
        if (listener.jsonPath) {
          value = extractJsonPath(data, listener.jsonPath)
        }
        queueMessage(listener.widgetId, value, data, subject)
      } catch { /* ignore */ }
    }
  }

  function cleanupSubscription(subRef: SubscriptionRef) {
    subRef.isActive = false
    if (subRef.natsSubscription) {
      try { subRef.natsSubscription.unsubscribe() } catch {}
      subRef.natsSubscription = undefined
    }
    if (subRef.iterator) {
      try { subRef.iterator.stop() } catch {}
      subRef.iterator = undefined
    }
  }
  
  function cleanupAll() {
    for (const subRef of subscriptions.values()) cleanupSubscription(subRef)
    subscriptions.clear()
    messageQueue.length = 0
    stats.messagesReceived = 0
    stats.messagesDropped = 0
    stats.lastDropTime = null
    stats.subscriptionErrors = 0
  }

  function extractJsonPath(data: any, path: string): any {
    if (!path || path === '$') return data
    try { return JSONPath({ path, json: data, wrap: false }) } catch { return null }
  }
  
  function getStats() {
    const subscriptionList: any[] = []
    for (const [key, subRef] of subscriptions.entries()) {
      subscriptionList.push({
        key, subject: subRef.subject,
        listenerCount: subRef.listeners.size,
        isActive: subRef.isActive,
        type: subRef.isJetStream ? 'JetStream (Ordered)' : 'Core',
        error: subRef.lastError || null
      })
    }
    return {
      subscriptionCount: subscriptions.size,
      queueSize: messageQueue.length,
      maxQueueSize: MAX_QUEUE_SIZE,
      messagesReceived: stats.messagesReceived,
      messagesDropped: stats.messagesDropped,
      lastDropTime: stats.lastDropTime,
      subscriptionErrors: stats.subscriptionErrors,
      subscriptions: subscriptionList,
    }
  }
  
  function isSubscribed(widgetId: string, config: DataSourceConfig): boolean {
    const key = getSubscriptionKey(widgetId, config)
    const subRef = subscriptions.get(key)
    return !!subRef && subRef.listeners.has(widgetId)
  }
  
  return { subscribe, unsubscribe, cleanupAll, extractJsonPath, getStats, isSubscribed }
}

let managerInstance: ReturnType<typeof useSubscriptionManager> | null = null
export function getSubscriptionManager(): ReturnType<typeof useSubscriptionManager> {
  if (!managerInstance) managerInstance = useSubscriptionManager()
  return managerInstance
}
export function resetSubscriptionManager(): void {
  if (managerInstance) {
    managerInstance.cleanupAll()
    managerInstance = null
  }
}
