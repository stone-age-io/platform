<template>
  <div class="slider-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    
    <!-- MOBILE / CARD LAYOUT -->
    <template v-if="layoutMode === 'card'">
      <div class="card-content" :class="{ 'is-disabled': isDisabled }">
        <!-- Header Row: Title and Value -->
        <div class="card-header">
          <div class="card-title">{{ config.title }}</div>
          <div class="card-value-display">
            <span class="card-value">{{ displayValue }}</span>
            <span v-if="cfg.unit" class="card-unit">{{ cfg.unit }}</span>
          </div>
        </div>

        <!-- Slider Row -->
        <div 
          class="card-slider-wrapper"
          @mousedown.stop
          @touchstart.stop
          @pointermove.stop
        >
          <input
            type="range"
            v-model.number="localValue"
            :min="cfg.min"
            :max="cfg.max"
            :step="cfg.step"
            :disabled="isDisabled"
            @mousedown="isDragging = true"
            @touchstart="isDragging = true"
            @mouseup="handleRelease"
            @touchend="handleRelease"
            @input="handleInput"
            class="slider-input mobile-input"
            :style="sliderStyle"
          />
        </div>

        <!-- Labels Row (New) -->
        <div class="card-slider-labels">
          <span>{{ cfg.min }}{{ cfg.unit }}</span>
          <span>{{ cfg.max }}{{ cfg.unit }}</span>
        </div>
      </div>
    </template>

    <!-- DESKTOP / STANDARD LAYOUT -->
    <template v-else>
      <div class="slider-container" :class="{ 'is-disabled': isDisabled }">
        <div class="value-display">
          <span class="value-number">{{ displayValue }}</span>
          <span v-if="cfg.unit" class="value-unit">{{ cfg.unit }}</span>
        </div>
        
        <div 
          class="slider-wrapper vue-grid-item-no-drag"
          @mousedown.stop
          @touchstart.stop
          @pointermove.stop
        >
          <input
            type="range"
            v-model.number="localValue"
            :min="cfg.min"
            :max="cfg.max"
            :step="cfg.step"
            :disabled="isDisabled"
            @mousedown="isDragging = true"
            @touchstart="isDragging = true"
            @mouseup="handleRelease"
            @touchend="handleRelease"
            @input="handleInput"
            class="slider-input"
            :style="sliderStyle"
          />
          
          <div class="slider-labels">
            <span class="label-min">{{ cfg.min }}{{ cfg.unit }}</span>
            <span class="label-max">{{ cfg.max }}{{ cfg.unit }}</span>
          </div>
        </div>
      </div>
    </template>
    
    <!-- Shared Status/Error Overlays -->
    <div v-if="publishStatus" class="publish-status" :class="statusClass">
      {{ publishStatus }}
    </div>
    
    <div v-if="error" class="error-message">
      {{ error }}
      <button class="retry-btn" @click="retry">Retry</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, inject } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { Kvm } from '@nats-io/kv'
import { JSONPath } from 'jsonpath-plus'
import type { WidgetConfig } from '@/types/dashboard'
import { encodeString, decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const requestConfirm = inject('requestConfirm') as (title: string, message: string, onConfirm: () => void) => void

const localValue = ref(0)
const isDragging = ref(false)
const error = ref<string | null>(null)
const publishStatus = ref<string | null>(null)
const pendingValue = ref<number | null>(null)

let kvWatcher: any = null
let kvInstance: any = null
let subscription: any = null

const cfg = computed(() => props.config.sliderConfig!)
const mode = computed(() => cfg.value.mode || 'core')
const isDisabled = computed(() => !natsStore.isConnected)

// Resolved configuration
const resolvedConfig = computed(() => {
  const vars = dashboardStore.currentVariableValues
  return {
    kvBucket: resolveTemplate(cfg.value.kvBucket, vars),
    kvKey: resolveTemplate(cfg.value.kvKey, vars),
    publishSubject: resolveTemplate(cfg.value.publishSubject, vars),
    stateSubject: resolveTemplate(cfg.value.stateSubject, vars),
  }
})

const displayValue = computed(() => {
  return localValue.value.toFixed(getDecimalPlaces(cfg.value.step))
})

function getDecimalPlaces(step: number): number {
  const str = step.toString()
  if (str.indexOf('.') === -1) return 0
  return str.split('.')[1].length
}

const fillPercent = computed(() => {
  const range = cfg.value.max - cfg.value.min
  const value = localValue.value - cfg.value.min
  return (value / range) * 100
})

const sliderStyle = computed(() => ({
  '--fill-percent': `${fillPercent.value}%`
}))

const statusClass = computed(() => {
  if (publishStatus.value?.includes('✓')) return 'status-success'
  if (publishStatus.value?.includes('⚠️')) return 'status-error'
  return ''
})

async function initialize() {
  if (!natsStore.isConnected) return
  error.value = null
  if (mode.value === 'kv') await initializeKv()
  else await initializeCore()
}

async function initializeKv() {
  const bucket = resolvedConfig.value.kvBucket
  const key = resolvedConfig.value.kvKey
  if (!bucket || !key) {
    error.value = 'KV Bucket/Key required'
    return
  }

  try {
    const kvm = new Kvm(natsStore.nc!)
    const kv = await kvm.open(bucket)
    kvInstance = kv

    try {
      const entry = await kv.get(key)
      if (entry) processIncomingData(decodeBytes(entry.value))
    } catch (e: any) {
      if (!e.message?.includes('key not found')) throw e
    }

    const iter = await kv.watch({ key })
    kvWatcher = iter
    ;(async () => {
      try {
        for await (const e of iter) {
          if (e.key === key && e.operation === 'PUT') {
            processIncomingData(decodeBytes(e.value!))
          }
        }
      } catch (err) {}
    })()
  } catch (err: any) {
    console.error('[Slider] KV Error:', err)
    if (err.message.includes('stream not found')) {
      error.value = `Bucket "${bucket}" not found`
    } else {
      error.value = err.message
    }
  }
}

async function initializeCore() {
  const subject = resolvedConfig.value.stateSubject || resolvedConfig.value.publishSubject
  if (!subject) return

  try {
    subscription = natsStore.nc!.subscribe(subject)
    ;(async () => {
      try {
        for await (const msg of subscription) {
          processIncomingData(decodeBytes(msg.data))
        }
      } catch (err) {}
    })()
  } catch (err: any) {
    console.error('[Slider] Sub Error:', err)
    error.value = err.message
  }
}

function processIncomingData(text: string) {
  if (isDragging.value) return
  try {
    let val: any = text
    try { val = JSON.parse(text) } catch {}
    if (props.config.jsonPath && typeof val === 'object') {
      const extracted = JSONPath({ path: props.config.jsonPath, json: val, wrap: false })
      if (extracted !== undefined) val = extracted
    }
    const num = Number(val)
    if (!isNaN(num)) localValue.value = num
  } catch (err) {
    console.warn('[Slider] Failed to process incoming data', err)
  }
}

function handleInput() {
  isDragging.value = true
}

function handleRelease() {
  isDragging.value = false
  
  if (cfg.value.confirmOnChange) {
    pendingValue.value = localValue.value
    const message = cfg.value.confirmMessage 
      ? cfg.value.confirmMessage.replace('{value}', displayValue.value + (cfg.value.unit || ''))
      : `Set value to ${displayValue.value}${cfg.value.unit || ''}?`
      
    requestConfirm('Confirm Change', message, () => {
      if (pendingValue.value !== null) publishValue(pendingValue.value)
      pendingValue.value = null
    })
  } else {
    publishValue(localValue.value)
  }
}

async function publishValue(val: number) {
  if (!natsStore.isConnected) {
    error.value = 'Not connected'
    return
  }

  error.value = null
  publishStatus.value = 'Publishing...'

  let payloadStr = String(val)
  if (cfg.value.valueTemplate) {
    payloadStr = cfg.value.valueTemplate.replace('{{value}}', String(val))
  }

  if (payloadStr.trim().startsWith('{') || payloadStr.trim().startsWith('[')) {
    try {
      payloadStr = JSON.stringify(JSON.parse(payloadStr))
    } catch (e) {
      error.value = "Invalid JSON Template"
      publishStatus.value = '⚠️ Error'
      return
    }
  }

  const data = encodeString(payloadStr)

  try {
    if (mode.value === 'kv') {
      if (!kvInstance || !resolvedConfig.value.kvKey) throw new Error('KV not initialized')
      await kvInstance.put(resolvedConfig.value.kvKey, data)
    } else {
      natsStore.nc!.publish(resolvedConfig.value.publishSubject, data)
    }
    
    publishStatus.value = `✓ Sent: ${val}${cfg.value.unit || ''}`
    setTimeout(() => {
      if (publishStatus.value?.includes('✓')) publishStatus.value = null
    }, 2000)
  } catch (err: any) {
    console.error('[Slider] Publish Error:', err)
    error.value = err.message
    publishStatus.value = '⚠️ Failed'
  }
}

function retry() {
  error.value = null
  initialize()
}

function cleanup() {
  if (kvWatcher) { try { kvWatcher.stop() } catch {} }
  if (subscription) { try { subscription.unsubscribe() } catch {} }
  kvWatcher = null
  subscription = null
  kvInstance = null
}

onMounted(() => {
  localValue.value = cfg.value.defaultValue
  if (natsStore.isConnected) initialize()
})

onUnmounted(cleanup)

watch(() => natsStore.isConnected, (connected) => {
  if (connected) initialize()
  else cleanup()
})

// Watch for config OR variables changing
watch([() => props.config.sliderConfig, () => dashboardStore.currentVariableValues], () => {
  cleanup()
  if (natsStore.isConnected) initialize()
}, { deep: true })
</script>

<style scoped>
.slider-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: var(--widget-bg);
  position: relative;
}

/* --- CARD LAYOUT STYLES --- */
.slider-widget.card-layout {
  padding: 12px;
  justify-content: flex-start;
}

.card-content {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px; /* Reduced gap to fit labels */
}

.card-content.is-disabled {
  opacity: 0.5;
  pointer-events: none;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  width: 100%;
  margin-bottom: 4px;
}

.card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-right: 8px;
}

.card-value-display {
  font-family: var(--mono);
  color: var(--color-accent);
  font-weight: 600;
  text-align: right;
  white-space: nowrap;
}

.card-value {
  font-size: 16px;
}

.card-unit {
  font-size: 12px;
  color: var(--muted);
  margin-left: 2px;
}

.card-slider-wrapper {
  width: 100%;
  display: flex;
  align-items: center;
  padding: 4px 0;
}

.card-slider-labels {
  display: flex;
  justify-content: space-between;
  width: 100%;
  font-size: 10px;
  color: var(--muted);
  font-family: var(--mono);
  margin-top: -2px;
}

/* --- STANDARD DESKTOP STYLES --- */
.slider-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 400px;
}

.slider-container.is-disabled {
  opacity: 0.5;
  pointer-events: none;
}

.value-display {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.value-number {
  font-size: clamp(24px, 20cqw, 48px);
  font-weight: 700;
  font-family: var(--mono);
  color: var(--color-accent);
  line-height: 1;
}

.value-unit {
  font-size: clamp(16px, 10cqw, 24px);
  font-weight: 500;
  color: var(--muted);
}

.slider-wrapper {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
  cursor: default;
}

/* --- SHARED SLIDER INPUT STYLES --- */
.slider-input {
  -webkit-appearance: none;
  width: 100%;
  height: 12px;
  border-radius: 6px;
  background: linear-gradient(
    to right,
    var(--color-accent) 0%,
    var(--color-accent) var(--fill-percent),
    rgba(255, 255, 255, 0.1) var(--fill-percent),
    rgba(255, 255, 255, 0.1) 100%
  );
  outline: none;
  cursor: pointer;
  transition: all 0.2s;
}

/* Mobile specific input tweaks */
.slider-input.mobile-input {
  height: 16px; /* Slightly taller track for touch */
}

.slider-input:hover {
  transform: scaleY(1.1);
}

.slider-input:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.slider-input::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: white;
  border: 3px solid var(--color-accent);
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  transition: all 0.2s;
}

/* Larger thumb for mobile touch */
.slider-input.mobile-input::-webkit-slider-thumb {
  width: 28px;
  height: 28px;
  box-shadow: 0 2px 6px rgba(0,0,0,0.4);
}

.slider-input::-webkit-slider-thumb:hover {
  transform: scale(1.2);
  box-shadow: 0 3px 12px rgba(0, 0, 0, 0.4);
}

.slider-labels {
  display: flex;
  justify-content: space-between;
  font-size: 18px;
  color: var(--muted);
  font-family: var(--mono);
}

@container (width < 150px) {
  .slider-labels { display: none; }
}

/* --- STATUS POPUP --- */
.publish-status {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 12px;
  font-weight: 500;
  padding: 6px 12px;
  border-radius: 4px;
  transition: all 0.3s;
  white-space: nowrap;
  animation: slideInUp 0.3s ease-out;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  z-index: 5;
}

/* 
   CARD LAYOUT OVERRIDE 
   Grug say: On mobile card, make status cover whole widget.
   No overlap. Just big clear message.
*/
.slider-widget.card-layout .publish-status {
  bottom: auto;
  top: 0;
  left: 0;
  right: 0;
  height: 100%;
  transform: none;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.85); /* High contrast background */
  backdrop-filter: blur(2px);
  border-radius: inherit; /* Matches card corners */
  z-index: 20;
  font-size: 14px;
  font-weight: 600;
  box-shadow: none;
  animation: fadeIn 0.2s ease-out;
}

@keyframes slideInUp {
  from { opacity: 0; transform: translateX(-50%) translateY(10px); }
  to { opacity: 1; transform: translateX(-50%) translateY(0); }
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.publish-status.status-success {
  color: var(--color-success);
  background: var(--color-success-bg);
}

/* In card layout, override background to be dark/neutral for readability */
.slider-widget.card-layout .publish-status.status-success {
  background: rgba(0, 0, 0, 0.85);
  color: var(--color-success);
  border: 1px solid var(--color-success-border);
}

.publish-status.status-error {
  color: var(--color-error);
  background: var(--color-error-bg);
}

.slider-widget.card-layout .publish-status.status-error {
  background: rgba(0, 0, 0, 0.85);
  color: var(--color-error);
  border: 1px solid var(--color-error-border);
}

.error-message {
  position: absolute;
  bottom: 8px;
  left: 8px;
  right: 8px;
  background: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 4px;
  padding: 8px;
  font-size: 11px;
  color: var(--color-error);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  z-index: 10;
}

.retry-btn {
  padding: 4px 8px;
  background: var(--color-error);
  color: white;
  border: none;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
}
</style>
