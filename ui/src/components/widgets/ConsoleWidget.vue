<template>
  <div class="console-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Toolbar -->
    <div class="console-toolbar">
      <div class="toolbar-left">
        <button 
          class="tool-btn" 
          :class="{ 'is-paused': isPaused }"
          @click="togglePause"
          :title="isPaused ? 'Resume scrolling' : 'Pause scrolling'"
        >
          {{ isPaused ? '▶' : '⏸' }}
        </button>
        <button class="tool-btn" @click="clear" title="Clear console">
          🚫
        </button>
      </div>
      
      <div class="toolbar-center">
        <input 
          v-model="filterText"
          type="text"
          class="filter-input"
          placeholder="Filter..."
        />
      </div>
      
      <div class="toolbar-right">
        <button 
          class="tool-btn text-btn" 
          @click="toggleExpand"
          :title="expandAll ? 'Collapse all' : 'Expand JSON'"
        >
          {{ expandAll ? '{ }' : '{...}' }}
        </button>
      </div>
    </div>

    <!-- Log List -->
    <div 
      class="console-body" 
      ref="scrollContainer"
      :style="{ fontSize: `${config.consoleConfig?.fontSize || 12}px` }"
    >
      <div v-if="displayMessages.length === 0" class="empty-state">
        <span v-if="filterText">No matching messages</span>
        <span v-else>Waiting for messages...</span>
      </div>

      <div 
        v-for="(msg, index) in displayMessages" 
        :key="msg.timestamp + '-' + index"
        class="log-entry"
      >
        <!-- Header Row: Time & Subject -->
        <div class="log-header">
          <span v-if="showTimestamp" class="log-time">
            {{ formatTime(msg.timestamp) }}
          </span>

          <span 
            v-if="msg.subject" 
            class="log-subject" 
            @click="copyText(msg.subject, 'Subject copied!')"
            title="Click to copy subject"
          >
            {{ msg.subject }}
          </span>
        </div>
        
        <!-- Content Row: Payload -->
        <div class="log-content">
          <div 
            class="content-wrapper" 
            @click="copyText(formatValue(msg.value), 'Payload copied!')" 
            title="Click to copy payload"
          >
            <JsonViewer 
              v-if="expandAll && isObject(msg.value)" 
              :data="msg.value" 
            />
            <span v-else class="log-text">{{ formatValue(msg.value) }}</span>
          </div>
        </div>
      </div>
    </div>
    
    <!-- New Messages Toast (when paused) -->
    <div v-if="isPaused && missedCount > 0" class="new-msgs-toast" @click="togglePause">
      {{ missedCount }} new messages ↓
    </div>

    <!-- Copy Feedback Toast -->
    <Transition name="fade">
      <div v-if="copyFeedback" class="copy-toast">
        {{ copyFeedback }}
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import JsonViewer from '@/components/common/JsonViewer.vue'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dataStore = useWidgetDataStore()
const scrollContainer = ref<HTMLElement | null>(null)

// State
const isPaused = ref(false)
const filterText = ref('')
const expandAll = ref(false)
const pausedMessages = ref<any[]>([]) // Snapshot when paused
const copyFeedback = ref<string | null>(null)

// Config
const showTimestamp = computed(() => props.config.consoleConfig?.showTimestamp ?? true)

// Data Source
const buffer = computed(() => dataStore.getBuffer(props.config.id))

// Computed Messages
const displayMessages = computed(() => {
  // If paused, use the snapshot. If not, use live buffer.
  const source = isPaused.value ? pausedMessages.value : buffer.value
  
  if (!filterText.value) return source
  
  const term = filterText.value.toLowerCase()
  return source.filter(msg => {
    // Match against subject or value
    const valStr = typeof msg.value === 'object' ? JSON.stringify(msg.value) : String(msg.value)
    const subjectStr = msg.subject || ''
    return valStr.toLowerCase().includes(term) || subjectStr.toLowerCase().includes(term)
  })
})

const missedCount = computed(() => {
  if (!isPaused.value) return 0
  return buffer.value.length - pausedMessages.value.length
})

// Actions
function togglePause() {
  isPaused.value = !isPaused.value
  if (isPaused.value) {
    // Freeze current state
    pausedMessages.value = [...buffer.value]
  } else {
    // Resume: jump to bottom
    scrollToBottom()
  }
}

function clear() {
  dataStore.clearBuffer(props.config.id)
  pausedMessages.value = []
}

function toggleExpand() {
  expandAll.value = !expandAll.value
}

async function copyText(text: string, feedback: string) {
  try {
    // 1. Try Modern API (HTTPS / Localhost)
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
    } 
    // 2. Fallback for HTTP Dev Server (Old Magic)
    else {
      const textArea = document.createElement("textarea")
      textArea.value = text
      
      // Ensure it's not visible but part of DOM
      textArea.style.position = "fixed"
      textArea.style.left = "-9999px"
      textArea.style.top = "0"
      document.body.appendChild(textArea)
      
      textArea.focus()
      textArea.select()
      
      const successful = document.execCommand('copy')
      document.body.removeChild(textArea)
      
      if (!successful) throw new Error('Copy command failed')
    }
    
    showFeedback(feedback)
  } catch (err) {
    console.error('Failed to copy:', err)
    showFeedback('Failed to copy')
  }
}

function showFeedback(msg: string) {
  copyFeedback.value = msg
  setTimeout(() => {
    copyFeedback.value = null
  }, 1500)
}

// Helpers
function formatTime(ts: number) {
  const d = new Date(ts)
  const time = d.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const ms = d.getMilliseconds().toString().padStart(3, '0')
  return `${time}.${ms}`
}

function isObject(val: any) {
  return typeof val === 'object' && val !== null
}

function formatValue(val: any) {
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

function scrollToBottom() {
  nextTick(() => {
    if (scrollContainer.value) {
      scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
    }
  })
}

// Watchers
watch(buffer, () => {
  if (!isPaused.value) {
    scrollToBottom()
  }
}, { deep: true })

onMounted(() => {
  scrollToBottom()
})
</script>

<style scoped>
.console-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  border-radius: 4px;
  overflow: hidden;
  position: relative;
}

.console-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 8px;
  background: var(--panel);
  border-bottom: 1px solid var(--border);
  gap: 8px;
  flex-shrink: 0;
}

.toolbar-left, .toolbar-right {
  display: flex;
  gap: 4px;
}

.toolbar-center {
  flex: 1;
  max-width: 200px;
}

.tool-btn {
  background: transparent;
  border: 1px solid transparent;
  color: var(--muted);
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 14px;
  line-height: 1;
  transition: all 0.2s;
}

.tool-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
}

.tool-btn.is-paused {
  color: var(--color-warning);
  border-color: var(--color-warning);
  background: var(--color-warning-bg);
}

.text-btn {
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
}

.filter-input {
  width: 100%;
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
}

.filter-input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.console-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px;
  font-family: var(--mono);
  color: #c9d1d9;
  scroll-behavior: smooth;
}

/* Log Entry Container */
.log-entry {
  display: flex;
  flex-direction: column;
  padding: 6px 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  gap: 4px;
}

.log-entry:hover {
  background: rgba(255, 255, 255, 0.05);
}

/* Header Row: Time & Subject */
.log-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
  opacity: 0.9;
}

.log-time {
  color: var(--muted);
  font-size: 0.85em;
  white-space: nowrap;
  flex-shrink: 0;
  user-select: none;
}

.log-subject {
  color: var(--color-accent);
  font-size: 0.9em;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.log-subject:hover {
  text-decoration: underline;
  opacity: 1;
}

/* Content Row */
.log-content {
  padding-left: 4px;
  min-width: 0;
}

.content-wrapper {
  word-break: break-all;
  white-space: pre-wrap;
  cursor: pointer;
  display: inline-block; /* Helps with click target sizing */
  width: 100%;
}

.content-wrapper:hover {
  background: rgba(255,255,255,0.03);
  border-radius: 2px;
}

.log-text {
  color: var(--text);
}

.empty-state {
  padding: 20px;
  text-align: center;
  color: var(--muted);
  font-style: italic;
  font-size: 12px;
}

.new-msgs-toast {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--color-accent);
  color: #000;
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0,0,0,0.3);
  animation: slideUp 0.2s ease-out;
  z-index: 10;
}

.copy-toast {
  position: absolute;
  top: 40px;
  right: 16px;
  background: var(--color-success-bg);
  color: var(--color-success);
  border: 1px solid var(--color-success-border);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  pointer-events: none;
  z-index: 20;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@keyframes slideUp {
  from { transform: translate(-50%, 10px); opacity: 0; }
  to { transform: translate(-50%, 0); opacity: 1; }
}

/* Scrollbar styling for console */
.console-body::-webkit-scrollbar {
  width: 8px;
}
.console-body::-webkit-scrollbar-track {
  background: #0d1117;
}
.console-body::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 4px;
}
</style>
