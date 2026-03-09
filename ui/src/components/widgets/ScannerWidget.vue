<!-- ui/src/components/widgets/ScannerWidget.vue -->
<template>
  <div class="scanner-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Idle State -->
    <div v-if="state === 'idle'" class="scanner-idle" @click="startScan">
      <div class="idle-icon">📷</div>
      <div class="idle-label">Tap to Scan</div>
    </div>

    <!-- Scanning State -->
    <div v-else-if="state === 'scanning'" class="scanner-scanning">
      <div :id="readerId" class="scanner-viewfinder"></div>
      <button class="btn btn-sm btn-ghost scanner-cancel" @click="stopScan">Cancel</button>
    </div>

    <!-- Loading State -->
    <div v-else-if="state === 'loading'" class="scanner-loading">
      <span class="loading loading-spinner loading-md"></span>
      <div class="loading-label">Looking up...</div>
      <div class="loading-value font-mono">{{ truncatedValue }}</div>
    </div>

    <!-- Result State -->
    <div v-else-if="state === 'result'" class="scanner-result">
      <div class="result-header">
        <span class="result-badge result-success">Found</span>
        <span class="result-scanned font-mono">{{ truncatedValue }}</span>
      </div>

      <div class="result-data">
        <!-- KV Result -->
        <div v-if="kvResult !== null" class="result-section">
          <div class="result-section-label">KV</div>
          <div v-if="typeof kvResult === 'object'" class="result-entries">
            <div v-for="(val, key) in kvResult" :key="String(key)" class="result-entry">
              <span class="entry-key">{{ key }}</span>
              <span class="entry-value font-mono">{{ formatValue(val) }}</span>
            </div>
          </div>
          <div v-else class="result-raw font-mono">{{ kvResult }}</div>
        </div>

        <!-- PB Result -->
        <div v-if="pbResult !== null" class="result-section">
          <div class="result-section-label">PocketBase</div>
          <div v-for="(item, idx) in pbItems" :key="idx" class="result-entries">
            <div v-for="(val, key) in item" :key="String(key)" class="result-entry">
              <span class="entry-key">{{ key }}</span>
              <span class="entry-value font-mono">{{ formatValue(val) }}</span>
            </div>
          </div>
          <div v-if="pbItems.length === 0" class="text-xs opacity-50 italic">No records</div>
        </div>
      </div>

      <button class="btn btn-sm btn-primary scanner-again" @click="reset">Scan Again</button>
    </div>

    <!-- Error State -->
    <div v-else-if="state === 'error'" class="scanner-error">
      <div class="error-icon">!</div>
      <div class="error-message">{{ errorMessage }}</div>
      <div v-if="scannedValue" class="error-scanned font-mono">{{ truncatedValue }}</div>
      <button class="btn btn-sm btn-primary scanner-again" @click="reset">Try Again</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted, nextTick } from 'vue'
import { Html5Qrcode, Html5QrcodeSupportedFormats } from 'html5-qrcode'
import { Kvm } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const natsStore = useNatsStore()

// --- State Machine ---
type ScanState = 'idle' | 'scanning' | 'loading' | 'result' | 'error'
const state = ref<ScanState>('idle')
const scannedValue = ref('')
const errorMessage = ref('')
const kvResult = ref<any>(null)
const pbResult = ref<any>(null)

let html5Qrcode: Html5Qrcode | null = null
const readerId = `scanner-${props.config.id}`
const decoder = new TextDecoder()

const cfg = computed(() => props.config.scannerConfig || {})

const truncatedValue = computed(() => {
  const v = scannedValue.value
  if (v.length <= 24) return v
  return v.slice(0, 10) + '...' + v.slice(-10)
})

const pbItems = computed(() => {
  if (!pbResult.value) return []
  if (Array.isArray(pbResult.value)) return pbResult.value
  return [pbResult.value]
})

// --- Scanner ---

async function startScan() {
  state.value = 'scanning'
  kvResult.value = null
  pbResult.value = null
  scannedValue.value = ''
  errorMessage.value = ''

  // Wait for DOM to render the viewfinder element with proper dimensions
  await nextTick()
  await new Promise(r => setTimeout(r, 100))

  try {
    html5Qrcode = new Html5Qrcode(readerId, {
      formatsToSupport: [Html5QrcodeSupportedFormats.QR_CODE],
      verbose: false,
    })
    await html5Qrcode.start(
      { facingMode: 'environment' },
      {
        fps: 10,
        // Use a function so qrbox adapts to actual rendered video size
        qrbox: (viewfinderWidth: number, viewfinderHeight: number) => {
          const minDim = Math.min(viewfinderWidth, viewfinderHeight)
          const size = Math.floor(minDim * 0.7) // 70% of smaller dimension
          return { width: size, height: size }
        },
      },
      handleScanResult,
      () => {} // ignore per-frame scan misses
    )
  } catch (err: any) {
    errorMessage.value = err.message || 'Camera access denied'
    state.value = 'error'
  }
}

async function stopScan() {
  if (html5Qrcode) {
    try {
      const scanState = html5Qrcode.getState()
      // 2 = SCANNING, 3 = PAUSED
      if (scanState === 2 || scanState === 3) {
        await html5Qrcode.stop()
      }
    } catch {
      // Already stopped
    }
    html5Qrcode.clear()
    html5Qrcode = null
  }
  if (state.value === 'scanning') {
    state.value = 'idle'
  }
}

async function handleScanResult(decodedText: string) {
  scannedValue.value = decodedText
  await stopScan()
  state.value = 'loading'
  await performLookup(decodedText)
}

function reset() {
  state.value = 'idle'
  scannedValue.value = ''
  kvResult.value = null
  pbResult.value = null
  errorMessage.value = ''
}

// --- Lookup ---

function replacePlaceholder(template: string, value: string): string {
  return template.replace(/\{value\}/g, value)
}

async function performLookup(value: string) {
  let hasResult = false
  const errors: string[] = []

  // KV Lookup
  if (cfg.value.kvEnabled && cfg.value.kvBucket) {
    try {
      const resolvedKey = replacePlaceholder(cfg.value.kvKeyTemplate || '{value}', value)
      const result = await kvGet(cfg.value.kvBucket, resolvedKey)
      if (result !== null) {
        kvResult.value = result
        hasResult = true
      }
    } catch (err: any) {
      errors.push(`KV: ${err.message}`)
    }
  }

  // PocketBase Lookup
  if (cfg.value.pbEnabled && cfg.value.pbCollection) {
    try {
      const filter = replacePlaceholder(cfg.value.pbFilter || '', value)
      const options: any = { requestKey: null }
      if (filter) options.filter = filter
      if (cfg.value.pbFields) options.fields = cfg.value.pbFields

      const result = await pb.collection(cfg.value.pbCollection).getList(1, 10, options)
      if (result.items.length > 0) {
        // Filter out PB system fields for cleaner display
        pbResult.value = result.items.map((item: any) => {
          const clean: Record<string, any> = {}
          for (const [k, v] of Object.entries(item)) {
            if (!['collectionId', 'collectionName', 'expand'].includes(k)) {
              clean[k] = v
            }
          }
          return clean
        })
        hasResult = true
      }
    } catch (err: any) {
      errors.push(`PB: ${err.message}`)
    }
  }

  // Publish audit event
  if (cfg.value.publishEnabled && cfg.value.publishSubject && natsStore.nc) {
    try {
      const encoder = new TextEncoder()
      const payload = JSON.stringify({
        value,
        found: hasResult,
        timestamp: new Date().toISOString(),
      })
      natsStore.nc.publish(cfg.value.publishSubject, encoder.encode(payload))
    } catch {
      // Non-critical, ignore
    }
  }

  if (hasResult) {
    state.value = 'result'
  } else {
    errorMessage.value = errors.length > 0
      ? errors.join('; ')
      : 'Not found'
    state.value = 'error'
  }
}

async function kvGet(bucket: string, key: string): Promise<any> {
  if (!natsStore.nc) throw new Error('NATS not connected')

  const kvm = new Kvm(natsStore.nc)
  const kvStore = await kvm.open(bucket)
  const entry = await kvStore.get(key)
  if (!entry || !entry.value) return null

  try {
    const str = decoder.decode(entry.value)
    try { return JSON.parse(str) } catch { return str }
  } catch {
    return '[Binary]'
  }
}

function formatValue(val: any): string {
  if (val === null || val === undefined) return '-'
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

// --- Cleanup ---
onUnmounted(() => {
  stopScan()
})
</script>

<style scoped>
.scanner-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.scanner-widget.card-layout {
  padding: 8px;
}

.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* --- Idle --- */
.scanner-idle {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: background 0.2s;
  border-radius: 4px;
}

.scanner-idle:hover {
  background: oklch(var(--b2) / 0.5);
}

.idle-icon {
  font-size: 48px;
  opacity: 0.6;
}

.idle-label {
  font-size: 14px;
  font-weight: 600;
  color: oklch(var(--bc) / 0.6);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* --- Scanning --- */
.scanner-scanning {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  min-height: 200px;
}

.scanner-viewfinder {
  flex: 1;
  min-height: 200px;
  width: 100%;
  border-radius: 4px;
  overflow: hidden;
  position: relative;
}

/* html5-qrcode injects a video element + shaded region canvas */
.scanner-viewfinder :deep(video) {
  width: 100% !important;
  height: 100% !important;
  object-fit: cover;
  border-radius: 4px;
}

.scanner-cancel {
  position: absolute;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  background: oklch(var(--b1) / 0.8) !important;
  backdrop-filter: blur(4px);
}

/* --- Loading --- */
.scanner-loading {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.loading-label {
  font-size: 14px;
  color: oklch(var(--bc) / 0.7);
}

.loading-value {
  font-size: 11px;
  color: oklch(var(--bc) / 0.4);
}

/* --- Result --- */
.scanner-result {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 12px;
  gap: 8px;
  min-height: 0;
}

.result-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.result-badge {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 2px 8px;
  border-radius: 100px;
}

.result-success {
  background: oklch(var(--su) / 0.15);
  color: oklch(var(--su));
}

.result-scanned {
  font-size: 11px;
  color: oklch(var(--bc) / 0.4);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-data {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.result-section {
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  padding: 8px;
}

.result-section-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: oklch(var(--bc) / 0.4);
  margin-bottom: 6px;
}

.result-entries {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.result-entry {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  padding: 2px 0;
  border-bottom: 1px solid oklch(var(--b3) / 0.5);
}

.result-entry:last-child {
  border-bottom: none;
}

.entry-key {
  font-size: 11px;
  color: oklch(var(--bc) / 0.5);
  flex-shrink: 0;
}

.entry-value {
  font-size: 11px;
  color: oklch(var(--bc));
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 200px;
}

.result-raw {
  font-size: 11px;
  color: oklch(var(--bc));
  word-break: break-all;
}

.scanner-again {
  flex-shrink: 0;
  align-self: center;
}

/* --- Error --- */
.scanner-error {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px;
  text-align: center;
}

.error-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: oklch(var(--er) / 0.15);
  color: oklch(var(--er));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
}

.error-message {
  font-size: 13px;
  color: oklch(var(--bc) / 0.7);
  max-width: 240px;
}

.error-scanned {
  font-size: 11px;
  color: oklch(var(--bc) / 0.4);
}
</style>
