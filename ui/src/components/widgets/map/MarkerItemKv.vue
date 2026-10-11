<template>
  <div class="marker-item-kv">
    <span class="kv-label">{{ item.label }}</span>
    <div class="kv-value-row">
      <span class="kv-value" :class="{ 'is-empty': isEmpty }">
        {{ displayValue }}
      </span>
    </div>
    <div v-if="error" class="kv-error">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useKvKey } from '@/composables/useKvKey'
import { resolveTemplate } from '@/utils/variables'
import { JSONPath } from 'jsonpath-plus'
import type { MapMarkerItem } from '@/types/dashboard'

/**
 * Marker Item: KV Display
 *
 * Watches a NATS KV bucket/key and displays the value.
 * Supports JSONPath extraction.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const dashboardStore = useDashboardStore()

const { entry, status, error } = useKvKey(
  () => resolveTemplate(props.item.kvConfig?.kvBucket ?? '', dashboardStore.currentVariableValues),
  () => resolveTemplate(props.item.kvConfig?.kvKey ?? '', dashboardStore.currentVariableValues),
)

const value = computed(() => {
  if (!entry.value) return null
  const parsed = entry.value.value
  const path = props.item.kvConfig?.jsonPath
  if (!path || typeof parsed !== 'object') return parsed
  try {
    const extracted = JSONPath({ path, json: parsed as object, wrap: false })
    return extracted !== undefined ? extracted : parsed
  } catch {
    return parsed
  }
})

const isEmpty = computed(() => status.value === 'missing')

const displayValue = computed(() => {
  if (error.value) return '—'
  if (isEmpty.value) return '<empty>'
  if (value.value === null) return '...'

  // Format objects as JSON
  if (typeof value.value === 'object') {
    return JSON.stringify(value.value)
  }
  return String(value.value)
})
</script>

<style scoped>
.marker-item-kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.kv-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
  flex-shrink: 0;
}

.kv-value-row {
  display: flex;
  align-items: baseline;
  gap: 3px;
  min-width: 0;
}

.kv-value {
  font-size: 14px;
  font-weight: 600;
  font-family: var(--mono);
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kv-value.is-empty {
  color: var(--muted);
  font-style: italic;
  font-size: 12px;
}

.kv-error {
  font-size: 10px;
  color: var(--color-error);
  padding: 2px 6px;
  background: var(--color-error-bg);
  border-radius: 3px;
}
</style>
