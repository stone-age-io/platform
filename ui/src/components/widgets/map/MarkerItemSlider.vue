<template>
  <div class="marker-item-slider">
    <div class="slider-head">
      <span class="slider-label">{{ item.label }}</span>
      <span class="slider-value">
        {{ shownValue }}<span v-if="cfg.unit" class="slider-unit">{{ cfg.unit }}</span>
      </span>
    </div>

    <input
      v-model.number="localValue"
      type="range"
      class="slider-input"
      :min="cfg.min"
      :max="cfg.max"
      :step="cfg.step"
      :disabled="!natsStore.isConnected"
      :aria-label="item.label"
      :style="{ '--fill-percent': `${fillPercent}%` }"
      @input="isDragging = true"
      @change="handleRelease"
      @pointerup="handleRelease"
      @keyup.enter="handleRelease"
    />

    <div v-if="shownError" class="slider-error">{{ shownError }}</div>
    <div v-else-if="status" class="slider-status" :class="statusClass">{{ status }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useKvKey } from '@/composables/useKvKey'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { encodeString } from '@/utils/encoding'
import { extractJsonPath } from '@/utils/jsonPath'
import { fillTemplate } from '@/utils/templates'
import { resolveTemplate } from '@/utils/variables'
import { toNumber } from '@/utils/gaugeScale'
import type { DataSourceConfig, SliderWidgetConfig, MapMarkerItem } from '@/types/dashboard'

/**
 * Marker item: setpoint slider, the write-side counterpart to the switch row for
 * values that are a number (a setpoint, a damper position, a dim level).
 *
 * Deliberately shares no code with SliderWidget. useSwitchState exists because
 * switch state is subtle (a missing key is the default, a toggle is pending
 * until confirmed); a slider is "parse a number" plus one publish or KV put, and
 * the widget's extra code is layout and retry handling a panel row lacks. Same
 * for MarkerItemKv and KvWidget. Extract only if slider semantics grow subtle.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const subManager = getSubscriptionManager()

const requestConfirm = inject<
  (title: string, message: string, onConfirm: () => void, confirmText?: string) => void
>('requestConfirm')

// Typed, not inferred: an inferred literal has no optional fields, so every
// `cfg.value.unit` below would be a type error on the fallback branch.
const DEFAULTS: SliderWidgetConfig = { mode: 'kv', min: 0, max: 100, step: 1, defaultValue: 0 }
const cfg = computed<SliderWidgetConfig>(() => props.item.sliderConfig ?? DEFAULTS)
const isKv = computed(() => cfg.value.mode !== 'core')

const vars = computed(() => dashboardStore.currentVariableValues)

const localValue = ref<number>(cfg.value.defaultValue ?? 0)
/**
 * While the thumb is held, incoming values are ignored, so a retained KV value
 * or a device echo cannot yank it back mid-drag.
 */
const isDragging = ref(false)
const error = ref<string | null>(null)
const status = ref<string | null>(null)

// --- reading ----------------------------------------------------------------

const kv = useKvKey(
  () => (isKv.value ? resolveTemplate(cfg.value.kvBucket || '', vars.value) : ''),
  () => (isKv.value ? resolveTemplate(cfg.value.kvKey || '', vars.value) : ''),
)

const subscriptionId = computed(() => `marker-slider-${props.item.id}`)

/** Core mode reads the state subject, falling back to the one it publishes on. */
const resolvedStateSubject = computed(() => {
  if (isKv.value) return ''
  const c = cfg.value
  return resolveTemplate(c.stateSubject || c.publishSubject || '', vars.value)
})

const resolvedPublishSubject = computed(() =>
  resolveTemplate(cfg.value.publishSubject || '', vars.value)
)

let subscribedConfig: DataSourceConfig | null = null

function subscribe() {
  if (isKv.value || !natsStore.isConnected || !resolvedStateSubject.value) return
  const config: DataSourceConfig = {
    type: 'subscription',
    subject: resolvedStateSubject.value,
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

const incoming = computed<number | null>(() => {
  if (isKv.value) {
    // A key that was never written, or was deleted, shows the configured
    // default rather than whatever the previous key held.
    if (kv.status.value === 'missing') return cfg.value.defaultValue ?? 0
    if (kv.status.value !== 'ready' || !kv.entry.value) return null
    return toNumber(extractJsonPath(kv.entry.value.value, cfg.value.jsonPath))
  }
  const buffer = dataStore.getBuffer(subscriptionId.value)
  if (buffer.length === 0) return null
  return toNumber(buffer[buffer.length - 1].value)
})

watch(incoming, (v) => {
  if (v !== null && !isDragging.value) localValue.value = v
})

// --- writing ----------------------------------------------------------------

function handleRelease() {
  if (!isDragging.value) return
  isDragging.value = false
  const val = localValue.value

  if (cfg.value.confirmOnChange && requestConfirm) {
    const shown = `${val}${cfg.value.unit || ''}`
    const message = cfg.value.confirmMessage
      ? cfg.value.confirmMessage.split('{value}').join(shown)
      : `Set ${props.item.label} to ${shown}?`
    requestConfirm('Confirm change', message, () => void publishValue(val), 'Confirm')
    return
  }
  void publishValue(val)
}

async function publishValue(val: number) {
  if (!natsStore.isConnected) {
    error.value = 'Not connected'
    return
  }
  error.value = null

  let payload = String(val)
  if (cfg.value.valueTemplate) {
    payload = fillTemplate(cfg.value.valueTemplate, { value: String(val) })
    const trimmed = payload.trim()
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      try {
        payload = JSON.stringify(JSON.parse(trimmed))
      } catch {
        // Caught here rather than written: nearly-JSON would be stored as a
        // string nothing can parse, behind a write that looked successful.
        error.value = 'Invalid JSON template'
        return
      }
    }
  }

  try {
    if (isKv.value) {
      await kv.put(encodeString(payload))
    } else {
      if (!resolvedPublishSubject.value) {
        error.value = 'Publish subject required'
        return
      }
      natsStore.nc!.publish(resolvedPublishSubject.value, encodeString(payload))
    }
    status.value = `Sent ${val}${cfg.value.unit || ''}`
    window.setTimeout(() => { if (status.value?.startsWith('Sent')) status.value = null }, 2000)
  } catch (err: any) {
    error.value = err?.message || 'Write failed'
  }
}

// --- presentation -----------------------------------------------------------

function decimalsOf(step: number): number {
  const s = String(step)
  return s.includes('.') ? s.split('.')[1].length : 0
}

const shownValue = computed(() => localValue.value.toFixed(decimalsOf(cfg.value.step ?? 1)))

const fillPercent = computed(() => {
  const span = cfg.value.max - cfg.value.min
  if (!Number.isFinite(span) || span <= 0) return 0
  const clamped = Math.max(cfg.value.min, Math.min(cfg.value.max, localValue.value))
  return ((clamped - cfg.value.min) / span) * 100
})

const configMissing = computed(() => {
  if (!natsStore.isConnected) return null
  if (isKv.value) return !cfg.value.kvBucket || !cfg.value.kvKey ? 'KV bucket and key required' : null
  return !cfg.value.publishSubject ? 'Publish subject required' : null
})

const shownError = computed(() => error.value || kv.error.value || configMissing.value)
const statusClass = computed(() => (status.value?.startsWith('Sent') ? 'is-sent' : ''))

// --- lifecycle --------------------------------------------------------------

onMounted(() => {
  localValue.value = cfg.value.defaultValue ?? 0
  subscribe()
})

onUnmounted(unsubscribe)

watch(() => natsStore.isConnected, (connected) => {
  if (connected) subscribe()
})

watch(resolvedStateSubject, () => {
  unsubscribe()
  subscribe()
})
</script>

<style scoped>
.marker-item-slider {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.slider-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.slider-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.slider-value {
  font-family: var(--mono);
  font-size: 15px;
  font-weight: 700;
  color: var(--text);
}

.slider-unit {
  font-size: 0.7em;
  font-weight: 500;
  color: var(--muted);
  margin-left: 3px;
}

.slider-input {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  height: 10px;
  border-radius: 999px;
  outline: none;
  cursor: pointer;
  background: linear-gradient(
    to right,
    var(--color-primary) 0%,
    var(--color-primary) var(--fill-percent),
    oklch(var(--bc) / 0.1) var(--fill-percent),
    oklch(var(--bc) / 0.1) 100%
  );
}

.slider-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.slider-input::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--color-primary);
  border: 2px solid var(--panel);
  cursor: pointer;
}

.slider-input::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--color-primary);
  border: 2px solid var(--panel);
  cursor: pointer;
}

.slider-input:focus-visible {
  box-shadow: 0 0 0 2px var(--color-primary);
}

.slider-error {
  font-size: 11px;
  color: var(--color-error);
}

.slider-status {
  font-size: 11px;
  color: var(--muted);
}

.slider-status.is-sent {
  color: var(--color-success);
}
</style>
