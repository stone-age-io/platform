<template>
  <div class="marker-item-kv">
    <span class="kv-label">{{ item.label }}</span>
    <div class="kv-value-row">
      <span class="kv-value" :class="{ 'is-empty': isEmpty }">
        {{ displayValue }}
      </span>
    </div>
    <div v-if="error" class="kv-error">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'
import { Kvm } from '@nats-io/kv'
import { JSONPath } from 'jsonpath-plus'
import type { MapMarkerItem } from '@/types/dashboard'

/**
 * Marker Item: KV Display
 * 
 * Watches a NATS KV bucket/key and displays the value.
 * Supports JSONPath extraction.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()

const value = ref<any>(null)
const error = ref<string | null>(null)
const isEmpty = ref(false)

let watcher: any = null

const displayValue = computed(() => {
  if (error.value) return '—'
  if (isEmpty.value) return '<empty>'
  if (value.value === null) return '...'
  
  // Format objects as JSON
  if (typeof value.value === 'object') {
    return JSON.stringify(value.value)
  }
  return String(value.value)
})

async function startWatcher() {
  if (!natsStore.nc || !natsStore.isConnected || !props.item.kvConfig) return
  
  const cfg = props.item.kvConfig
  const bucket = resolveTemplate(cfg.kvBucket, dashboardStore.currentVariableValues)
  const key = resolveTemplate(cfg.kvKey, dashboardStore.currentVariableValues)
  
  if (!bucket || !key) return
  
  error.value = null
  isEmpty.value = false
  
  try {
    const kvm = new Kvm(natsStore.nc)
    const kv = await kvm.open(bucket)
    
    // Get initial value
    try {
      const entry = await kv.get(key)
      if (entry) {
        processValue(entry.value)
      } else {
        isEmpty.value = true
      }
    } catch (e: any) {
      if (e.message?.includes('key not found')) {
        isEmpty.value = true
      } else {
        throw e
      }
    }
    
    // Start watching for updates
    const iter = await kv.watch({ key })
    watcher = iter
    
    ;(async () => {
      try {
        for await (const e of iter) {
          if (e.key === key) {
            if (e.operation === 'PUT') {
              processValue(e.value!)
              isEmpty.value = false
            } else if (e.operation === 'DEL' || e.operation === 'PURGE') {
              value.value = null
              isEmpty.value = true
            }
          }
        }
      } catch {
        // Watcher ended
      }
    })()
    
  } catch (err: any) {
    console.error('[MarkerItemKv] Watch error:', err)
    if (err.message?.includes('stream not found')) {
      error.value = `Bucket not found`
    } else {
      error.value = err.message || 'Watch failed'
    }
  }
}

function processValue(data: Uint8Array) {
  try {
    const text = decodeBytes(data)
    let parsed: any = text
    
    try {
      parsed = JSON.parse(text)
    } catch {
      // Keep as string
    }
    
    // Apply JSONPath if configured
    if (props.item.kvConfig?.jsonPath && typeof parsed === 'object') {
      try {
        const extracted = JSONPath({ 
          path: props.item.kvConfig.jsonPath, 
          json: parsed, 
          wrap: false 
        })
        if (extracted !== undefined) {
          parsed = extracted
        }
      } catch {
        // Keep original
      }
    }
    
    value.value = parsed
  } catch (err) {
    console.error('[MarkerItemKv] Process error:', err)
  }
}

function stopWatcher() {
  if (watcher) {
    try {
      watcher.stop()
    } catch {
      // Ignore cleanup errors
    }
    watcher = null
  }
}

// Lifecycle
onMounted(() => {
  if (natsStore.isConnected) {
    startWatcher()
  }
})

onUnmounted(() => {
  stopWatcher()
})

// Reconnection handling
watch(() => natsStore.isConnected, (connected) => {
  if (connected) {
    stopWatcher()
    startWatcher()
  } else {
    stopWatcher()
  }
})

// Variable changes - restart watcher
watch(() => dashboardStore.currentVariableValues, () => {
  if (natsStore.isConnected) {
    stopWatcher()
    startWatcher()
  }
}, { deep: true })
</script>

<style scoped>
.marker-item-kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.kv-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
  flex-shrink: 0;
}

.kv-value-row {
  display: flex;
  align-items: baseline;
  gap: 3px;
  min-width: 0;
}

.kv-value {
  font-size: 14px;
  font-weight: 600;
  font-family: var(--mono);
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kv-value.is-empty {
  color: var(--muted);
  font-style: italic;
  font-size: 12px;
}

.kv-error {
  font-size: 10px;
  color: var(--color-error);
  padding: 2px 6px;
  background: var(--color-error-bg);
  border-radius: 3px;
}
</style>
