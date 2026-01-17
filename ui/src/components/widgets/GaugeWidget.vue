<template>
  <div class="gauge-widget">
    <!-- SVG Gauge -->
    <svg :viewBox="`0 0 ${size} ${size}`" class="gauge-svg">
      <!-- Background track -->
      <path
        :d="backgroundPath"
        fill="none"
        :stroke="backgroundColor"
        :stroke-width="strokeWidth"
        stroke-linecap="round"
        class="background-arc"
      />
      
      <!-- Zone segments -->
      <path
        v-for="(zone, index) in zonePaths"
        :key="index"
        :d="zone.path"
        fill="none"
        :stroke="zone.color"
        :stroke-width="strokeWidth"
        stroke-linecap="round"
        class="zone-arc"
      />
      
      <!-- Value arc -->
      <path
        v-if="hasData"
        :d="fullGaugePath"
        fill="none"
        :stroke="valueColor"
        :stroke-width="strokeWidth + 2"
        :stroke-dasharray="valueArcLength"
        :stroke-dashoffset="valueDashOffset"
        stroke-linecap="round"
        class="value-arc"
      />
      
      <!-- Center value text -->
      <text
        :x="center"
        :y="center - 5"
        text-anchor="middle"
        dominant-baseline="middle"
        class="gauge-value"
        :fill="valueColor"
      >
        {{ displayValue }}
      </text>
      
      <!-- Unit text -->
      <text
        v-if="cfg.unit"
        :x="center"
        :y="center + 18"
        text-anchor="middle"
        dominant-baseline="middle"
        class="gauge-unit"
        fill="var(--muted)"
      >
        {{ cfg.unit }}
      </text>
    </svg>
    
    <!-- Min/Max labels -->
    <div class="gauge-labels">
      <span class="label-min">{{ cfg.min }}</span>
      <span class="label-max">{{ cfg.max }}</span>
    </div>
    
    <!-- No data state -->
    <div v-if="!hasData" class="no-data-overlay">
      <div class="no-data-icon">📊</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDesignTokens } from '@/composables/useDesignTokens'
import type { WidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  config: WidgetConfig
}>()

const dataStore = useWidgetDataStore()
const { chartColors } = useDesignTokens()

const size = 200
const center = size / 2
const strokeWidth = 20
const radius = (size / 2) - (strokeWidth / 2) - 5

const cfg = computed(() => props.config.gaugeConfig!)
const buffer = computed(() => dataStore.getBuffer(props.config.id))
const hasData = computed(() => buffer.value.length > 0)

const latestValue = computed(() => {
  if (buffer.value.length === 0) return null
  const val = buffer.value[buffer.value.length - 1].value
  return typeof val === 'number' ? val : Number(val)
})

const clampedValue = computed(() => {
  if (latestValue.value === null) return cfg.value.min
  return Math.max(cfg.value.min, Math.min(cfg.value.max, latestValue.value))
})

const displayValue = computed(() => {
  if (latestValue.value === null) return '—'
  return latestValue.value.toFixed(getDecimalPlaces(latestValue.value))
})

function getDecimalPlaces(value: number): number {
  if (Math.abs(value) >= 100) return 0
  if (Math.abs(value) >= 10) return 1
  return 2
}

const valuePercent = computed(() => {
  const range = cfg.value.max - cfg.value.min
  return (clampedValue.value - cfg.value.min) / range
})

function polarToCartesian(centerX: number, centerY: number, radius: number, angleInDegrees: number) {
  const angleInRadians = (angleInDegrees - 90) * Math.PI / 180.0
  return {
    x: centerX + (radius * Math.cos(angleInRadians)),
    y: centerY + (radius * Math.sin(angleInRadians))
  }
}

function describeArc(centerX: number, centerY: number, radius: number, startAngle: number, endAngle: number) {
  const start = polarToCartesian(centerX, centerY, radius, endAngle)
  const end = polarToCartesian(centerX, centerY, radius, startAngle)
  const largeArcFlag = endAngle - startAngle <= 180 ? "0" : "1"
  return ["M", start.x, start.y, "A", radius, radius, 0, largeArcFlag, 0, end.x, end.y].join(" ")
}

const backgroundPath = computed(() => describeArc(center, center, radius, -135, 135))
const fullGaugePath = computed(() => describeArc(center, center, radius, -135, 135))
const fullArcLength = computed(() => radius * (270 * Math.PI) / 180)
const valueArcLength = computed(() => `${fullArcLength.value * valuePercent.value} ${fullArcLength.value * 10}`)
const valueDashOffset = computed(() => -fullArcLength.value * (1 - valuePercent.value))
const backgroundColor = computed(() => 'rgba(255, 255, 255, 0.1)')

const valueColor = computed(() => {
  if (!cfg.value.zones || cfg.value.zones.length === 0) return chartColors.value.color1
  const value = clampedValue.value
  for (const zone of cfg.value.zones) {
    if (value >= zone.min && value <= zone.max) return zone.color
  }
  return chartColors.value.color1
})

const zonePaths = computed(() => {
  if (!cfg.value.zones) return []
  const range = cfg.value.max - cfg.value.min
  return cfg.value.zones.map(zone => {
    const start = -135 + (270 * ((zone.min - cfg.value.min) / range))
    const end = -135 + (270 * ((zone.max - cfg.value.min) / range))
    return { color: zone.color, path: describeArc(center, center, radius, start, end) }
  })
})
</script>

<style scoped>
.gauge-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 8px;
  background: var(--widget-bg);
  position: relative;
}

.gauge-svg {
  width: 100%;
  max-width: 200px;
  height: auto;
  max-height: 100%;
}

.background-arc { transition: stroke 0.3s; }
.zone-arc { opacity: 0.3; transition: opacity 0.3s; }
.value-arc { transition: stroke-dasharray 0.5s ease-out, stroke-dashoffset 0.5s ease-out, stroke 0.3s ease; filter: drop-shadow(0 0 4px currentColor); }

.gauge-value {
  font-size: 36px;
  font-weight: 700;
  font-family: var(--mono);
  transition: fill 0.3s ease;
}

.gauge-unit {
  font-size: 18px;
  font-weight: 500;
}

.gauge-labels {
  display: flex;
  justify-content: space-between;
  width: 100%;
  max-width: 180px;
  font-size: 18px;
  color: var(--muted);
  font-family: var(--mono);
}

/* Hide labels on small width */
@container (width < 140px) {
  .gauge-labels { display: none; }
  .gauge-unit { display: none; }
}

.no-data-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--widget-bg);
  color: var(--muted);
}

.no-data-icon {
  font-size: 32px;
  opacity: 0.5;
}
</style>
