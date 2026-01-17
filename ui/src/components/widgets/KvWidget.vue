<template>
  <div class="kv-widget" :class="{ 'single-value-mode': isSingleValue, 'card-layout': layoutMode === 'card' }">
    <div v-if="loading" class="kv-loading">
      <LoadingState size="small" inline />
    </div>
    
    <div v-else-if="error" class="kv-error">
      <div class="error-icon">⚠️</div>
      <div class="error-text">{{ layoutMode === 'card' ? 'Error' : error }}</div>
    </div>
    
    <div v-else-if="kvValue !== null" class="kv-content">
      
      <!-- CARD LAYOUT (Mobile/Compact) -->
      <template v-if="layoutMode === 'card'">
        
        <!-- Case A: Simple Value (Row Layout) -->
        <div v-if="isSingleValue" class="card-content">
          <div class="card-icon">🗄️</div>
          <div class="card-info">
            <div class="card-title">{{ config.title }}</div>
            <div class="card-value" :style="{ color: valueColor }">
              {{ displayContent }}
            </div>
          </div>
        </div>

        <!-- Case B: Complex/JSON Value (Stacked Layout) -->
        <div v-else class="card-content-stacked">
          <div class="card-header-row">
            <div class="card-icon small">🗄️</div>
            <div class="card-title">{{ config.title }}</div>
          </div>
          <div class="card-json-body">
            <JsonViewer :data="processedValue" />
          </div>
          <div class="card-footer">
             <span class="meta-label">Rev:</span> {{ revision }}
          </div>
        </div>

      </template>

      <!-- STANDARD LAYOUT (Desktop) -->
      <template v-else>
        <div v-if="!isSingleValue" class="kv-header">
          <div class="kv-bucket">{{ resolvedConfig.bucket }}</div>
          <div class="kv-key">{{ resolvedConfig.key }}</div>
        </div>
        
        <div class="kv-value">
          <div 
            v-if="isSingleValue" 
            class="value-display"
            :style="{ color: valueColor }"
          >
            {{ displayContent }}
          </div>
          <div v-else class="kv-value-content">
            <JsonViewer :data="processedValue" />
          </div>
        </div>
        
        <div class="kv-meta">
          <template v-if="isSingleValue">
            <span class="meta-simple">Rev {{ revision }} • {{ lastUpdated }}</span>
          </template>
          <template v-else>
            <span class="meta-item">
              <span class="meta-label">Rev:</span><span class="meta-value">{{ revision }}</span>
            </span>
            <span v-if="lastUpdated" class="meta-item">
              <span class="meta-label">Upd:</span><span class="meta-value">{{ lastUpdated }}</span>
            </span>
          </template>
        </div>
      </template>
    </div>
    
    <div v-else class="kv-empty">
      <div class="empty-icon">🗄️</div>
      <div v-if="layoutMode !== 'card'">Key not found</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { Kvm } from '@nats-io/kv'
import { JSONPath } from 'jsonpath-plus'
import LoadingState from '@/components/common/LoadingState.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import { useThresholds } from '@/composables/useThresholds'
import type { WidgetConfig } from '@/types/dashboard'
import { decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const { evaluateThresholds } = useThresholds()

const kvValue = ref<string | null>(null)
const revision = ref<number>(0)
const lastUpdated = ref<string | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
let watcher: any = null

const resolvedConfig = computed(() => {
  const vars = dashboardStore.currentVariableValues
  return {
    bucket: resolveTemplate(props.config.dataSource.kvBucket, vars),
    key: resolveTemplate(props.config.dataSource.kvKey, vars),
    jsonPath: props.config.jsonPath
  }
})

const processedValue = computed(() => {
  if (kvValue.value === null) return null
  let val: any = kvValue.value
  let isJson = false
  try {
    val = JSON.parse(kvValue.value)
    isJson = true
  } catch { }

  if (resolvedConfig.value.jsonPath && isJson) {
    try {
      const extracted = JSONPath({ 
        path: resolvedConfig.value.jsonPath, 
        json: val, 
        wrap: false 
      })
      if (extracted === undefined) return undefined
      return extracted
    } catch (err) {
      return undefined
    }
  }
  return val
})

const isSingleValue = computed(() => {
  const val = processedValue.value
  return val === null || val === undefined || (typeof val !== 'object')
})

const displayContent = computed(() => {
  const val = processedValue.value
  if (val === undefined) return '<path not found>'
  if (val === null) return ''
  if (typeof val === 'object') return JSON.stringify(val, null, 2)
  return String(val)
})

const valueColor = computed(() => {
  if (!isSingleValue.value) return 'var(--text)'
  const val = processedValue.value
  const rules = props.config.kvConfig?.thresholds || []
  const color = evaluateThresholds(val, rules)
  return color || 'var(--text)'
})

async function loadKvValue() {
  const { bucket, key } = resolvedConfig.value
  if (!bucket || !key || bucket === 'my-bucket') {
    error.value = 'Config required'
    loading.value = false
    return
  }
  
  // Grug say: If not connected, stop here. 
  // Do NOT set error.value, just leave existing data (stale) or loading state.
  if (!natsStore.nc || !natsStore.isConnected) {
    loading.value = false
    return
  }

  try {
    loading.value = true
    error.value = null
    const kvm = new Kvm(natsStore.nc)
    const kv = await kvm.open(bucket)
    const entry = await kv.get(key)
    if (entry) {
      kvValue.value = decodeBytes(entry.value)
      revision.value = entry.revision
      lastUpdated.value = new Date().toLocaleTimeString()
    } else {
      kvValue.value = null
    }
    const iter = await kv.watch({ key })
    watcher = iter
    ;(async () => {
      try {
        for await (const e of iter) {
          if (e.key === key) {
            if (e.operation === 'PUT') {
              const decoder = new TextDecoder()
              kvValue.value = decoder.decode(e.value!)
              revision.value = e.revision
              lastUpdated.value = new Date().toLocaleTimeString()
            } else if (e.operation === 'DEL' || e.operation === 'PURGE') {
              kvValue.value = null
              revision.value = e.revision
              lastUpdated.value = new Date().toLocaleTimeString()
            }
          }
        }
      } catch (err) {}
    })()
    loading.value = false
  } catch (err: any) {
    if (err.message.includes('stream not found')) {
      error.value = `Bucket not found`
    } else {
      error.value = err.message
    }
    loading.value = false
  }
}

function cleanup() {
  if (watcher) { try { watcher.stop() } catch {} watcher = null }
}

onMounted(() => { if (natsStore.isConnected) loadKvValue() })
onUnmounted(cleanup)
watch(resolvedConfig, () => { cleanup(); if (natsStore.isConnected) loadKvValue() }, { deep: true })

watch(() => natsStore.isConnected, (isConnected) => {
  if (isConnected) {
    loadKvValue()
  } else { 
    cleanup()
    loading.value = false
    // Grug say: Keep old value on screen.
  }
})
</script>

<style scoped>
.kv-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
}

/* --- CARD LAYOUT --- */
.kv-widget.card-layout {
  padding: 12px;
}

.card-content {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

/* Stacked Layout for JSON */
.card-content-stacked {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}

.card-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.card-json-body {
  flex: 1;
  background: rgba(0,0,0,0.05);
  border-radius: 4px;
  padding: 8px;
  overflow: auto;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text);
  margin-bottom: 4px;
}

.card-footer {
  font-size: 10px;
  color: var(--muted);
  text-align: right;
}

.card-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: rgba(0,0,0,0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: var(--muted);
  flex-shrink: 0;
}

.card-icon.small {
  width: 24px;
  height: 24px;
  font-size: 14px;
}

.card-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  overflow: hidden;
}

.card-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 2px;
}

.card-value {
  font-size: 16px;
  font-weight: 600;
  font-family: var(--font);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* --- STANDARD LAYOUT --- */
.kv-widget:not(.card-layout) {
  padding: 8px;
}

.kv-widget.single-value-mode:not(.card-layout) {
  justify-content: center;
  align-items: center;
}

.kv-loading, .kv-error, .kv-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  gap: 8px;
}

.kv-error { color: var(--color-error); }
.kv-empty { color: var(--muted); }
.error-icon, .empty-icon { font-size: 32px; }
.error-text { font-size: 13px; line-height: 1.4; }

.kv-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  gap: 8px;
}

.kv-header {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.kv-bucket { font-size: 10px; color: var(--muted); text-transform: uppercase; }
.kv-key { font-size: 12px; font-weight: 600; color: var(--color-accent); font-family: var(--mono); }

.kv-value {
  flex: 1;
  min-height: 0;
  overflow: auto;
  position: relative;
}

.single-value-mode .kv-value {
  display: flex;
  flex-direction: column;
  justify-content: center;
  overflow: hidden;
}

.value-display {
  font-size: clamp(16px, 15cqw, 60px);
  font-weight: 600;
  text-align: center;
  word-break: break-word;
  line-height: 1.2;
  font-family: var(--mono);
  transition: color 0.3s ease;
}

.kv-value-content {
  margin: 0;
  padding: 8px;
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  min-height: 100%;
}

.kv-meta {
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  flex-shrink: 0;
}

.kv-widget:not(.single-value-mode) .kv-meta {
  padding-top: 4px;
  border-top: 1px solid var(--border);
}

.single-value-mode .kv-meta {
  margin-top: 8px;
  justify-content: center;
}

.meta-simple { color: var(--muted); font-family: var(--mono); }
.meta-item { display: flex; gap: 4px; }
.meta-label { color: var(--muted); }
.meta-value { color: var(--text); font-family: var(--mono); }

@container (height < 80px) {
  .kv-meta { display: none; }
}
</style>
