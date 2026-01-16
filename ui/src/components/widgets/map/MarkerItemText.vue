<template>
  <div class="marker-item-text">
    <span class="text-label">{{ item.label }}</span>
    <div class="text-value-row">
      <span class="text-value" :class="{ 'is-stale': isStale }">
        {{ displayValue }}
      </span>
      <span v-if="item.textConfig?.unit" class="text-unit">
        {{ item.textConfig.unit }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { resolveTemplate } from '@/utils/variables'
import type { MapMarkerItem, DataSourceConfig } from '@/types/dashboard'

/**
 * Marker Item: Text Display
 * 
 * Subscribes to a NATS subject via the central subscription manager
 * and displays the extracted value from the widget data store.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const subManager = getSubscriptionManager()

// Use item.id as the unique identifier for subscriptions
const subscriptionId = computed(() => `marker-text-${props.item.id}`)

// Track staleness
const lastUpdate = ref<number>(0)
const isStale = ref(false)
let staleCheckInterval: number | null = null

// Get the latest value from the data store buffer
const latestMessage = computed(() => {
  const buffer = dataStore.getBuffer(subscriptionId.value)
  if (buffer.length === 0) return null
  return buffer[buffer.length - 1]
})

const displayValue = computed(() => {
  if (!latestMessage.value) return '...'
  const val = latestMessage.value.value
  if (val === null || val === undefined) return '—'
  return String(val)
})

// Build the data source config for the subscription manager
function buildDataSourceConfig(): DataSourceConfig | null {
  const cfg = props.item.textConfig
  if (!cfg?.subject) return null
  
  const subject = resolveTemplate(cfg.subject, dashboardStore.currentVariableValues)
  if (!subject) return null
  
  return {
    type: 'subscription',
    subject,
    useJetStream: cfg.useJetStream || false,
    deliverPolicy: cfg.deliverPolicy || 'last',
    timeWindow: cfg.timeWindow
  }
}

function subscribe() {
  if (!natsStore.isConnected) return
  
  const config = buildDataSourceConfig()
  if (!config) return
  
  // Initialize buffer for this item
  dataStore.initializeBuffer(subscriptionId.value, 10) // Small buffer, we only need latest
  
  // Subscribe via central manager
  subManager.subscribe(subscriptionId.value, config, props.item.textConfig?.jsonPath)
}

function unsubscribe() {
  const config = buildDataSourceConfig()
  if (!config) return
  
  subManager.unsubscribe(subscriptionId.value, config)
  dataStore.removeBuffer(subscriptionId.value)
}

// Watch for new messages to track staleness
watch(latestMessage, (msg) => {
  if (msg) {
    lastUpdate.value = msg.timestamp
    isStale.value = false
  }
})

function startStaleCheck() {
  staleCheckInterval = window.setInterval(() => {
    if (lastUpdate.value > 0 && Date.now() - lastUpdate.value > 30000) {
      isStale.value = true
    }
  }, 10000)
}

function stopStaleCheck() {
  if (staleCheckInterval) {
    clearInterval(staleCheckInterval)
    staleCheckInterval = null
  }
}

// Lifecycle
onMounted(() => {
  if (natsStore.isConnected) {
    subscribe()
  }
  startStaleCheck()
})

onUnmounted(() => {
  unsubscribe()
  stopStaleCheck()
})

// Reconnection handling
watch(() => natsStore.isConnected, (connected) => {
  if (connected) {
    subscribe()
  } else {
    // Don't unsubscribe on disconnect - manager handles reconnection
  }
})

// Variable changes - resubscribe with new resolved values
watch(() => dashboardStore.currentVariableValues, () => {
  if (natsStore.isConnected) {
    unsubscribe()
    subscribe()
  }
}, { deep: true })
</script>

<style scoped>
.marker-item-text {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.text-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
  flex-shrink: 0;
}

.text-value-row {
  display: flex;
  align-items: baseline;
  gap: 3px;
  min-width: 0;
}

.text-value {
  font-size: 14px;
  font-weight: 600;
  font-family: var(--mono);
  color: var(--text);
  transition: opacity 0.3s;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.text-value.is-stale {
  opacity: 0.5;
}

.text-unit {
  font-size: 11px;
  color: var(--muted);
  flex-shrink: 0;
}
</style>
