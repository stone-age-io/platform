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
const JETSTREAM_CONSUMER_INACTIVE_THRESHOLD_MS = 120000 // 2 minutes

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
  
  // --- Message Queue ---
  
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
    
    messageQueue.push({
      widgetId,
      value,
      raw,
      subject,
      timestamp: Date.now()
    })
    
    if (!flushPending) {
      flushPending = true
      requestAnimationFrame(flushQueue)
    }
  }

  // --- Helpers ---

  function calculateStartTime(windowStr: string | undefined): Date {
    const now = Date.now()
    if (!windowStr) return new Date(now - 10 * 60 * 1000)
    
    const match = windowStr.match(/^(\d+)([smhd])$/)
    if (!match) return new Date(now - 10 * 60 * 1000)
    
    const val = parseInt(match[1])
    const unit = match[2]
    let ms = 0
    
    switch (unit) {
      case 's': ms = val * 1000; break
      case 'm': ms = val * 60 * 1000; break
      case 'h': ms = val * 60 * 60 * 1000; break
      case 'd': ms = val * 24 * 60 * 60 * 1000; break
    }
    
    return new Date(now - ms)
  }

  function getSubscriptionKey(widgetId: string, config: DataSourceConfig): string {
    if (config.useJetStream) {
      return `js:${widgetId}:${config.subject}`
    }
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
    for (const subRef of subscriptions.values()) {
      subRef.isActive = false
    }
  }

  function handleClose() {
    for (const subRef of subscriptions.values()) {
      cleanupSubscription(subRef)
    }
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
      
      const firstListener = subRef.listeners.values().next().value
      if (!firstListener) continue
      
      try {
        await createSubscription(subRef, firstListener.jsonPath)
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
      key,
      subject,
      listeners: new Map([[widgetId, { widgetId, jsonPath }]]),
      isActive: true,
      isJetStream: !!config.useJetStream,
      config: { ...config },
    }
    
    subscriptions.set(key, newSubRef)
    newSubRef.initPromise = createSubscription(newSubRef, jsonPath)
    
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

  async function createSubscription(subRef: SubscriptionRef, _jsonPath?: string): Promise<void> {
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

  async function createJetStreamSubscription(subRef: SubscriptionRef): Promise<void> {
    if (!natsStore.nc) throw new Error('Not connected')
    
    const config = subRef.config
    const jsm = await jetstreamManager(natsStore.nc)
    
    let streamName: string
    try {
      streamName = await jsm.streams.find(subRef.subject)
    } catch {
      const errorMsg = `No stream found for subject: ${subRef.subject}`
      subRef.lastError = errorMsg
      for (const listener of subRef.listeners.values()) {
        queueMessage(listener.widgetId, null, { error: errorMsg }, subRef.subject)
      }
      throw new Error(errorMsg)
    }

    const js = jetstream(natsStore.nc)
    
    const consumerOpts: any = {
      filter_subjects: [subRef.subject],
      deliver_policy: config.deliverPolicy || 'last',
      inactive_threshold: JETSTREAM_CONSUMER_INACTIVE_THRESHOLD_MS
    }

    if (config.deliverPolicy === 'by_start_time') {
      consumerOpts.opt_start_time = calculateStartTime(config.timeWindow).toISOString()
    }

    const consumer = await js.consumers.get(streamName, consumerOpts)
    const messages = await consumer.consume()
    
    subRef.iterator = messages
    subRef.isActive = true
    
    processJetStreamMessages(subRef)
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
        msg.ack()
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
    for (const subRef of subscriptions.values()) {
      cleanupSubscription(subRef)
    }
    subscriptions.clear()
    messageQueue.length = 0
    stats.messagesReceived = 0
    stats.messagesDropped = 0
    stats.lastDropTime = null
    stats.subscriptionErrors = 0
  }

  function extractJsonPath(data: any, path: string): any {
    if (!path || path === '$') return data
    try {
      return JSONPath({ path, json: data, wrap: false })
    } catch {
      return null
    }
  }
  
  function getStats() {
    const subscriptionList: any[] = []
    for (const [key, subRef] of subscriptions.entries()) {
      subscriptionList.push({
        key,
        subject: subRef.subject,
        listenerCount: subRef.listeners.size,
        isActive: subRef.isActive,
        type: subRef.isJetStream ? 'JetStream' : 'Core',
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
  
  return {
    subscribe,
    unsubscribe,
    cleanupAll,
    extractJsonPath,
    getStats,
    isSubscribed,
  }
}

let managerInstance: ReturnType<typeof useSubscriptionManager> | null = null

export function getSubscriptionManager(): ReturnType<typeof useSubscriptionManager> {
  if (!managerInstance) {
    managerInstance = useSubscriptionManager()
  }
  return managerInstance
}

export function resetSubscriptionManager(): void {
  if (managerInstance) {
    managerInstance.cleanupAll()
    managerInstance = null
  }
}
