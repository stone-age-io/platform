<!-- ui/src/components/widgets/StreamTableWidget.vue -->
<template>
  <div class="streamtable-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Not Configured -->
    <div v-if="!hasSubjects" class="empty-state">
      <div class="empty-icon">📡</div>
      <div v-if="layoutMode !== 'card'">Configure one or more subjects</div>
    </div>

    <!-- Data -->
    <template v-else>
      <!-- Toolbar -->
      <div class="toolbar">
        <button
          class="tool-btn"
          :class="{ 'is-paused': isPaused }"
          :title="isPaused ? 'Resume' : 'Pause'"
          @click="togglePause"
        >
          {{ isPaused ? '▶' : '⏸' }}
        </button>
        <button class="tool-btn" title="Clear" @click="clear">🚫</button>
        <input
          v-model="searchQuery"
          type="text"
          class="search-input"
          placeholder="Filter rows..."
        />
      </div>

      <!-- Table -->
      <div class="data-container">
        <ResponsiveList
          :items="filteredRows"
          :columns="tableColumns"
          :clickable="true"
          @row-click="openDetail"
        >
          <template v-for="col in cfg.columns || []" :key="col.id" #[`cell-${col.id}`]="{ item, value }">
            <span
              class="text-xs truncate block max-w-[240px]"
              :class="cellClass(item, col)"
              :title="isEmpty(value) ? '' : String(value)"
            >
              {{ displayValue(value) }}
            </span>
          </template>

          <template v-for="(col, idx) in cfg.columns || []" :key="col.id" #[`card-${col.id}`]="{ item, value }">
            <span
              v-if="idx === 0"
              class="text-sm font-bold text-primary truncate"
              :class="cellClass(item, col)"
            >{{ displayValue(value) }}</span>
            <span
              v-else
              class="text-xs font-medium text-base-content/80 truncate"
              :class="cellClass(item, col)"
            >{{ displayValue(value) }}</span>
          </template>

          <template #empty>
            <div class="text-center py-8 opacity-50 text-xs italic">
              {{ buffer.length > 0 ? 'No matching rows' : 'Waiting for messages…' }}
            </div>
          </template>
        </ResponsiveList>
      </div>

      <!-- Footer -->
      <div class="widget-footer">
        <span class="text-[10px] opacity-50">
          <template v-if="isPaused">
            paused · {{ filteredRows.length }} rows
            <span v-if="missedCount > 0" class="missed">(+{{ missedCount }} missed)</span>
          </template>
          <template v-else>
            live · {{ filteredRows.length }} row{{ filteredRows.length !== 1 ? 's' : '' }}
          </template>
        </span>
        <button
          v-if="filteredRows.length > 0"
          class="csv-btn"
          title="Export visible rows as CSV"
          @click="downloadCsv"
        >
          CSV
        </button>
      </div>
    </template>

    <!-- Detail Modal -->
    <Teleport to="body">
      <div v-if="selectedRow" class="modal-overlay" @click.self="selectedRow = null">
        <div class="detail-modal">
          <div class="detail-header">
            <h4>{{ selectedRow.__raw__.subject || '(no subject)' }}</h4>
            <button class="close-btn" @click="selectedRow = null">✕</button>
          </div>
          <div class="detail-body">
            <div class="detail-meta">
              <span class="meta-item">Subject: <code>{{ selectedRow.__raw__.subject }}</code></span>
              <span class="meta-item">Received: {{ new Date(selectedRow.__raw__.timestamp).toLocaleString() }}</span>
            </div>
            <JsonViewer :data="selectedRow.__raw__.value" />
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { formatColumnValue } from '@/utils/format'
import { JSONPath } from 'jsonpath-plus'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import type { WidgetConfig, TableColumn } from '@/types/dashboard'
import type { BufferedMessage } from '@/stores/widgetData'
import {
  cellClass as cellClassFor,
  compareMixed,
  displayValue,
  exportCsv,
  isEmpty,
} from '@/utils/tableColumns'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dataStore = useWidgetDataStore()

const searchQuery = ref('')
const selectedRow = ref<any>(null)
const isPaused = ref(false)
const pausedSnapshot = ref<BufferedMessage[]>([])

const cfg = computed(() => props.config.streamtableConfig || { columns: [] })

const subjectsConfigured = computed(() => {
  const ds = props.config.dataSource
  return (ds?.subjects?.length ?? 0) > 0 || !!ds?.subject
})
const hasSubjects = subjectsConfigured

const buffer = computed<BufferedMessage[]>(() => dataStore.getBuffer(props.config.id))

const tableColumns = computed<Column<any>[]>(() =>
  (cfg.value.columns || []).map(col => ({
    key: col.id,
    label: col.label,
    mobileLabel: col.label,
  }))
)

const SUBJECT_TOKEN_RE = /^__subject\.(\d+)__$/

function extractValue(msg: BufferedMessage, col: TableColumn): any {
  const path = col.path
  if (path === '__subject__')   return msg.subject
  if (path === '__timestamp__') return msg.timestamp
  const m = SUBJECT_TOKEN_RE.exec(path)
  if (m) return msg.subject?.split('.')[Number(m[1])]
  try {
    return JSONPath({ path, json: msg.value, wrap: false })
  } catch {
    return null
  }
}

const tableRows = computed(() => {
  const columns = cfg.value.columns || []
  const src = isPaused.value ? pausedSnapshot.value : buffer.value
  // Newest first — buffer pushes append, so walk in reverse.
  const result: any[] = []
  for (let i = src.length - 1; i >= 0; i--) {
    const msg = src[i]
    const item: any = {
      id: `${msg.timestamp}-${i}`,
      __raw__: msg,
      __values: {} as Record<string, any>,
    }
    for (const col of columns) {
      const raw = extractValue(msg, col)
      item.__values[col.id] = raw
      item[col.id] = formatColumnValue(raw, col.format, col.formatOptions)
    }
    result.push(item)
  }

  const sortCol = cfg.value.defaultSortColumn
  if (sortCol) {
    const colDef = columns.find(c => c.id === sortCol)
    if (colDef) {
      const dir = cfg.value.defaultSortDirection || 'desc'
      result.sort((a, b) => {
        const cmp = compareMixed(a.__values[colDef.id], b.__values[colDef.id])
        return dir === 'asc' ? cmp : -cmp
      })
    }
  }
  return result
})

const filteredRows = computed(() => {
  if (!searchQuery.value.trim()) return tableRows.value
  const q = searchQuery.value.toLowerCase()
  return tableRows.value.filter(row => {
    for (const col of cfg.value.columns || []) {
      const val = row[col.id]
      if (val !== null && val !== undefined && String(val).toLowerCase().includes(q)) return true
    }
    return false
  })
})

const missedCount = computed(() =>
  isPaused.value ? Math.max(0, buffer.value.length - pausedSnapshot.value.length) : 0
)

function togglePause() {
  isPaused.value = !isPaused.value
  if (isPaused.value) {
    pausedSnapshot.value = [...buffer.value]
  } else {
    pausedSnapshot.value = []
  }
}

function clear() {
  dataStore.clearBuffer(props.config.id)
  pausedSnapshot.value = []
}

function openDetail(item: any) {
  selectedRow.value = item
}

function cellClass(item: any, col: TableColumn): string {
  return cellClassFor(item.__values?.[col.id], col)
}

function downloadCsv() {
  exportCsv({
    columns: cfg.value.columns || [],
    rows: filteredRows.value,
    filename: props.config.title || 'stream-table',
  })
}

function handleRefresh() {
  // Grug say: refresh means show new data, not stay stuck on old snapshot.
  isPaused.value = false
  pausedSnapshot.value = []
}

onMounted(() => {
  window.addEventListener('dashboard:refresh', handleRefresh)
})

onUnmounted(() => {
  window.removeEventListener('dashboard:refresh', handleRefresh)
})
</script>

<style scoped>
.streamtable-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.streamtable-widget.card-layout {
  padding: 8px;
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 16px;
  gap: 8px;
  color: oklch(var(--bc) / 0.4);
}
.empty-icon { font-size: 32px; }

.toolbar {
  flex-shrink: 0;
  display: flex;
  gap: 6px;
  align-items: center;
  padding: 6px 8px;
  border-bottom: 1px solid oklch(var(--b3));
}

.tool-btn {
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  color: oklch(var(--bc) / 0.7);
  border-radius: 4px;
  cursor: pointer;
  padding: 2px 8px;
  font-size: 12px;
  flex-shrink: 0;
  transition: all 0.15s;
}

.tool-btn:hover {
  border-color: oklch(var(--a));
  color: oklch(var(--a));
}

.tool-btn.is-paused {
  border-color: oklch(var(--wa));
  color: oklch(var(--wa));
}

.search-input {
  flex: 1;
  padding: 4px 8px;
  border: 1px solid oklch(var(--b3));
  border-radius: 4px;
  background: oklch(var(--b1));
  color: oklch(var(--bc));
  font-size: 12px;
  outline: none;
}

.search-input:focus { border-color: oklch(var(--a)); }

.data-container {
  flex: 1;
  overflow: auto;
}

.widget-footer {
  flex-shrink: 0;
  padding: 4px 8px;
  border-top: 1px solid oklch(var(--b3));
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  background: oklch(var(--b2) / 0.5);
}

.missed { color: oklch(var(--wa)); }

.csv-btn {
  font-size: 10px;
  letter-spacing: 0.05em;
  padding: 2px 8px;
  border: 1px solid oklch(var(--b3));
  border-radius: 3px;
  background: oklch(var(--b1));
  color: oklch(var(--bc) / 0.7);
  cursor: pointer;
  transition: all 0.15s;
}

.csv-btn:hover {
  border-color: oklch(var(--a));
  color: oklch(var(--a));
}

/* Conditional formatting (applies to both desktop cells and mobile cards) */
.cell-style-success { color: oklch(var(--su)); font-weight: 600; }
.cell-style-warning { color: oklch(var(--wa)); font-weight: 600; }
.cell-style-error   { color: oklch(var(--er)); font-weight: 600; }
.cell-style-info    { color: oklch(var(--in)); font-weight: 600; }

/* Detail modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  backdrop-filter: blur(2px);
}

.detail-modal {
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid oklch(var(--b3));
}

.detail-header h4 {
  margin: 0;
  font-size: 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  color: oklch(var(--bc) / 0.5);
  font-size: 20px;
  cursor: pointer;
  flex-shrink: 0;
}

.close-btn:hover { color: oklch(var(--bc)); }

.detail-body {
  padding: 16px;
  overflow-y: auto;
}

.detail-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 12px;
  font-size: 12px;
  color: oklch(var(--bc) / 0.6);
}

.detail-meta code {
  font-size: 11px;
  background: oklch(var(--b2));
  padding: 1px 4px;
  border-radius: 3px;
}
</style>
