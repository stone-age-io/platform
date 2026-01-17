<template>
  <div class="publisher-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Top Bar: Subject & History -->
    <div class="pub-header">
      <div class="input-group">
        <input 
          v-model="subject" 
          type="text" 
          class="subject-input" 
          placeholder="Subject..."
          @keyup.enter="focusPayload"
        />
        
        <!-- History Dropdown -->
        <div class="history-dropdown" v-if="history.length > 0">
          <button class="history-btn" @click="showHistory = !showHistory" title="Recent Messages">
            🕒
          </button>
          <div v-if="showHistory" class="history-menu">
            <div class="history-title">Recent Messages</div>
            <div 
              v-for="(item, idx) in history" 
              :key="idx" 
              class="history-item"
              @click="loadHistory(item)"
            >
              <div class="hist-sub">{{ item.subject }}</div>
              <div class="hist-pay">{{ truncate(item.payload) }}</div>
            </div>
            <div class="history-footer">
              <button class="clear-btn" @click="clearHistory">Clear History</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Middle: Payload Editor -->
    <div class="pub-body">
      <textarea 
        ref="payloadInput"
        v-model="payload" 
        class="payload-input" 
        placeholder="Message payload (JSON or text)..."
        spellcheck="false"
      ></textarea>
    </div>

    <!-- Bottom: Actions -->
    <div class="pub-footer">
      <div class="status-text" :class="statusType">
        {{ statusMessage }}
      </div>
      <div class="action-group">
        <button 
          class="btn-action secondary" 
          :disabled="isSending || !natsStore.isConnected"
          @click="handleSend('request')"
          title="Send Request and wait for Reply"
        >
          Request
        </button>
        <button 
          class="btn-action primary" 
          :disabled="isSending || !natsStore.isConnected"
          @click="handleSend('publish')"
          title="Fire and Forget"
        >
          Publish
        </button>
      </div>
    </div>

    <!-- Overlay for History Menu Click-away -->
    <div v-if="showHistory" class="click-overlay" @click="showHistory = false"></div>

    <!-- Response Modal -->
    <ResponseModal
      v-model="showResponse"
      :title="responseTitle"
      :status="responseStatus"
      :data="responseData"
      :latency="responseLatency"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import ResponseModal from '@/components/common/ResponseModal.vue'
import type { WidgetConfig, PublisherHistoryItem } from '@/types/dashboard'
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
const { updateWidgetConfiguration } = useWidgetOperations()

// State
const subject = ref('')
const payload = ref('')
const isSending = ref(false)
const statusMessage = ref('')
const statusType = ref<'info' | 'success' | 'error'>('info')
const showHistory = ref(false)
const payloadInput = ref<HTMLTextAreaElement | null>(null)

// Response Modal
const showResponse = ref(false)
const responseTitle = ref('')
const responseData = ref('')
const responseStatus = ref<'success' | 'error'>('success')
const responseLatency = ref(0)

// Config Access
const cfg = computed(() => props.config.publisherConfig || {})
const history = computed(() => cfg.value.history || [])

// Actions
function focusPayload() {
  payloadInput.value?.focus()
}

function truncate(str: string, len = 30) {
  return str.length > len ? str.substring(0, len) + '...' : str
}

function loadHistory(item: PublisherHistoryItem) {
  subject.value = item.subject
  payload.value = item.payload
  showHistory.value = false
}

function clearHistory() {
  updateWidgetConfiguration(props.config.id, {
    publisherConfig: {
      ...cfg.value,
      history: []
    }
  })
  showHistory.value = false
}

function addToHistory(sub: string, pay: string) {
  const newHistory = [
    { timestamp: Date.now(), subject: sub, payload: pay },
    ...history.value
  ].slice(0, 10) // Keep last 10

  // Persist to widget config
  updateWidgetConfiguration(props.config.id, {
    publisherConfig: {
      ...cfg.value,
      history: newHistory
    }
  })
}

async function handleSend(mode: 'publish' | 'request') {
  if (!natsStore.isConnected) {
    setStatus('Not connected', 'error')
    return
  }
  if (!subject.value.trim()) {
    setStatus('Subject required', 'error')
    return
  }

  isSending.value = true
  setStatus('Sending...', 'info')

  // Resolve variables
  const finalSubject = resolveTemplate(subject.value, dashboardStore.currentVariableValues)
  const finalPayloadStr = resolveTemplate(payload.value, dashboardStore.currentVariableValues)
  const data = encodeString(finalPayloadStr)

  // Add to history (save the raw input, not resolved, so variables are preserved)
  addToHistory(subject.value, payload.value)

  try {
    if (mode === 'publish') {
      natsStore.nc!.publish(finalSubject, data)
      setStatus(`Published to ${finalSubject}`, 'success')
      setTimeout(() => setStatus('', 'info'), 2000)
    } else {
      const start = Date.now()
      const timeout = cfg.value.timeout || 2000
      const msg = await natsStore.nc!.request(finalSubject, data, { timeout })
      const latency = Date.now() - start
      
      responseData.value = decodeBytes(msg.data)
      responseStatus.value = 'success'
      responseLatency.value = latency
      responseTitle.value = `Reply from ${finalSubject}`
      showResponse.value = true
      setStatus('Reply received', 'success')
    }
  } catch (err: any) {
    if (mode === 'request') {
      responseData.value = err.message || 'Request failed'
      responseStatus.value = 'error'
      responseLatency.value = 0
      responseTitle.value = 'Request Failed'
      showResponse.value = true
    }
    setStatus(err.message, 'error')
  } finally {
    isSending.value = false
  }
}

function setStatus(msg: string, type: 'info' | 'success' | 'error') {
  statusMessage.value = msg
  statusType.value = type
}

// Initialize
onMounted(() => {
  subject.value = cfg.value.defaultSubject || ''
  payload.value = cfg.value.defaultPayload || ''
})

// Watch for external config changes (e.g. undo/redo or remote update)
watch(() => props.config.publisherConfig, (newCfg) => {
  if (newCfg && !isSending.value) {
    // Only update if we aren't typing/sending to avoid overwriting user input
    // This is a bit loose, but prevents jarring resets
  }
}, { deep: true })
</script>

<style scoped>
.publisher-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--panel);
  position: relative;
}

.pub-header {
  padding: 8px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.input-group {
  display: flex;
  gap: 4px;
  position: relative;
}

.subject-input {
  flex: 1;
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px 8px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 13px;
}

.subject-input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.history-btn {
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 0 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}

.history-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

/* History Dropdown */
.history-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  width: 250px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 100;
  max-height: 300px;
  display: flex;
  flex-direction: column;
}

.history-title {
  padding: 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.1);
}

.history-item {
  padding: 8px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.2s;
}

.history-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.hist-sub {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-accent);
  margin-bottom: 2px;
  font-family: var(--mono);
}

.hist-pay {
  font-size: 11px;
  color: var(--muted);
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.history-footer {
  padding: 4px;
  text-align: center;
}

.clear-btn {
  background: none;
  border: none;
  color: var(--color-error);
  font-size: 11px;
  cursor: pointer;
  padding: 4px;
}

.clear-btn:hover {
  text-decoration: underline;
}

.click-overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
}

/* Body */
.pub-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 8px;
  min-height: 0;
}

.payload-input {
  flex: 1;
  width: 100%;
  resize: none;
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 8px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.4;
}

.payload-input:focus {
  outline: none;
  border-color: var(--color-accent);
}

/* Footer */
.pub-footer {
  padding: 8px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.status-text {
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.status-text.info { color: var(--muted); }
.status-text.success { color: var(--color-success); }
.status-text.error { color: var(--color-error); }

.action-group {
  display: flex;
  gap: 8px;
}

.btn-action {
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-action.primary {
  background: var(--color-primary);
  color: white;
}

.btn-action.primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.btn-action.secondary {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
  border: 1px solid var(--border);
}

.btn-action.secondary:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.15);
}
</style>
