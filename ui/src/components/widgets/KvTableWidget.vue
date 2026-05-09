<!-- ui/src/components/widgets/KvTableWidget.vue -->
<template>
  <div class="kvtable-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Loading -->
    <div v-if="loading && tableRows.length === 0" class="loading-overlay">
      <span class="loading loading-spinner"></span>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="error-state">
      <span class="text-error">{{ error }}</span>
    </div>

    <!-- Not Configured -->
    <div v-else-if="!cfg.kvBucket" class="empty-state">
      <div class="empty-icon">📋</div>
      <div v-if="layoutMode !== 'card'">Configure bucket &amp; key pattern</div>
    </div>

    <!-- Data -->
    <template v-else>
      <!-- Search -->
      <div v-if="tableRows.length > 0" class="search-bar">
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
              class="text-xs truncate block max-w-[200px]"
              :class="cellClass(item, col)"
              :title="isEmpty(value) ? '' : String(value)"
            >
              {{ displayValue(value) }}
            </span>
          </template>

          <!-- Mobile card: first column = bold identity header, rest = value only (ResponsiveList adds labels) -->
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
              {{ rows.size > 0 ? 'No matching rows' : 'No entries' }}
            </div>
          </template>
        </ResponsiveList>
      </div>

      <!-- Footer -->
      <div class="widget-footer">
        <span class="text-[10px] opacity-50">
          <template v-if="isCapped">
            showing {{ filteredRows.length }} of {{ totalRowCount.toLocaleString() }} (cap {{ maxRows }})
          </template>
          <template v-else>
            {{ filteredRows.length }}{{ filteredRows.length !== cappedRows.length ? `/${cappedRows.length}` : '' }}
            row{{ filteredRows.length !== 1 ? 's' : '' }}
          </template>
        </span>
        <button
          v-if="filteredRows.length > 0"
          class="csv-btn"
          title="Export visible rows as CSV"
          @click="exportCsv"
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
            <h4>{{ selectedRow.id }}</h4>
            <button class="close-btn" @click="selectedRow = null">✕</button>
          </div>
          <div class="detail-body">
            <div class="detail-meta">
              <span class="meta-item">Key: <code>{{ selectedRow.__raw__.key }}</code></span>
              <span class="meta-item">Rev: {{ selectedRow.__raw__.revision }}</span>
              <span class="meta-item">Updated: {{ selectedRow.__raw__.timestamp?.toLocaleString() }}</span>
            </div>
            <JsonViewer :data="selectedRow.__raw__.data" />
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useNatsKvWatcher, type KvRow } from '@/composables/useNatsKvWatcher'
import { resolveTemplate } from '@/utils/variables'
import { formatColumnValue } from '@/utils/format'
import { JSONPath } from 'jsonpath-plus'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import type { WidgetConfig, KvTableColumn, ConditionalRule } from '@/types/dashboard'

const NULL_DISPLAY = '—'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dashboardStore = useDashboardStore()
const searchQuery = ref('')
const selectedRow = ref<any>(null)

const cfg = computed(() => props.config.kvtableConfig || {
  kvBucket: '',
  keyPattern: '>',
  columns: [],
})

const resolvedBucket = computed(() =>
  resolveTemplate(cfg.value.kvBucket, dashboardStore.currentVariableValues)
)

const resolvedPattern = computed(() =>
  resolveTemplate(cfg.value.keyPattern, dashboardStore.currentVariableValues)
)

const { rows, loading, error } = useNatsKvWatcher(
  () => resolvedBucket.value,
  () => resolvedPattern.value
)

// Build ResponsiveList columns from config
const tableColumns = computed<Column<any>[]>(() => {
  return (cfg.value.columns || []).map((col: KvTableColumn) => ({
    key: col.id,
    label: col.label,
    mobileLabel: col.label
  }))
})

// Extract column values from each KvRow
function extractValue(row: KvRow, col: KvTableColumn): any {
  const path = col.path
  // Meta-paths
  if (path === '__key__') return row.key
  if (path === '__key_suffix__') return row.keySuffix
  if (path === '__revision__') return row.revision
  if (path === '__timestamp__') return row.timestamp

  // JSONPath extraction
  try {
    return JSONPath({ path, json: row.data, wrap: false })
  } catch {
    return null
  }
}

// Transform KvRow Map into flat table rows
const tableRows = computed(() => {
  const columns = cfg.value.columns || []
  const result: any[] = []

  for (const [key, row] of rows.value) {
    const item: any = { id: key, __raw__: row, __values: {} as Record<string, any> }
    for (const col of columns) {
      const raw = extractValue(row, col)
      item.__values[col.id] = raw
      item[col.id] = formatColumnValue(raw, col.format, col.formatOptions)
    }
    result.push(item)
  }

  // Sort
  const sortCol = cfg.value.defaultSortColumn
  const sortDir = cfg.value.defaultSortDirection || 'desc'
  if (sortCol) {
    const colDef = columns.find(c => c.id === sortCol)
    if (colDef) {
      result.sort((a, b) => {
        const cmp = compareMixed(a.__values[colDef.id], b.__values[colDef.id])
        return sortDir === 'asc' ? cmp : -cmp
      })
    }
  }

  return result
})

const totalRowCount = computed(() => tableRows.value.length)
const maxRows = computed(() => {
  const m = cfg.value.maxRows
  return m && m > 0 ? m : Infinity
})
const cappedRows = computed(() =>
  totalRowCount.value > maxRows.value ? tableRows.value.slice(0, maxRows.value) : tableRows.value
)
const isCapped = computed(() => totalRowCount.value > maxRows.value)

// Client-side search filter
const filteredRows = computed(() => {
  if (!searchQuery.value.trim()) return cappedRows.value
  const q = searchQuery.value.toLowerCase()
  return cappedRows.value.filter(row => {
    for (const col of (cfg.value.columns || [])) {
      const val = row[col.id]
      if (val !== null && val !== undefined && String(val).toLowerCase().includes(q)) return true
    }
    return false
  })
})

function compareMixed(a: any, b: any): number {
  if (a == null && b == null) return 0
  if (a == null) return -1
  if (b == null) return 1
  if (typeof a === 'number' && typeof b === 'number') return a - b
  if (a instanceof Date && b instanceof Date) return a.getTime() - b.getTime()
  return String(a).localeCompare(String(b))
}

function openDetail(item: any) {
  selectedRow.value = item
}

function isEmpty(v: any): boolean {
  return v === null || v === undefined || v === ''
}

function displayValue(v: any): string {
  return isEmpty(v) ? NULL_DISPLAY : String(v)
}

function evalRule(raw: any, rule: ConditionalRule): boolean {
  if (raw === null || raw === undefined) return false
  if (rule.op === 'eq') return String(raw) === String(rule.value)
  if (rule.op === 'contains') {
    return String(raw).toLowerCase().includes(String(rule.value).toLowerCase())
  }
  const a = Number(raw)
  const b = Number(rule.value)
  if (!Number.isFinite(a) || !Number.isFinite(b)) return false
  switch (rule.op) {
    case 'gt': return a > b
    case 'lt': return a < b
    case 'gte': return a >= b
    case 'lte': return a <= b
  }
  return false
}

function cellClass(item: any, col: KvTableColumn): string {
  if (!col.rules || col.rules.length === 0) return ''
  const raw = item.__values?.[col.id]
  for (const rule of col.rules) {
    if (evalRule(raw, rule)) return `cell-style-${rule.style}`
  }
  return ''
}

function csvField(v: any): string {
  if (isEmpty(v)) return ''
  const s = String(v)
  if (/[",\n\r]/.test(s)) return '"' + s.replace(/"/g, '""') + '"'
  return s
}

function exportCsv() {
  const cols = cfg.value.columns || []
  if (cols.length === 0) return
  const header = cols.map(c => csvField(c.label || c.id)).join(',')
  const lines = filteredRows.value.map(row =>
    cols.map(c => csvField(row[c.id])).join(',')
  )
  const csv = [header, ...lines].join('\r\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const name = (props.config.title || 'kv-table').replace(/[^a-z0-9._-]+/gi, '_')
  a.download = `${name}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.kvtable-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.kvtable-widget.card-layout {
  padding: 8px;
}

.loading-overlay, .error-state, .empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 16px;
  gap: 8px;
}

.error-state { color: oklch(var(--er)); font-size: 13px; }
.empty-state { color: oklch(var(--bc) / 0.4); }
.empty-icon { font-size: 32px; }

.search-bar {
  flex-shrink: 0;
  padding: 6px 8px;
  border-bottom: 1px solid oklch(var(--b3));
}

.search-input {
  width: 100%;
  padding: 4px 8px;
  border: 1px solid oklch(var(--b3));
  border-radius: 4px;
  background: oklch(var(--b1));
  color: oklch(var(--bc));
  font-size: 12px;
  outline: none;
}

.search-input:focus {
  border-color: oklch(var(--a));
}

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

/* Detail Modal */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
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

.close-btn:hover {
  color: oklch(var(--bc));
}

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
