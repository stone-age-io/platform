<template>
  <div class="status-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    
    <!-- CARD LAYOUT (Mobile) -->
    <template v-if="layoutMode === 'card'">
      <div class="card-content">
        <div class="card-info">
          <div class="card-title">{{ config.title }}</div>
          
          <div class="card-status-row">
            <div 
              class="status-dot-card"
              :class="{ 'is-blinking': currentState.blink }"
              :style="{ backgroundColor: currentState.color, boxShadow: `0 0 8px ${currentState.color}60` }"
            ></div>
            <div class="card-status-label" :style="{ color: currentState.color }">
              <span v-if="isStale" class="stale-icon">⚠️</span>
              {{ currentState.label }}
            </div>
          </div>
        </div>
        
        <div class="card-meta">{{ timeAgo }}</div>
      </div>
    </template>

    <!-- STANDARD LAYOUT (Desktop) -->
    <template v-else>
      <!-- Main Indicator -->
      <div 
        class="status-indicator"
        :class="{ 'is-blinking': currentState.blink }"
        :style="{ 
          backgroundColor: currentState.color,
          boxShadow: `0 0 20px ${currentState.color}40`
        }"
      >
        <div class="status-content">
          <span class="status-label">{{ currentState.label }}</span>
        </div>
      </div>

      <!-- Footer Info (Last Updated) -->
      <div class="status-footer">
        <span v-if="isStale" class="stale-warning">⚠️ Stale</span>
        <span class="time-ago">{{ timeAgo }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
import { useKvKey } from '@/composables/useKvKey'
import { resolveTemplate } from '@/utils/variables'
import { extractJsonPath } from '@/utils/jsonPath'
import type { WidgetConfigOf } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfigOf<'status'>
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dataStore = useWidgetDataStore()
const dashboardStore = useDashboardStore()

// Config Accessors
const cfg = computed(() => props.config.statusConfig)
const mappings = computed(() => cfg.value.mappings || [])
const staleThreshold = computed(() => cfg.value.stalenessThreshold || 60000)
const isKvMode = computed(() => props.config.dataSource.type === 'kv')

// Data
const buffer = computed(() => dataStore.getBuffer(props.config.id))
const latestMsg = computed(() => buffer.value[buffer.value.length - 1])

// State
const now = ref(Date.now())
let timer: number | null = null

// Computed State Logic
const isStale = computed(() => {
  if (!cfg.value.showStale) return false
  if (!latestMsg.value) return true // No data = stale
  return (now.value - latestMsg.value.timestamp) > staleThreshold.value
})

const displayValue = computed(() => {
  if (!latestMsg.value) return ''
  const val = latestMsg.value.value
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
})

const currentState = computed(() => {
  // 1. Check Staleness
  if (isStale.value) {
    return {
      color: cfg.value.staleColor || 'var(--muted)',
      label: cfg.value.staleLabel || 'Stale',
      blink: false
    }
  }

  // 2. Check Mappings
  const valStr = displayValue.value
  
  // Try exact match first
  let match = mappings.value.find(m => m.value === valStr)
  
  // If no exact match, try wildcard '*'
  if (!match) {
    match = mappings.value.find(m => m.value === '*')
  }
  
  if (match) {
    return {
      color: match.color,
      label: match.label || valStr,
      blink: match.blink
    }
  }

  // 3. Default
  return {
    color: cfg.value.defaultColor || 'var(--color-info)',
    label: cfg.value.defaultLabel || valStr || 'Unknown',
    blink: false
  }
})

const timeAgo = computed(() => {
  if (!latestMsg.value) return 'No data'
  const seconds = Math.floor((now.value - latestMsg.value.timestamp) / 1000)
  if (seconds < 60) return `${seconds}s` // Shortened for mobile card space
  return `${Math.floor(seconds / 60)}m`
})

// --- KV Logic ---
const kvKey = useKvKey(
  () => isKvMode.value ? resolveTemplate(props.config.dataSource.kvBucket, dashboardStore.currentVariableValues) : '',
  () => isKvMode.value ? resolveTemplate(props.config.dataSource.kvKey, dashboardStore.currentVariableValues) : '',
)

watch(kvKey.entry, (e) => {
  if (!isKvMode.value) return
  // No entry: the key was deleted, or a variable change points this widget at
  // another key. Either way the old value must not stand in for it.
  if (!e) {
    dataStore.clearBuffer(props.config.id)
    return
  }
  // The same shape a subscription delivers: parsed JSON, the widget's
  // JSONPath applied, and the time the value was WRITTEN. It used to push the
  // raw string stamped with the time it was read, so `$.state` never applied
  // and a key untouched for hours read as live for a minute after every load.
  dataStore.batchAddMessages([{
    widgetId: props.config.id,
    value: extractJsonPath(e.value, props.config.jsonPath),
    raw: e.value,
    subject: 'KV',
    timestamp: e.updated.getTime(),
  }])
})

// Lifecycle
onMounted(() => {
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.status-widget {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: var(--widget-bg);
  position: relative;
}

/* --- STANDARD LAYOUT --- */
.status-indicator {
  width: 100%;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: all 0.3s ease;
  min-height: 60px;
  max-height: 120px;
}

.status-indicator.is-blinking,
.status-dot-card.is-blinking {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.6; }
  100% { opacity: 1; }
}

.status-content {
  text-align: center;
  color: white; 
  text-shadow: 0 1px 2px rgba(0,0,0,0.3);
  padding: 0 8px;
  overflow: hidden;
}

.status-label {
  display: block;
  font-size: clamp(16px, 15cqw, 32px);
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-footer {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--muted);
  font-family: var(--mono);
}

.stale-warning {
  color: var(--color-warning);
  font-weight: 600;
}

/* --- CARD LAYOUT (MOBILE) --- */
.status-widget.card-layout {
  padding: 12px;
  display: flex;
  flex-direction: row;
  align-items: stretch;
}

.card-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.card-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  overflow: hidden;
  gap: 4px;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.card-status-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot-card {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}

.card-status-label {
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stale-icon {
  font-size: 12px;
  margin-right: 2px;
}

.card-meta {
  font-size: 11px;
  color: var(--muted);
  font-family: var(--mono);
  margin-left: 8px;
  align-self: flex-start;
  padding-top: 2px;
}
</style>
