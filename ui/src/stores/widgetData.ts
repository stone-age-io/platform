import { defineStore } from 'pinia'
import { ref, computed, shallowRef, triggerRef } from 'vue'

export interface BufferedMessage {
  timestamp: number
  value: any
  raw?: any
  subject?: string
}

// CHANGED: messages is now a Ref, allowing granular updates
interface WidgetDataBuffer {
  widgetId: string
  messages:  import('vue').Ref<BufferedMessage[]> 
  maxCount: number
  maxAge?: number
}

const MEMORY_LIMITS = {
  MAX_TOTAL_MESSAGES: 20000,
  MAX_BUFFER_SIZE_MB: 50,
  MAX_SINGLE_BUFFER: 2000,
  BYTES_PER_MESSAGE_ESTIMATE: 1024
}

export const useWidgetDataStore = defineStore('widgetData', () => {
  // State
  // CHANGED: Use shallowRef for the Map itself. We rarely add/remove widgets compared to messages.
  const buffers = shallowRef<Map<string, WidgetDataBuffer>>(new Map())
  const memoryWarning = ref<string | null>(null)
  const cumulativeCount = ref(0)
  
  // Computed
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

  // Actions
  function initializeBuffer(widgetId: string, maxCount: number = 100, maxAge?: number) {
    const safeMaxCount = Math.min(maxCount, MEMORY_LIMITS.MAX_SINGLE_BUFFER)
    
    if (!buffers.value.has(widgetId)) {
      buffers.value.set(widgetId, {
        widgetId,
        messages: shallowRef([]), // CHANGED: Individual reactive ref
        maxCount: safeMaxCount,
        maxAge,
      })
      triggerRef(buffers) // Only trigger map structure changes
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
        // Direct array mutation on the value
        const newMsgs = buffer.messages.value.slice(toRemove)
        buffer.messages.value = newMsgs
      }
    }
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
  }>) {
    const currentVal = Number(cumulativeCount.value) || 0
    cumulativeCount.value = currentVal + items.length

    // Group by widget
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
        subject: item.subject
      })
    }

    // Apply updates
    for (const [widgetId, newMessages] of updates) {
      let buffer = buffers.value.get(widgetId)
      
      if (!buffer) {
        initializeBuffer(widgetId)
        buffer = buffers.value.get(widgetId)!
      }

      const max = buffer.maxCount
      // Work with a copy to avoid intermediate reactivity triggers
      let currentMsgs = buffer.messages.value
      const incomingLen = newMessages.length

      let updatedMsgs: BufferedMessage[]

      if (incomingLen >= max) {
        // Incoming flood: just keep newest
        updatedMsgs = newMessages.slice(incomingLen - max)
      } else {
        // Normal case: make room and push
        const currentLen = currentMsgs.length
        const availableSpace = max - currentLen
        
        if (incomingLen > availableSpace) {
          const toRemove = incomingLen - availableSpace
          updatedMsgs = currentMsgs.slice(toRemove).concat(newMessages)
        } else {
          updatedMsgs = currentMsgs.concat(newMessages)
        }
      }
      
      // CHANGED: Update the specific ref for this widget only
      buffer.messages.value = updatedMsgs
    }

    // Check memory occasionally
    if (items.length > 100) {
      pruneIfNeeded()
    }
    
    // REMOVED: triggerRef(buffers) - No longer needed for data updates!
  }
  
  function getBuffer(widgetId: string): BufferedMessage[] {
    const buffer = buffers.value.get(widgetId)
    // CHANGED: Return the value of the ref. 
    // Computed properties using this will now track `buffer.messages`, not `buffers`.
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
