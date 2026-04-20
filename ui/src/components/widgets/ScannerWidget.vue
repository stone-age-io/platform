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
      <div class="result-header">
        <span
          class="result-badge"
          :class="found ? 'result-go' : 'result-nogo'"
        >{{ found ? 'GO' : 'NO-GO' }}</span>
        <span class="result-scanned font-mono">{{ truncatedValue }}</span>
      </div>

      <div v-if="!found" class="result-reason">
        {{ reasonLabel }}
      </div>

      <div class="result-data">
        <!-- KV Record (generic — any shape) -->
        <div v-if="kvRecord" class="result-section">
          <div class="result-section-label">Record</div>
          <div class="result-entries">
            <div
              v-for="(val, key) in kvRecord"
              :key="String(key)"
              class="result-entry"
            >
              <span class="entry-key">{{ key }}</span>
              <span class="entry-value font-mono">{{ formatValue(val) }}</span>
            </div>
          </div>
        </div>

        <!-- PB Result (non-badge fallback) -->
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

        <!-- Publish Status -->
        <div v-if="publishStatus" class="publish-status">
          {{ publishStatus }}
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
import type { WidgetConfig, ScannerRule } from '@/types/dashboard'

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
const publishStatus = ref<string | null>(null)
const manualInput = ref('')
const found = ref(false)
const reason = ref<string>('unknown')

let html5Qrcode: Html5Qrcode | null = null
const readerId = `scanner-${props.config.id}`

// Dedup tracking: value -> last-seen ms
const recentScans = new Map<string, number>()

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

const reasonLabel = computed(() => {
  const r = reason.value
  if (r === 'ok') return 'OK'
  if (r === 'unknown') return 'Unknown — no record found'
  return r
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
  // Dedup check
  const window = cfg.value.dedupWindowMs ?? 3000
  if (window > 0) {
    const now = Date.now()
    const last = recentScans.get(value)
    if (last !== undefined && now - last < window) {
      // Suppress — show briefly as the current result without re-publishing
      scannedValue.value = value
      errorMessage.value = 'Duplicate scan suppressed'
      state.value = 'error'
      return
    }
    recentScans.set(value, now)
    // Keep map bounded
    if (recentScans.size > 200) {
      const oldest = [...recentScans.entries()].sort((a, b) => a[1] - b[1])[0]
      if (oldest) recentScans.delete(oldest[0])
    }
  }

  scannedValue.value = value
  state.value = 'loading'
  await performLookup(value)
}

function resetResults() {
  kvRecord.value = null
  pbResult.value = null
  publishStatus.value = null
  scannedValue.value = ''
  errorMessage.value = ''
  found.value = false
  reason.value = 'unknown'
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
      found.value = false
      reason.value = 'unknown'
    } else {
      const v = evaluateRules(kvRecord.value, cfg.value.rules)
      found.value = v.ok
      reason.value = v.reason
    }
  } else if (cfg.value.pbEnabled) {
    found.value = pbHadHit
    reason.value = pbHadHit ? 'ok' : 'unknown'
  } else {
    found.value = false
    reason.value = 'unknown'
  }

  // Publish (templatable)
  if (cfg.value.publishEnabled && natsStore.nc) {
    try {
      const subject = renderSubject(
        cfg.value.publishSubjectTemplate || cfg.value.publishSubject || '',
        buildSubjectTokens(value)
      )
      const payloadTemplate = cfg.value.publishPayloadTemplate ||
        '{ "value": "{value}", "found": {found}, "ts": "{ts}" }'
      const payloadJson = renderPayload(payloadTemplate, buildPayloadTokens(value))
      if (!subject) throw new Error('Subject template resolved to empty')
      natsStore.nc.publish(subject, encodeString(payloadJson))
      publishStatus.value = `Published to ${subject}`
    } catch (err: any) {
      publishStatus.value = `Publish failed: ${err.message}`
    }
  }

  // Decide final UI state — always show result so operator sees GO/NO-GO + reason
  if (kvHadHit || pbHadHit || cfg.value.kvEnabled) {
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

function getByPath(obj: any, path: string): any {
  if (!obj || !path) return undefined
  return path.split('.').reduce((acc, k) => (acc == null ? acc : acc[k]), obj)
}

function evalRule(rec: any, r: ScannerRule): boolean {
  const v = getByPath(rec, r.field)
  switch (r.op) {
    case 'truthy':     return !!v
    case 'falsy':      return !v
    case 'equals':     return v === r.value
    case 'not_equals': return v !== r.value
    case 'in':         return Array.isArray(r.value) && r.value.includes(v)
    case 'not_in':     return Array.isArray(r.value) && !r.value.includes(v)
    case 'exists':     return v !== undefined && v !== null
    case 'missing':    return v === undefined || v === null
    case 'future': {
      // Absent value ≡ no expiry → pass. Mirrors legacy behavior for expires_at.
      if (v == null) return true
      const t = Date.parse(String(v))
      return !Number.isNaN(t) && t > Date.now()
    }
    case 'past': {
      if (v == null) return false
      const t = Date.parse(String(v))
      return !Number.isNaN(t) && t < Date.now()
    }
  }
}

function evaluateRules(rec: any, rules: ScannerRule[] | undefined): { ok: boolean; reason: string } {
  if (!rec) return { ok: false, reason: 'unknown' }
  if (!rules || rules.length === 0) return { ok: true, reason: 'ok' }
  for (const r of rules) {
    if (!evalRule(rec, r)) return { ok: false, reason: r.reason || `${r.field} ${r.op}` }
  }
  return { ok: true, reason: 'ok' }
}

// String tokens get JSON-escaped (without surrounding quotes) for safe injection
// inside "…" in the payload template. Raw tokens inject as-is (bool/JSON literal).
function escapeInsideQuotes(s: string): string {
  const j = JSON.stringify(s)
  return j.slice(1, j.length - 1)
}

function buildSubjectTokens(value: string): Record<string, string> {
  const scannerNkey = authStore.currentNatsUser?.public_key || ''
  const scannerKind = (pb.authStore.record as any)?.collectionName === 'things' ? 'thing' : 'user'
  return {
    value,
    scanner: scannerNkey,
    scanner_kind: scannerKind,
    device_label: cfg.value.deviceLabel || '',
    purpose: cfg.value.scanPurpose || 'other',
    location: cfg.value.location || '',
    found: found.value ? 'true' : 'false',
    reason: reason.value,
    ts: new Date().toISOString(),
  }
}

function buildPayloadTokens(value: string): Record<string, string> {
  const scannerNkey = authStore.currentNatsUser?.public_key || ''
  const scannerKind = (pb.authStore.record as any)?.collectionName === 'things' ? 'thing' : 'user'
  return {
    // String tokens — escaped for injection inside "..."
    value: escapeInsideQuotes(value),
    scanner: escapeInsideQuotes(scannerNkey),
    scanner_kind: escapeInsideQuotes(scannerKind),
    device_label: escapeInsideQuotes(cfg.value.deviceLabel || ''),
    purpose: escapeInsideQuotes(cfg.value.scanPurpose || 'other'),
    location: escapeInsideQuotes(cfg.value.location || ''),
    reason: escapeInsideQuotes(reason.value),
    ts: escapeInsideQuotes(new Date().toISOString()),
    // Raw-value tokens
    found: found.value ? 'true' : 'false',
    metadata: JSON.stringify(kvRecord.value?.metadata ?? kvRecord.value ?? {}),
  }
}

function renderSubject(template: string, tokens: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_m, tok) => tokens[tok] ?? '')
}

function renderPayload(template: string, tokens: Record<string, string>): string {
  // Operator controls types by quoting: "{ts}" → string, bare {found}/{metadata} → literal.
  const rendered = template.replace(/\{(\w+)\}/g, (_m, tok) => tokens[tok] ?? '')
  // Validate it parses to JSON; throws → caught by caller.
  JSON.parse(rendered)
  return rendered
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

.result-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.result-badge {
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 3px 10px;
  border-radius: 100px;
}

.result-go {
  background: oklch(var(--su) / 0.15);
  color: oklch(var(--su));
}

.result-nogo {
  background: oklch(var(--er) / 0.15);
  color: oklch(var(--er));
}

.result-scanned {
  font-size: 11px;
  color: oklch(var(--bc) / 0.4);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-reason {
  font-size: 12px;
  color: oklch(var(--er));
  flex-shrink: 0;
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

.publish-status {
  font-size: 10px;
  color: oklch(var(--bc) / 0.5);
  text-align: center;
  padding: 4px;
  border-top: 1px dashed oklch(var(--b3));
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
