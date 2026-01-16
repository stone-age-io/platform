<template>
  <div class="markdown-widget" :class="{ 'card-layout': layoutMode === 'card', 'is-editing': isEditing }">
    
    <!-- Edit Toggle (Only visible when Unlocked) -->
    <div v-if="!dashboardStore.isLocked" class="md-toolbar">
      <button 
        class="mode-btn" 
        @click="toggleEditMode"
        :title="isEditing ? 'Save & Preview' : 'Edit Markdown'"
      >
        {{ isEditing ? '✓ Done' : '✎ Edit' }}
      </button>
    </div>

    <!-- Edit Mode: Textarea -->
    <div v-if="isEditing" class="editor-container">
      <textarea 
        v-model="localContent" 
        class="markdown-editor" 
        placeholder="# Type markdown here..."
        @keydown.stop
        @blur="saveChanges" 
      ></textarea>
      <div class="editor-hint">
        Supports Markdown & Variables (e.g. <code v-pre>{{device_id}}</code>). Blur or click Done to save.
      </div>
    </div>

    <!-- View Mode: Rendered HTML -->
    <div v-else class="markdown-body" v-html="renderedContent"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onUnmounted, watch } from 'vue'
import { marked } from 'marked'
import { JSONPath } from 'jsonpath-plus'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useNatsStore } from '@/stores/nats'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import { Kvm } from '@nats-io/kv'
import { resolveTemplate } from '@/utils/variables'
import { decodeBytes } from '@/utils/encoding'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const natsStore = useNatsStore()
const { updateWidgetConfiguration } = useWidgetOperations()

// State for In-Place Editing
const isEditing = ref(false)
const localContent = ref('')

// KV State
const kvData = ref<any>(null)
let kvWatcher: any = null

// --- Data Logic ---

// 1. Subscription Data (NATS Core/JetStream)
const buffer = computed(() => {
  if (props.config.dataSource.type !== 'subscription') return []
  return dataStore.getBuffer(props.config.id)
})

const latestSubData = computed(() => {
  if (buffer.value.length === 0) return null
  return buffer.value[buffer.value.length - 1].value
})

// 2. KV Data (Direct Watch)
async function setupKvWatcher() {
  if (props.config.dataSource.type !== 'kv') return
  if (!natsStore.nc || !natsStore.isConnected) return

  const bucket = resolveTemplate(props.config.dataSource.kvBucket, dashboardStore.currentVariableValues)
  const key = resolveTemplate(props.config.dataSource.kvKey, dashboardStore.currentVariableValues)

  if (!bucket || !key) return

  try {
    const kvm = new Kvm(natsStore.nc)
    const kv = await kvm.open(bucket)
    
    // Initial Get
    try {
      const entry = await kv.get(key)
      if (entry) {
        processKvEntry(entry.value)
      } else {
        kvData.value = null // Key missing
      }
    } catch {
      kvData.value = null
    }

    // Watch
    if (kvWatcher) { try { kvWatcher.stop() } catch {} }
    kvWatcher = await kv.watch({ key })
    
    ;(async () => {
      for await (const e of kvWatcher) {
        if (e.key === key) {
          if (e.operation === 'PUT') {
            processKvEntry(e.value!)
          } else if (e.operation === 'DEL' || e.operation === 'PURGE') {
            kvData.value = null
          }
        }
      }
    })()
  } catch (err) {
    console.warn('[Markdown] KV Error:', err)
  }
}

function processKvEntry(data: Uint8Array) {
  try {
    const str = decodeBytes(data)
    let val = JSON.parse(str)
    
    // Apply JSONPath if configured
    if (props.config.jsonPath) {
      try {
        const extracted = JSONPath({ 
          path: props.config.jsonPath, 
          json: val, 
          wrap: false 
        })
        if (extracted !== undefined) val = extracted
      } catch (err) {
        console.warn('[Markdown] JSONPath Error:', err)
      }
    }
    
    kvData.value = val
  } catch {
    // If not JSON, return as simple value object
    kvData.value = decodeBytes(data)
  }
}

// 3. Flatten Helper
function flatten(obj: any, prefix = '', res: Record<string, any> = {}) {
  for (const key in obj) {
    const val = obj[key]
    const newKey = prefix ? `${prefix}.${key}` : key
    if (typeof val === 'object' && val !== null && !Array.isArray(val)) {
      flatten(val, newKey, res)
    } else {
      res[newKey] = val
    }
  }
  return res
}

// 4. Combined Variable Context
const variableContext = computed(() => {
  const globalVars = dashboardStore.currentVariableValues
  
  // Decide source: Subscription vs KV
  let payload = null
  if (props.config.dataSource.type === 'kv') {
    payload = kvData.value
  } else {
    payload = latestSubData.value
  }

  // If no payload, just return globals
  if (payload === null || payload === undefined) return globalVars

  // Flatten logic
  let payloadVars: Record<string, any> = {}
  
  if (typeof payload === 'object' && payload !== null) {
    payloadVars = flatten(payload)
  } else {
    // Primitive value (string, number, boolean)
    payloadVars = { value: payload }
  }

  // Merge: Payload overrides global if collision occurs
  return {
    ...globalVars,
    ...payloadVars
  }
})

// --- Rendering ---

const renderedContent = computed(() => {
  // If editing, we still render what's in config, not localContent (until saved)
  const raw = props.config.markdownConfig?.content || ''
  
  // Resolve using merged context
  const stringContext: Record<string, string> = {}
  for (const [k, v] of Object.entries(variableContext.value)) {
    stringContext[k] = String(v)
  }

  const resolved = resolveTemplate(raw, stringContext)
  return marked.parse(resolved)
})

// --- Edit Mode Logic ---

function toggleEditMode() {
  if (isEditing.value) {
    // Save on exit
    saveChanges()
    isEditing.value = false
  } else {
    // Enter edit mode
    // Initialize local content from props
    localContent.value = props.config.markdownConfig?.content || ''
    isEditing.value = true
  }
}

function saveChanges() {
  // Only save if changed
  if (localContent.value !== props.config.markdownConfig?.content) {
    updateWidgetConfiguration(props.config.id, {
      markdownConfig: {
        content: localContent.value
      }
    })
  }
}

// Lifecycle
watch(() => [props.config.dataSource, dashboardStore.currentVariableValues, natsStore.isConnected], () => {
  if (props.config.dataSource.type === 'kv') {
    setupKvWatcher()
  } else {
    if (kvWatcher) { try { kvWatcher.stop() } catch {} kvWatcher = null }
    // We are definitely not in KV mode here, so safe to clear
    kvData.value = null
  }
}, { deep: true, immediate: true })

onUnmounted(() => {
  if (kvWatcher) { try { kvWatcher.stop() } catch {} }
})
</script>

<style scoped>
.markdown-widget {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  background: var(--widget-bg);
  position: relative;
}

.markdown-widget.card-layout {
  padding: 8px;
}

/* Toolbar for Edit Button */
.md-toolbar {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
  opacity: 0;
  transition: opacity 0.2s;
}

/* Show toolbar on hover OR if we are currently editing */
.markdown-widget:hover .md-toolbar,
.markdown-widget.is-editing .md-toolbar {
  opacity: 1;
}

.mode-btn {
  background: var(--panel);
  border: 1px solid var(--border);
  color: var(--color-accent);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0,0,0,0.2);
  transition: all 0.2s;
}

.mode-btn:hover {
  background: var(--color-accent);
  color: white;
  border-color: var(--color-accent);
}

/* Editor Styles */
.editor-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 8px;
  min-height: 0;
}

.markdown-editor {
  flex: 1;
  width: 100%;
  resize: none;
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 12px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.5;
}

.markdown-editor:focus {
  outline: none;
  border-color: var(--color-accent);
}

.editor-hint {
  font-size: 11px;
  color: var(--muted);
  margin-top: 4px;
  text-align: right;
}

.editor-hint code {
  background: rgba(255,255,255,0.1);
  padding: 2px 4px;
  border-radius: 3px;
}

/* Viewer Styles */
.markdown-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  color: var(--text);
  font-size: 14px;
  line-height: 1.6;
}

:deep(h1), :deep(h2), :deep(h3), :deep(h4) {
  margin-top: 0.5em;
  margin-bottom: 0.5em;
  font-weight: 600;
  color: var(--text);
  line-height: 1.25;
}

:deep(h1) { font-size: 1.5em; border-bottom: 1px solid var(--border); padding-bottom: 0.3em; }
:deep(h2) { font-size: 1.3em; border-bottom: 1px solid var(--border); padding-bottom: 0.3em; }
:deep(h3) { font-size: 1.1em; }
:deep(h4) { font-size: 1em; }

:deep(p) { margin-bottom: 1em; }
:deep(p:last-child) { margin-bottom: 0; }

:deep(a) { color: var(--color-accent); text-decoration: none; }
:deep(a:hover) { text-decoration: underline; }

:deep(code) {
  background: rgba(255, 255, 255, 0.1);
  padding: 0.2em 0.4em;
  border-radius: 3px;
  font-family: var(--mono);
  font-size: 0.9em;
}

:deep(pre) {
  background: rgba(0, 0, 0, 0.2);
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
  margin-bottom: 1em;
}

:deep(pre code) {
  background: none;
  padding: 0;
  color: inherit;
}

:deep(ul), :deep(ol) {
  padding-left: 1.5em;
  margin-bottom: 1em;
}

:deep(blockquote) {
  border-left: 4px solid var(--border);
  padding-left: 1em;
  color: var(--muted);
  margin-left: 0;
  margin-right: 0;
}

:deep(img) {
  max-width: 100%;
  border-radius: 4px;
}

:deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 1em;
}

:deep(th), :deep(td) {
  border: 1px solid var(--border);
  padding: 6px 12px;
  text-align: left;
}

:deep(th) {
  background: rgba(255, 255, 255, 0.05);
  font-weight: 600;
}
</style>
