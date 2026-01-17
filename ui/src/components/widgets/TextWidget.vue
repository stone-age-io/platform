<template>
  <div class="text-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Card Layout -->
    <template v-if="layoutMode === 'card'">
      <div class="card-content">
        <div class="card-icon">
          <span>📊</span> 
        </div>
        
        <div class="card-info">
          <div class="card-title">{{ config.title }}</div>
          <div class="card-value" :style="valueStyle">
            {{ displayValue }}
          </div>
        </div>
      </div>
    </template>

    <!-- Standard Layout -->
    <template v-else>
      <div 
        class="value-display"
        :style="valueStyle"
        :title="resolvedSubject ? `Source: ${resolvedSubject}` : ''"
      >
        {{ displayValue }}
      </div>
      <div v-if="showTimestamp" class="timestamp">
        {{ timestamp }}
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
import { useThresholds } from '@/composables/useThresholds'
import type { WidgetConfig } from '@/types/dashboard'
import { resolveTemplate } from '@/utils/variables'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dataStore = useWidgetDataStore()
const dashboardStore = useDashboardStore()
const { evaluateThresholds } = useThresholds()

const fontSize = computed(() => props.config.textConfig?.fontSize || 24)
const configColor = computed(() => props.config.textConfig?.color)
const format = computed(() => props.config.textConfig?.format)
const thresholds = computed(() => props.config.textConfig?.thresholds || [])

const resolvedSubject = computed(() => {
  return resolveTemplate(props.config.dataSource.subject, dashboardStore.currentVariableValues)
})

const latestMessage = computed(() => {
  const buffer = dataStore.getBuffer(props.config.id)
  if (buffer.length === 0) return null
  return buffer[buffer.length - 1]
})

const latestValue = computed(() => latestMessage.value?.value)

const effectiveColor = computed(() => {
  const val = latestValue.value
  const thresholdColor = evaluateThresholds(val, thresholds.value)
  if (thresholdColor) return thresholdColor
  if (configColor.value) return configColor.value
  return null
})

const displayValue = computed(() => {
  const value = latestValue.value
  if (value === null || value === undefined) return '...'
  if (format.value) {
    try { return format.value.replace('{value}', String(value)) } catch { return String(value) }
  }
  if (typeof value === 'object') return JSON.stringify(value).substring(0, 20) + '...'
  return String(value)
})

const timestamp = computed(() => {
  if (!latestMessage.value) return ''
  const date = new Date(latestMessage.value.timestamp)
  return date.toLocaleTimeString()
})

const showTimestamp = computed(() => latestMessage.value !== null)

const valueStyle = computed(() => {
  if (props.layoutMode === 'card') {
    return {
      color: effectiveColor.value || 'var(--text-secondary)',
      transition: 'color 0.3s ease'
    }
  }
  
  const style: Record<string, string> = {
    fontSize: `clamp(12px, 15cqw, ${fontSize.value * 2}px)`,
    transition: 'color 0.3s ease'
  }
  if (effectiveColor.value) style.color = effectiveColor.value
  return style
})
</script>

<style scoped>
.text-widget {
  height: 100%;
  width: 100%;
}

/* --- CARD LAYOUT --- */
.text-widget.card-layout {
  padding: 12px;
  display: flex;
  align-items: center;
}

.card-content {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
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
.text-widget:not(.card-layout) {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 8px;
}

.value-display {
  font-weight: 600;
  text-align: center;
  word-break: break-word;
  line-height: 1.2;
  font-family: var(--mono);
  max-height: 100%;
  overflow-y: auto;
  color: var(--text);
  cursor: default;
}

.timestamp {
  margin-top: 8px;
  font-size: 11px;
  color: var(--muted);
  font-family: var(--mono);
}

@container (height < 80px) {
  .timestamp { display: none; }
}
</style>
