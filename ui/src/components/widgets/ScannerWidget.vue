<!-- ui/src/components/widgets/ScannerWidget.vue -->
<template>
  <div class="scanner-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Idle State -->
    <div v-if="state === 'idle'" class="scanner-idle">
      <div class="idle-scan" @click="startScan">
        <div class="idle-icon">📷</div>
        <div class="idle-label">Tap to Scan</div>
      </div>
      <div v-if="cfg.allowManualEntry ?? true" class="manual-entry">
        <input
          v-model="manualInput"
          type="text"
          class="manual-input font-mono"
          placeholder="Or type a value..."
          @keydown.enter="submitManual"
        />
        <button
          class="btn btn-xs btn-ghost"
          :disabled="!manualInput.trim()"
          @click="submitManual"
        >Go</button>
      </div>
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
      <div
        class="result-banner"
        :class="passed ? 'result-banner--go' : 'result-banner--nogo'"
      >
        <div class="banner-icon">{{ passed ? '✓' : '✕' }}</div>
        <div class="banner-main">
          <div class="banner-status">{{ passed ? 'GO' : 'NO-GO' }}</div>
          <div v-if="!passed" class="banner-reason">{{ outcome }}</div>
        </div>
      </div>

      <div class="result-scanned-row font-mono" :title="scannedValue">
        {{ scannedValue }}
      </div>

      <div class="result-data">
        <!-- KV Record (generic — any shape, nested objects flattened to dot-paths) -->
        <div v-if="kvRecord" class="result-section">
          <div class="result-section-label">Record</div>
          <div class="result-entries">
            <div
              v-for="row in flattenedKvRecord"
              :key="row.key"
              class="result-entry"
            >
              <span class="entry-key">{{ row.key }}</span>
              <span class="entry-value font-mono" :title="formatValue(row.value)">
                {{ formatValue(row.value) }}
              </span>
            </div>
          </div>
        </div>

        <!-- PB Result (non-badge fallback) -->
        <div v-if="pbResult !== null" class="result-section">
          <div class="result-section-label">PocketBase</div>
          <div v-for="(rows, idx) in flattenedPbItems" :key="idx" class="result-entries">
            <div v-for="row in rows" :key="row.key" class="result-entry">
              <span class="entry-key">{{ row.key }}</span>
              <span class="entry-value font-mono" :title="formatValue(row.value)">
                {{ formatValue(row.value) }}
              </span>
            </div>
          </div>
          <div v-if="flattenedPbItems.length === 0" class="text-xs opacity-50 italic">No records</div>
        </div>

        <!-- Publish Status -->
        <div v-if="publishedSubject || publishError" class="publish-block">
          <div v-if="publishedSubject" class="publish-success">
            <div class="publish-line">
              <span class="publish-label">Published →</span>
              <span class="publish-subject font-mono">{{ publishedSubject }}</span>
            </div>
            <button
              v-if="publishedPayload"
              type="button"
              class="btn btn-xs btn-ghost publish-toggle"
              @click="showPayload = !showPayload"
            >
              {{ showPayload ? 'Hide message' : 'View message' }}
            </button>
            <pre v-if="showPayload && publishedPayload" class="publish-payload">{{ prettyPayload }}</pre>
          </div>
          <div v-else class="publish-error">
            Publish failed: {{ publishError }}
          </div>
        </div>
      </div>

      <button class="btn btn-sm btn-primary scanner-again" @click="reset">Scan Again</button>
    </div>

    <!-- Error State -->
    <div v-else-if="state === 'error'" class="scanner-error">
      <div class="error-icon">!</div>
      <div class="error-message">{{ errorMessage }}</div>
      <div v-if="scannedValue" class="error-scanned font-mono">{{ truncatedValue }}</div>
      <div v-if="cfg.allowManualEntry ?? true" class="manual-entry">
        <input
          v-model="manualInput"
          type="text"
          class="manual-input font-mono"
          placeholder="Type a value..."
          @keydown.enter="submitManual"
        />
        <button
          class="btn btn-xs btn-ghost"
          :disabled="!manualInput.trim()"
          @click="submitManual"
        >Go</button>
      </div>
      <button class="btn btn-sm btn-primary scanner-again" @click="reset">Try Again</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted, nextTick } from 'vue'
import { Html5Qrcode, Html5QrcodeSupportedFormats } from 'html5-qrcode'
import { Kvm } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { useAuthStore } from '@/stores/auth'
import { pb } from '@/utils/pb'
import { encodeString, decodeBytes } from '@/utils/encoding'
import type { WidgetConfig } from '@/types/dashboard'
import { evaluateRules } from './scannerRules'
import { buildScanPublish } from './scannerPublish'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const natsStore = useNatsStore()
const authStore = useAuthStore()

// --- State Machine ---
type ScanState = 'idle' | 'scanning' | 'loading' | 'result' | 'error'
const state = ref<ScanState>('idle')
const scannedValue = ref('')
const errorMessage = ref('')
const kvRecord = ref<Record<string, any> | null>(null)
const pbResult = ref<any>(null)
const publishedSubject = ref<string | null>(null)
const publishedPayload = ref<string | null>(null)
const publishError = ref<string | null>(null)
const showPayload = ref(false)
const manualInput = ref('')

// outcome === 'ok' → GO; any other non-empty string → NO-GO label; '' → pending.
const outcome = ref<string>('')
const passed = computed(() => outcome.value === 'ok')

let html5Qrcode: Html5Qrcode | null = null
const readerId = `scanner-${props.config.id}`

// Dedup — each entry auto-expires via its own timer, no LRU bookkeeping needed.
const recentScans = new Map<string, ReturnType<typeof setTimeout>>()

function isDuplicate(value: string, windowMs: number): boolean {
  if (windowMs <= 0) return false
  if (recentScans.has(value)) return true
  const t = setTimeout(() => recentScans.delete(value), windowMs)
  recentScans.set(value, t)
  return false
}

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

// Flatten nested objects to dot-path rows so each leaf renders on its own line
// instead of being stringified and truncated. Arrays are kept as a single row
// (rendered via JSON.stringify by formatValue).
function flattenRecord(obj: any, prefix = ''): { key: string; value: any }[] {
  if (obj == null || typeof obj !== 'object' || Array.isArray(obj)) {
    return [{ key: prefix || 'value', value: obj }]
  }
  const out: { key: string; value: any }[] = []
  for (const [k, v] of Object.entries(obj)) {
    const nextKey = prefix ? `${prefix}.${k}` : k
    if (v != null && typeof v === 'object' && !Array.isArray(v)) {
      out.push(...flattenRecord(v, nextKey))
    } else {
      out.push({ key: nextKey, value: v })
    }
  }
  return out
}

const flattenedKvRecord = computed(() =>
  kvRecord.value ? flattenRecord(kvRecord.value) : []
)

const flattenedPbItems = computed(() =>
  pbItems.value.map(item => flattenRecord(item))
)

const prettyPayload = computed(() => {
  if (!publishedPayload.value) return ''
  try {
    return JSON.stringify(JSON.parse(publishedPayload.value), null, 2)
  } catch {
    return publishedPayload.value
  }
})

// --- Scanner ---

async function startScan() {
  state.value = 'scanning'
  resetResults()

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
        qrbox: (viewfinderWidth: number, viewfinderHeight: number) => {
          const minDim = Math.min(viewfinderWidth, viewfinderHeight)
          const size = Math.floor(minDim * 0.7)
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
  await stopScan()
  await processValue(decodedText)
}

async function submitManual() {
  const v = manualInput.value.trim()
  if (!v) return
  manualInput.value = ''
  await processValue(v)
}

async function processValue(value: string) {
  if (isDuplicate(value, cfg.value.dedupWindowMs ?? 3000)) {
    scannedValue.value = value
    errorMessage.value = 'Duplicate scan suppressed'
    state.value = 'error'
    return
  }

  scannedValue.value = value
  state.value = 'loading'
  await performLookup(value)
}

function resetResults() {
  kvRecord.value = null
  pbResult.value = null
  publishedSubject.value = null
  publishedPayload.value = null
  publishError.value = null
  showPayload.value = false
  scannedValue.value = ''
  errorMessage.value = ''
  outcome.value = ''
}

function reset() {
  state.value = 'idle'
  resetResults()
}

// --- Lookup ---

function replacePlaceholder(template: string, value: string): string {
  return template.replace(/\{value\}/g, value)
}

function withTimeout<T>(p: Promise<T>, ms: number, label: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const t = setTimeout(() => reject(new Error(`${label} timed out after ${ms}ms`)), ms)
    p.then(v => { clearTimeout(t); resolve(v) }, e => { clearTimeout(t); reject(e) })
  })
}

async function performLookup(value: string) {
  const timeoutMs = cfg.value.lookupTimeoutMs ?? 5000
  let kvHadHit = false
  let pbHadHit = false
  let kvErr: string | null = null
  let pbErr: string | null = null

  // KV badge lookup
  if (cfg.value.kvEnabled && cfg.value.kvBucket) {
    try {
      const resolvedKey = replacePlaceholder(cfg.value.kvKeyTemplate || '{value}', value)
      const result = await withTimeout(kvGet(cfg.value.kvBucket, resolvedKey), timeoutMs, 'KV lookup')
      if (result !== null) {
        // Normalize non-object values into a single-field record so generic
        // rendering and rule evaluation still work.
        if (typeof result === 'object' && !Array.isArray(result)) {
          kvRecord.value = result as Record<string, any>
        } else {
          kvRecord.value = { value: result }
        }
        kvHadHit = true
      } else {
        kvErr = 'Not found'
      }
    } catch (err: any) {
      kvErr = err.message || 'KV lookup failed'
    }
  }

  // PB lookup — optional, for non-badge asset/thing scans
  if (cfg.value.pbEnabled && cfg.value.pbCollection) {
    try {
      const filter = replacePlaceholder(cfg.value.pbFilter || '', value)
      const options: any = { requestKey: null }
      if (filter) options.filter = filter
      if (cfg.value.pbFields) options.fields = cfg.value.pbFields

      const result = await withTimeout(
        pb.collection(cfg.value.pbCollection).getList(1, 10, options),
        timeoutMs,
        'PB lookup'
      )
      if (result.items.length > 0) {
        pbResult.value = result.items.map((item: any) => {
          const clean: Record<string, any> = {}
          for (const [k, v] of Object.entries(item)) {
            if (!['collectionId', 'collectionName', 'expand'].includes(k)) {
              clean[k] = v
            }
          }
          return clean
        })
        pbHadHit = true
      } else {
        pbErr = 'No matching records'
      }
    } catch (err: any) {
      pbErr = err.message || 'PB lookup failed'
    }
  }

  // Decide GO/NO-GO + reason
  if (cfg.value.kvEnabled) {
    if (!kvHadHit) {
      outcome.value = 'No record found'
    } else {
      const v = evaluateRules(kvRecord.value, cfg.value.rules)
      outcome.value = v.ok ? 'ok' : v.reason
    }
  } else if (cfg.value.pbEnabled) {
    outcome.value = pbHadHit ? 'ok' : 'No record found'
  } else {
    outcome.value = 'No lookup source enabled'
  }

  // Publish — fixed payload shape; operator controls only the subject template.
  if (cfg.value.publishEnabled && natsStore.nc) {
    try {
      const record = kvRecord.value
        ?? (Array.isArray(pbResult.value) ? pbResult.value[0] : pbResult.value)
        ?? null
      const { subject, payload } = buildScanPublish(cfg.value, {
        value,
        passed: passed.value,
        reason: outcome.value,
        scanner: authStore.currentNatsUser?.public_key || '',
        scannerKind: (pb.authStore.record as any)?.collectionName === 'things' ? 'thing' : 'user',
        record,
      })
      if (!subject) throw new Error('Subject template resolved to empty')
      const payloadJson = JSON.stringify(payload)
      natsStore.nc.publish(subject, encodeString(payloadJson))
      publishedSubject.value = subject
      publishedPayload.value = payloadJson
    } catch (err: any) {
      publishError.value = err.message || String(err)
    }
  }

  // Decide final UI state — always show result so operator sees GO/NO-GO + reason
  if (kvHadHit || pbHadHit || cfg.value.kvEnabled) {
    // Haptic cue — no-op on browsers/devices without Vibration API (e.g. iOS Safari)
    if (typeof navigator !== 'undefined' && 'vibrate' in navigator) {
      navigator.vibrate(passed.value ? 80 : [60, 60, 60, 60, 60])
    }
    state.value = 'result'
  } else {
    // No enabled source produced anything
    const parts: string[] = []
    if (kvErr) parts.push(`KV: ${kvErr}`)
    if (pbErr) parts.push(`PB: ${pbErr}`)
    errorMessage.value = parts.length > 0 ? parts.join('; ') : 'Not found'
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
    const str = decodeBytes(entry.value)
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
  for (const t of recentScans.values()) clearTimeout(t)
  recentScans.clear()
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

.text-error {
  color: oklch(var(--er));
}

/* --- Idle --- */
.scanner-idle {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-radius: 4px;
}

.idle-scan {
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

.idle-scan:hover {
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

.manual-entry {
  display: flex;
  gap: 4px;
  padding: 4px 8px 8px;
  align-items: center;
  flex-shrink: 0;
}

.manual-input {
  flex: 1;
  min-width: 0;
  padding: 4px 8px;
  font-size: 11px;
  border: 1px solid oklch(var(--b3));
  border-radius: 4px;
  background: oklch(var(--b1));
  color: oklch(var(--bc));
}

.manual-input:focus {
  outline: 2px solid oklch(var(--p) / 0.3);
  outline-offset: -2px;
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

.result-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  flex-shrink: 0;
  border-left: 4px solid transparent;
}

.result-banner--go {
  background: oklch(var(--su) / 0.18);
  border-left-color: oklch(var(--su));
  color: oklch(var(--su));
}

.result-banner--nogo {
  background: oklch(var(--er) / 0.18);
  border-left-color: oklch(var(--er));
  color: oklch(var(--er));
}

.banner-icon {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  flex-shrink: 0;
}

.banner-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.banner-status {
  font-size: 20px;
  font-weight: 800;
  letter-spacing: 1px;
  line-height: 1;
}

.banner-reason {
  font-size: 12px;
  opacity: 0.85;
  font-weight: 500;
}

.result-scanned-row {
  font-size: 12px;
  color: oklch(var(--bc) / 0.5);
  padding: 4px 2px;
  border-bottom: 1px dashed oklch(var(--b3));
  flex-shrink: 0;
  overflow-wrap: anywhere;
  white-space: normal;
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
  font-size: 11px;
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
  font-size: 13px;
  color: oklch(var(--bc) / 0.6);
  flex-shrink: 0;
}

.entry-value {
  font-size: 13px;
  color: oklch(var(--bc));
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 240px;
}

.publish-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 4px 4px;
  border-top: 1px dashed oklch(var(--b3));
  flex-shrink: 0;
  overflow-wrap: anywhere;
}

.publish-line {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: baseline;
  font-size: 13px;
}

.publish-label {
  color: oklch(var(--bc) / 0.5);
  font-weight: 600;
}

.publish-subject {
  color: oklch(var(--bc));
  font-weight: 500;
  word-break: break-all;
}

.publish-toggle {
  align-self: flex-start;
}

.publish-payload {
  margin: 0;
  padding: 8px;
  background: oklch(var(--b2));
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: oklch(var(--bc));
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
  max-height: 200px;
}

.publish-error {
  color: oklch(var(--er));
  font-size: 13px;
  font-weight: 500;
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
