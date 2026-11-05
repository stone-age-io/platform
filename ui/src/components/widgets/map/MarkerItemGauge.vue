<template>
  <div class="marker-item-gauge">
    <div class="gauge-head">
      <span class="gauge-label">{{ item.label }}</span>
      <span class="gauge-value" :style="{ color: valueColour }">
        {{ shownValue }}<span v-if="cfg.unit" class="gauge-unit">{{ cfg.unit }}</span>
      </span>
    </div>

    <div v-if="!rangeValid" class="gauge-problem">Set a max above the min</div>

    <template v-else>
      <div class="gauge-track">
        <div
          v-for="(zone, i) in spans"
          :key="i"
          class="gauge-zone"
          :style="{ left: `${zone.start * 100}%`, width: `${(zone.end - zone.start) * 100}%`, background: zone.color }"
        />
        <div
          v-if="value !== null"
          class="gauge-fill"
          :style="{ width: `${fraction * 100}%`, background: valueColour }"
        />
      </div>

      <div class="gauge-scale">
        <span>{{ cfg.min }}</span>
        <span>{{ cfg.max }}</span>
      </div>
    </template>

    <div v-if="error" class="gauge-error">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useKvKey } from '@/composables/useKvKey'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { extractJsonPath } from '@/utils/jsonPath'
import { resolveTemplate } from '@/utils/variables'
import {
  formatValue,
  rangeIsValid,
  toNumber,
  valueFraction,
  zoneColor,
  zoneSpans,
} from '@/utils/gaugeScale'
import type { DataSourceConfig, MapItemGaugeConfig, MapMarkerItem } from '@/types/dashboard'

/**
 * Marker Item: level bar.
 *
 * READ ONLY, and bar only. The arc wants a whole widget of space; at panel
 * width it would be a dial too small to read with a number too small to read
 * inside it. The scale arithmetic is utils/gaugeScale.ts, shared with the gauge
 * widget so the two cannot disagree about which band a value is in.
 *
 * It reads its own source rather than taking a value from the parent, for the
 * same reason every other marker row does: the panel is built fresh from a
 * marker and its items, and a row that needed feeding would make the parent
 * know what each row wants.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const subManager = getSubscriptionManager()

const FALLBACK_COLOUR = 'var(--color-primary)'

// Typed, not inferred: see MarkerItemSlider.
const DEFAULTS: MapItemGaugeConfig = { mode: 'kv', min: 0, max: 100 }
const cfg = computed<MapItemGaugeConfig>(() => props.item.gaugeConfig ?? DEFAULTS)
const isKv = computed(() => cfg.value.mode !== 'core')

const range = computed(() => ({ min: cfg.value.min, max: cfg.value.max }))
const rangeValid = computed(() => rangeIsValid(range.value))

const vars = computed(() => dashboardStore.currentVariableValues)

// --- kv mode ---------------------------------------------------------------

const kv = useKvKey(
  () => (isKv.value ? resolveTemplate(cfg.value.kvBucket || '', vars.value) : ''),
  () => (isKv.value ? resolveTemplate(cfg.value.kvKey || '', vars.value) : ''),
)

// --- core mode -------------------------------------------------------------

const subscriptionId = computed(() => `marker-gauge-${props.item.id}`)

const resolvedSubject = computed(() =>
  !isKv.value ? resolveTemplate(cfg.value.subject || '', vars.value) : ''
)

// Captured, not rebuilt: unsubscribing with a config rebuilt from the NEW
// variable values leaves the old subject feeding this row. MarkerItemText
// carries the same note; it is the same bug in the same place.
let subscribedConfig: DataSourceConfig | null = null

function subscribe() {
  if (isKv.value || !natsStore.isConnected || !resolvedSubject.value) return
  const config: DataSourceConfig = {
    type: 'subscription',
    subject: resolvedSubject.value,
    useJetStream: false,
  }
  dataStore.initializeBuffer(subscriptionId.value, 10)
  subManager.subscribe(subscriptionId.value, config, cfg.value.jsonPath)
  subscribedConfig = config
}

function unsubscribe() {
  if (subscribedConfig) subManager.unsubscribe(subscriptionId.value, subscribedConfig)
  subscribedConfig = null
  dataStore.removeBuffer(subscriptionId.value)
}

// --- the value -------------------------------------------------------------

const value = computed<number | null>(() => {
  if (isKv.value) {
    if (kv.status.value !== 'ready' || !kv.entry.value) return null
    return toNumber(extractJsonPath(kv.entry.value.value, cfg.value.jsonPath))
  }
  const buffer = dataStore.getBuffer(subscriptionId.value)
  if (buffer.length === 0) return null
  return toNumber(buffer[buffer.length - 1].value)
})

const error = computed(() => {
  if (isKv.value) {
    if (kv.error.value) return kv.error.value
    if (natsStore.isConnected && (!cfg.value.kvBucket || !cfg.value.kvKey)) {
      return 'KV bucket and key required'
    }
  } else if (natsStore.isConnected && !cfg.value.subject) {
    return 'Subject required'
  }
  return null
})

const shownValue = computed(() => formatValue(value.value, cfg.value.decimals))
const fraction = computed(() => valueFraction(value.value, range.value))
const spans = computed(() => zoneSpans(cfg.value.zones, range.value))
const valueColour = computed(() =>
  value.value === null
    ? 'var(--muted)'
    : zoneColor(value.value, cfg.value.zones, range.value, FALLBACK_COLOUR)
)

// --- lifecycle --------------------------------------------------------------

onMounted(subscribe)
onUnmounted(unsubscribe)

watch(() => natsStore.isConnected, (connected) => {
  // Not unsubscribed on disconnect: the manager handles reconnection.
  if (connected) subscribe()
})

watch(resolvedSubject, () => {
  unsubscribe()
  subscribe()
})
</script>

<style scoped>
.marker-item-gauge {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.gauge-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.gauge-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.gauge-value {
  font-family: var(--mono);
  font-size: 15px;
  font-weight: 700;
  transition: color 0.3s ease;
}

.gauge-unit {
  font-size: 0.7em;
  font-weight: 500;
  color: var(--muted);
  margin-left: 3px;
}

.gauge-track {
  position: relative;
  height: 10px;
  border-radius: 999px;
  background: oklch(var(--bc) / 0.1);
  overflow: hidden;
}

.gauge-zone {
  position: absolute;
  top: 0;
  bottom: 0;
  opacity: 0.3;
}

.gauge-fill {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  border-radius: 999px;
  transition: width 0.5s ease-out, background 0.3s ease;
}

.gauge-scale {
  display: flex;
  justify-content: space-between;
  font-family: var(--mono);
  font-size: 10px;
  color: var(--muted);
}

.gauge-problem,
.gauge-error {
  font-size: 11px;
  color: var(--color-error);
}
</style>
