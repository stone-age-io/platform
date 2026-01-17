<template>
  <div class="stat-card-widget">
    <!-- Main value -->
    <div class="stat-value" :style="{ color: valueColor }">
      {{ displayValue }}
    </div>
    
    <!-- Trend indicator -->
    <div v-if="showTrend && trend !== null" class="stat-trend" :class="trendClass">
      <span class="trend-arrow">{{ trendArrow }}</span>
      <span class="trend-text">{{ trendText }}</span>
    </div>
    
    <!-- Mini sparkline -->
    <div v-if="showSparkline" class="sparkline">
      <svg :viewBox="`0 0 ${sparklineWidth} ${sparklineHeight}`" class="sparkline-svg">
        <polyline
          :points="sparklinePoints"
          fill="none"
          :stroke="valueColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </div>
    
    <!-- No data state -->
    <div v-if="!hasData" class="no-data">
      <div class="no-data-icon">📊</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDesignTokens } from '@/composables/useDesignTokens'
import { useThresholds } from '@/composables/useThresholds'
import type { WidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  config: WidgetConfig
}>()

const dataStore = useWidgetDataStore()
const { baseColors } = useDesignTokens()
const { evaluateThresholds } = useThresholds()

const cfg = computed(() => props.config.statConfig!)
const showTrend = computed(() => cfg.value.showTrend ?? true)
const trendWindow = computed(() => cfg.value.trendWindow ?? 10)
const thresholds = computed(() => cfg.value.thresholds || [])

const buffer = computed(() => dataStore.getBuffer(props.config.id))
const hasData = computed(() => buffer.value.length > 0)

const latestValue = computed(() => {
  if (buffer.value.length === 0) return null
  return buffer.value[buffer.value.length - 1].value
})

const previousValue = computed(() => {
  const index = buffer.value.length - 1 - trendWindow.value
  if (index < 0 || buffer.value.length < 2) return null
  return buffer.value[index].value
})

const trend = computed(() => {
  if (latestValue.value === null || previousValue.value === null) return null
  const current = Number(latestValue.value)
  const previous = Number(previousValue.value)
  if (isNaN(current) || isNaN(previous) || previous === 0) return null
  const change = current - previous
  const percent = (change / Math.abs(previous)) * 100
  return { change, percent, isPositive: change > 0, isNegative: change < 0, isNeutral: change === 0 }
})

const trendArrow = computed(() => {
  if (!trend.value) return ''
  if (trend.value.isPositive) return '↑'
  if (trend.value.isNegative) return '↓'
  return '→'
})

const trendText = computed(() => {
  if (!trend.value) return ''
  return `${Math.abs(trend.value.percent).toFixed(1)}%`
})

const trendClass = computed(() => {
  if (!trend.value) return ''
  if (trend.value.isPositive) return 'trend-positive'
  if (trend.value.isNegative) return 'trend-negative'
  return 'trend-neutral'
})

const displayValue = computed(() => {
  if (latestValue.value === null) return '—'
  const value = latestValue.value
  const format = cfg.value.format
  const unit = cfg.value.unit || ''
  if (format) return format.replace('{value}', String(value))
  if (typeof value === 'number') return `${value.toFixed(getDecimalPlaces(value))}${unit}`
  return `${value}${unit}`
})

function getDecimalPlaces(value: number): number {
  if (Math.abs(value) >= 100) return 0
  if (Math.abs(value) >= 10) return 1
  return 2
}

const valueColor = computed(() => {
  if (latestValue.value === null) return baseColors.value.muted
  const color = evaluateThresholds(latestValue.value, thresholds.value)
  return color || baseColors.value.text
})

const showSparkline = computed(() => buffer.value.length > 5)
const sparklineWidth = 100
const sparklineHeight = 20

const sparklinePoints = computed(() => {
  if (!showSparkline.value) return ''
  const values = buffer.value.slice(-20).map(m => Number(m.value)).filter(v => !isNaN(v))
  if (values.length < 2) return ''
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  return values.map((value, index) => {
    const x = (index / (values.length - 1)) * sparklineWidth
    const y = sparklineHeight - ((value - min) / range) * sparklineHeight
    return `${x},${y}`
  }).join(' ')
})
</script>

<style scoped>
.stat-card-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 8px;
  background: var(--widget-bg);
  gap: 4px;
}

.stat-value {
  font-size: clamp(24px, 25cqw, 64px);
  font-weight: 700;
  font-family: var(--mono);
  line-height: 1;
  text-align: center;
  transition: color 0.3s ease;
}

.stat-trend {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: clamp(10px, 8cqw, 16px);
  font-weight: 600;
}

.trend-positive { color: var(--color-success); }
.trend-negative { color: var(--color-error); }
.trend-neutral { color: var(--muted); }

.trend-arrow {
  font-size: 1.2em;
  line-height: 1;
}

.sparkline {
  width: 100%;
  max-width: 120px;
  height: 24px;
  opacity: 0.6;
}

.sparkline-svg {
  width: 100%;
  height: 100%;
}

/* Hide sparkline on small height */
@container (height < 80px) {
  .sparkline { display: none; }
}

.no-data {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--muted);
}

.no-data-icon {
  font-size: 32px;
  opacity: 0.5;
}
</style>
