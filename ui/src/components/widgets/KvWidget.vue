<template>
  <div class="kv-widget" :class="{ 'single-value-mode': isSingleValue, 'card-layout': layoutMode === 'card' }">
    <WidgetStateOverlay
      v-if="loading"
      state="loading"
      :compact="layoutMode === 'card'"
    />

    <WidgetStateOverlay
      v-else-if="error"
      state="error"
      :message="error"
      :compact="layoutMode === 'card'"
    />
    
    <div v-else-if="hasValue" class="kv-content">
      
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
    
    <WidgetStateOverlay
      v-else
      state="empty"
      icon="🗄️"
      message="Key not found"
      :compact="layoutMode === 'card'"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { JSONPath } from 'jsonpath-plus'
import JsonViewer from '@/components/common/JsonViewer.vue'
import WidgetStateOverlay from '@/components/dashboard/WidgetStateOverlay.vue'
import { useThresholds } from '@/composables/useThresholds'
import { useNatsKvWatcher } from '@/composables/useNatsKvWatcher'
import type { WidgetConfig } from '@/types/dashboard'
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

const resolvedConfig = computed(() => {
  const vars = dashboardStore.currentVariableValues
  return {
    bucket: resolveTemplate(props.config.dataSource.kvBucket, vars),
    key: resolveTemplate(props.config.dataSource.kvKey, vars),
    jsonPath: props.config.jsonPath
  }
})

// The watcher handles lifecycle, dashboard:refresh, reconnects, and variable
// changes (it resolves templates internally, so pass the raw config values).
const { rows, loading, error: watchError } = useNatsKvWatcher(
  () => props.config.dataSource.kvBucket || '',
  () => props.config.dataSource.kvKey || ''
)

// Latest entry for the watched key. Kept (stale) across disconnects;
// cleared only when the key is deleted or missing while connected.
const kvData = ref<any>(null)
const wasJson = ref(false)
const hasValue = ref(false)
const revision = ref<number>(0)
const lastUpdated = ref<string | null>(null)

watch(rows, (map) => {
  const row = map.values().next().value
  if (row) {
    const data: any = row.data
    if (data && typeof data === 'object' && '__value__' in data) {
      // Non-JSON value — the watcher wraps the raw string
      kvData.value = data.__value__
      wasJson.value = false
    } else {
      kvData.value = data
      wasJson.value = true
    }
    hasValue.value = true
    revision.value = row.revision
    lastUpdated.value = new Date().toLocaleTimeString()
  } else if (natsStore.isConnected) {
    hasValue.value = false
    kvData.value = null
  }
})

const error = computed(() => {
  const { bucket, key } = resolvedConfig.value
  if (!bucket || !key || bucket === 'my-bucket') return 'Config required'
  return watchError.value
})

const processedValue = computed(() => {
  if (!hasValue.value) return null
  if (resolvedConfig.value.jsonPath && wasJson.value) {
    try {
      return JSONPath({
        path: resolvedConfig.value.jsonPath,
        json: kvData.value,
        wrap: false
      })
    } catch (err) {
      return undefined
    }
  }
  return kvData.value
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
