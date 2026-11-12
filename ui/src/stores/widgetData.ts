import { defineStore } from 'pinia'
import { ref, computed, shallowRef, triggerRef } from 'vue'

export interface BufferedMessage {
  timestamp: number
  value: any
  raw?: any
  subject?: string
  // JetStream stream sequence. With the subject it identifies one stored
  // message, so a replay onto a kept buffer can skip what it already holds.
  seq?: number
}

// A ref per buffer, so a message only triggers that widget's readers.
interface WidgetDataBuffer {
  widgetId: string
  messages:  import('vue').Ref<BufferedMessage[]>
  maxCount: number
  maxAge?: number
  // The widget's resolved subjects; kept data is dropped when this changes.
  source?: string
}

const MEMORY_LIMITS = {
  MAX_TOTAL_MESSAGES: 20000,
  MAX_BUFFER_SIZE_MB: 50,
  MAX_SINGLE_BUFFER: 2000,
  BYTES_PER_MESSAGE_ESTIMATE: 1024
}

export const useWidgetDataStore = defineStore('widgetData', () => {
  // shallowRef: the Map changes only when widgets come and go; call
  // triggerRef(buffers) after adding or removing one.
  const buffers = shallowRef<Map<string, WidgetDataBuffer>>(new Map())
  const memoryWarning = ref<string | null>(null)
  const cumulativeCount = ref(0)

  const activeBufferCount = computed(() => buffers.value.size)
  
  const totalBufferedCount = computed(() => {
    let total = 0
    for (const buffer of buffers.value.values()) {
      total += buffer.messages.value.length
    }
    return total
  })
  
  const memoryEstimate = computed(() => {
    return totalBufferedCount.value * MEMORY_LIMITS.BYTES_PER_MESSAGE_ESTIMATE
  })
  
  const hasMemoryWarning = computed(() => memoryWarning.value !== null)
  
  const memoryUsagePercent = computed(() => {
    const percent = (totalBufferedCount.value / MEMORY_LIMITS.MAX_TOTAL_MESSAGES) * 100
    return Math.min(percent, 100)
  })

  /**
   * Create the buffer, or keep an existing one warm. If `source` differs from
   * what filled it (e.g. a variable moved `site.{device}.temp` to another
   * device), the old data is dropped.
   */
  function initializeBuffer(widgetId: string, maxCount: number = 100, maxAge?: number, source?: string) {
    const safeMaxCount = Math.min(maxCount, MEMORY_LIMITS.MAX_SINGLE_BUFFER)

    const existing = buffers.value.get(widgetId)
    if (!existing) {
      buffers.value.set(widgetId, {
        widgetId,
        messages: shallowRef([]),
        maxCount: safeMaxCount,
        maxAge,
        source,
      })
      triggerRef(buffers)
    } else if (source !== undefined && existing.source !== source) {
      existing.messages.value = []
      existing.source = source
    }
  }
  
  function checkMemoryPressure(): boolean {
    const totalMessages = totalBufferedCount.value
    
    if (totalMessages >= MEMORY_LIMITS.MAX_TOTAL_MESSAGES) {
      memoryWarning.value = `Memory limit reached (${totalMessages} msgs). Trimming history.`
      return true
    }
    
    if (totalMessages < MEMORY_LIMITS.MAX_TOTAL_MESSAGES * 0.8) {
      memoryWarning.value = null
    }
    
    return false
  }
  
  function pruneIfNeeded() {
    if (!checkMemoryPressure()) return

    for (const buffer of buffers.value.values()) {
      if (buffer.messages.value.length > 100) {
        const toRemove = Math.ceil(buffer.messages.value.length * 0.2)
        const newMsgs = buffer.messages.value.slice(toRemove)
        buffer.messages.value = newMsgs
      }
    }
  }
  
  function seenKey(m: BufferedMessage): string {
    return `${m.seq}\u0000${m.subject ?? ''}`
  }

  function dropSeen(current: BufferedMessage[], incoming: BufferedMessage[]): BufferedMessage[] {
    if (!incoming.some(m => m.seq !== undefined)) return incoming
    const seen = new Set<string>()
    for (const m of current) if (m.seq !== undefined) seen.add(seenKey(m))
    return incoming.filter(m => {
      if (m.seq === undefined) return true
      const k = seenKey(m)
      if (seen.has(k)) return false
      seen.add(k)
      return true
    })
  }

  function addMessage(widgetId: string, value: any, raw?: any, subject?: string) {
    batchAddMessages([{ widgetId, value, raw, subject, timestamp: Date.now() }])
  }

  function batchAddMessages(items: Array<{ 
    widgetId: string
    value: any
    raw?: any
    subject?: string
    timestamp: number
    seq?: number
  }>) {
    const currentVal = Number(cumulativeCount.value) || 0
    cumulativeCount.value = currentVal + items.length

    const updates = new Map<string, BufferedMessage[]>()
    
    for (const item of items) {
      let list = updates.get(item.widgetId)
      if (!list) {
        list = []
        updates.set(item.widgetId, list)
      }
      list.push({
        timestamp: item.timestamp,
        value: item.value,
        raw: item.raw,
        subject: item.subject,
        seq: item.seq,
      })
    }

    for (const [widgetId, incoming] of updates) {
      let buffer = buffers.value.get(widgetId)

      if (!buffer) {
        initializeBuffer(widgetId)
        buffer = buffers.value.get(widgetId)!
      }

      // A JetStream replay onto a kept buffer repeats what it holds; skip those.
      const newMessages = dropSeen(buffer.messages.value, incoming)
      if (newMessages.length === 0) continue

      const max = buffer.maxCount
      let currentMsgs = buffer.messages.value
      const incomingLen = newMessages.length

      let updatedMsgs: BufferedMessage[]

      if (incomingLen >= max) {
        updatedMsgs = newMessages.slice(incomingLen - max)
      } else {
        const currentLen = currentMsgs.length
        const availableSpace = max - currentLen
        
        if (incomingLen > availableSpace) {
          const toRemove = incomingLen - availableSpace
          updatedMsgs = currentMsgs.slice(toRemove).concat(newMessages)
        } else {
          updatedMsgs = currentMsgs.concat(newMessages)
        }
      }

      // Filter, not front-trim: timestamps may come from the payload or
      // JetStream, so arrival order is not time order.
      if (buffer.maxAge) {
        const cutoff = Date.now() - buffer.maxAge
        updatedMsgs = updatedMsgs.filter(m => m.timestamp >= cutoff)
      }

      buffer.messages.value = updatedMsgs
    }

    if (items.length > 100) {
      pruneIfNeeded()
    }
  }
  
  function getBuffer(widgetId: string): BufferedMessage[] {
    const buffer = buffers.value.get(widgetId)
    // A computed calling this tracks this buffer's ref, not the whole Map.
    return buffer ? buffer.messages.value : []
  }
  
  function getLatest(widgetId: string): any {
    const buffer = buffers.value.get(widgetId)
    if (!buffer || buffer.messages.value.length === 0) return null
    return buffer.messages.value[buffer.messages.value.length - 1].value
  }
  
  function getBufferSize(widgetId: string): number {
    const buffer = buffers.value.get(widgetId)
    return buffer ? buffer.messages.value.length : 0
  }
  
  function clearBuffer(widgetId: string) {
    const buffer = buffers.value.get(widgetId)
    if (buffer) {
      buffer.messages.value = []
    }
  }
  
  function removeBuffer(widgetId: string) {
    if (buffers.value.delete(widgetId)) {
      triggerRef(buffers)
    }
  }
  
  function clearAllBuffers() {
    buffers.value.clear()
    memoryWarning.value = null
    triggerRef(buffers)
  }
  
  function updateBufferConfig(widgetId: string, maxCount?: number, maxAge?: number) {
    const buffer = buffers.value.get(widgetId)
    if (!buffer) return
    
    if (maxCount !== undefined) {
      const safeMaxCount = Math.min(maxCount, MEMORY_LIMITS.MAX_SINGLE_BUFFER)
      buffer.maxCount = safeMaxCount
      
      if (buffer.messages.value.length > safeMaxCount) {
        const toRemove = buffer.messages.value.length - safeMaxCount
        buffer.messages.value = buffer.messages.value.slice(toRemove)
      }
    }
    if (maxAge !== undefined) buffer.maxAge = maxAge
  }
  
  function useLatestValue(widgetId: string) {
    return computed(() => getLatest(widgetId))
  }
  
  function useBuffer(widgetId: string) {
    return computed(() => getBuffer(widgetId))
  }
  
  return {
    buffers,
    memoryWarning,
    cumulativeCount,
    initializeBuffer,
    addMessage,
    batchAddMessages,
    getBuffer,
    getLatest,
    getBufferSize,
    clearBuffer,
    removeBuffer,
    clearAllBuffers,
    updateBufferConfig,
    useLatestValue,
    useBuffer,
    activeBufferCount,
    totalBufferedCount,
    memoryEstimate,
    hasMemoryWarning,
    memoryUsagePercent,
    MEMORY_LIMITS,
  }
})
