<template>
  <div class="marker-detail-panel" :class="{ 'is-mobile': isMobile }">
    <!-- Header -->
    <div class="panel-header">
      <div class="header-content">
        <span class="marker-icon">📍</span>
        <h3 class="marker-label">{{ label }}</h3>
      </div>
      <button class="close-btn" @click="$emit('close')" title="Close panel">
        ✕
      </button>
    </div>

    <!-- Body: popup fields -->
    <div class="panel-body">
      <div v-if="!popupFields || popupFields.length === 0" class="empty-items">
        <span class="empty-icon">📭</span>
        <span class="empty-text">No popup fields configured</span>
      </div>

      <div v-else class="fields-list">
        <div v-for="field in popupFields" :key="field.path" class="field-row">
          <span class="field-label">{{ field.label }}</span>
          <span class="field-value mono">{{ getFieldValue(field) }}</span>
        </div>
      </div>
    </div>

    <!-- Footer: KV key + revision -->
    <div class="panel-footer">
      <span class="footer-label">Key:</span>
      <span class="footer-value mono">{{ row.key }}</span>
      <span class="footer-sep">·</span>
      <span class="footer-label">Rev:</span>
      <span class="footer-value mono">{{ row.revision }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { KvRow } from '@/composables/useNatsKvWatcher'
import type { DynamicMarkerPopupField } from '@/types/dashboard'
import { JSONPath } from 'jsonpath-plus'
import { formatColumnValue } from '@/utils/format'

const props = defineProps<{
  row: KvRow
  popupFields: DynamicMarkerPopupField[]
  label: string
  isMobile: boolean
}>()

defineEmits<{
  close: []
}>()

function getFieldValue(field: DynamicMarkerPopupField): string {
  let raw: any
  const path = field.path

  // Meta-paths
  if (path === '__key__') raw = props.row.key
  else if (path === '__key_suffix__') raw = props.row.keySuffix
  else if (path === '__revision__') raw = props.row.revision
  else if (path === '__timestamp__') raw = props.row.timestamp
  else {
    try {
      raw = JSONPath({ path, json: props.row.data, wrap: false })
    } catch {
      raw = null
    }
  }

  return formatColumnValue(raw, field.format || 'text')
}
</script>

<style scoped>
/* Desktop: Right sidebar - matches MarkerDetailPanel */
.marker-detail-panel {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 280px;
  background: var(--panel);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  z-index: 10;
  box-shadow: -4px 0 12px rgba(0, 0, 0, 0.2);
}

/* Mobile: Full overlay */
.marker-detail-panel.is-mobile {
  right: 0;
  left: 0;
  top: 0;
  bottom: 0;
  width: 100%;
  border-left: none;
  border-radius: 0;
  box-shadow: none;
}

/* Header */
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.1);
  flex-shrink: 0;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.marker-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.marker-label {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.close-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: var(--muted);
  font-size: 18px;
  width: 32px;
  height: 32px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  flex-shrink: 0;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  color: var(--text);
}

/* Body */
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}

.empty-items {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 16px;
  color: var(--muted);
  text-align: center;
}

.empty-icon {
  font-size: 32px;
  opacity: 0.5;
}

.empty-text {
  font-size: 14px;
}

.fields-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.field-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.08);
}

.field-row:nth-child(even) {
  background: transparent;
}

.field-label {
  font-size: 12px;
  color: var(--muted);
  flex-shrink: 0;
  font-weight: 500;
}

.field-value {
  font-size: 13px;
  color: var(--text);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-value.mono {
  font-family: var(--mono);
}

/* Footer */
.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.footer-label {
  font-size: 11px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.footer-value {
  font-size: 12px;
  color: var(--text);
}

.footer-value.mono {
  font-family: var(--mono);
}

.footer-sep {
  color: var(--muted);
  font-size: 11px;
}
</style>
