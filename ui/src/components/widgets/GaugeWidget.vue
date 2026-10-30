<template>
  <div class="gauge-widget">
    <WidgetStateOverlay
      v-if="!hasData"
      state="empty"
      :icon="Gauge"
      message="Waiting for data..."
      :compact="layoutMode === 'card'"
    />

    <!-- Bar: the same range, zones and colour as the arc, laid flat -->
    <div v-else-if="isBar" class="bar-gauge">
      <div class="bar-value" :style="{ color: valueColor }">
        {{ displayValue }}<span v-if="cfg.unit" class="bar-unit">{{ cfg.unit }}</span>
      </div>
      <div class="bar-track">
        <div
          v-for="(zone, index) in zoneSpans"
          :key="index"
          class="bar-zone"
          :style="{ left: `${zone.start * 100}%`, width: `${(zone.end - zone.start) * 100}%`, background: zone.color }"
        />
        <div v-if="hasData" class="bar-fill" :style="{ width: `${valuePercent * 100}%`, background: valueColor }" />
      </div>
      <div class="bar-labels">
        <span>{{ cfg.min }}</span>
        <span>{{ cfg.max }}</span>
      </div>
    </div>

    <!-- SVG Gauge -->
    <svg v-else :viewBox="`0 0 ${size} ${size}`" class="gauge-svg">
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
        :y="center - 8"
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
        :y="center + 28"
        text-anchor="middle"
        dominant-baseline="middle"
        class="gauge-unit"
        fill="var(--muted)"
      >
        {{ cfg.unit }}
      </text>
    </svg>
    
    <!-- Min/Max labels -->
    <div v-if="hasData && !isBar" class="gauge-labels">
      <span class="label-min">{{ cfg.min }}</span>
      <span class="label-max">{{ cfg.max }}</span>
    </div>

  </div>
</template>

<script setup lang="ts">
import { Gauge } from '@lucide/vue'
import { computed } from 'vue'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDesignTokens } from '@/composables/useDesignTokens'
import WidgetStateOverlay from '@/components/dashboard/WidgetStateOverlay.vue'
import type { WidgetConfigOf } from '@/types/dashboard'

const props = defineProps<{
  config: WidgetConfigOf<'gauge'>
  layoutMode?: 'standard' | 'card'
}>()

const dataStore = useWidgetDataStore()
const { chartColors } = useDesignTokens()

const size = 200
const center = size / 2
const strokeWidth = 20
const radius = (size / 2) - (strokeWidth / 2) - 5

const cfg = computed(() => props.config.gaugeConfig)
const isBar = computed(() => cfg.value.style === 'bar')
const buffer = computed(() => dataStore.getBuffer(props.config.id))
const hasData = computed(() => buffer.value.length > 0)

// null when there is no usable number: Number(undefined) is NaN (a JSONPath
// that matched nothing drew "NaN" and a NaN arc) and Number(null) is 0 (a
// JetStream error drew a confident "0.00").
const latestValue = computed<number | null>(() => {
  if (buffer.value.length === 0) return null
  const val = buffer.value[buffer.value.length - 1].value
  if (val === null || val === undefined || val === '') return null
  const n = typeof val === 'number' ? val : Number(val)
  return Number.isFinite(n) ? n : null
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
// Was white at 10%, which vanished on the light theme.
const backgroundColor = computed(() => 'oklch(var(--bc) / 0.1)')

const valueColor = computed(() => {
  if (!cfg.value.zones || cfg.value.zones.length === 0) return chartColors.value.color1
  const value = clampedValue.value
  for (const zone of cfg.value.zones) {
    if (value >= zone.min && value <= zone.max) return zone.color
  }
  return chartColors.value.color1
})

// Each zone as a fraction of the range, 0..1. The arc and the bar both draw
// from this.
const zoneSpans = computed(() => {
  if (!cfg.value.zones) return []
  const range = cfg.value.max - cfg.value.min
  const frac = (v: number) => Math.max(0, Math.min(1, (v - cfg.value.min) / range))
  return cfg.value.zones.map(zone => ({ color: zone.color, start: frac(zone.min), end: frac(zone.max) }))
})

const zonePaths = computed(() =>
  zoneSpans.value.map(zone => ({
    color: zone.color,
    path: describeArc(center, center, radius, -135 + 270 * zone.start, -135 + 270 * zone.end),
  }))
)
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
  font-size: 44px;
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
  width: 80%;
  max-width: 180px;
  font-size: 12px;
  color: var(--muted);
  font-family: var(--mono);
}

.bar-gauge {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 0 8px;
}

.bar-value {
  font-size: clamp(20px, 18cqw, 40px);
  font-weight: 700;
  font-family: var(--mono);
  line-height: 1;
  transition: color 0.3s ease;
}

.bar-unit {
  font-size: 0.5em;
  font-weight: 500;
  color: var(--muted);
  margin-left: 4px;
}

.bar-track {
  position: relative;
  height: clamp(10px, 12cqh, 24px);
  border-radius: 999px;
  background: oklch(var(--bc) / 0.1);
  overflow: hidden;
}

.bar-zone {
  position: absolute;
  top: 0;
  bottom: 0;
  opacity: 0.3;
}

.bar-fill {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  border-radius: 999px;
  transition: width 0.5s ease-out, background 0.3s ease;
}

.bar-labels {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
  font-family: var(--mono);
}

/* Hide labels on small width */
@container (width < 140px) {
  .gauge-labels { display: none; }
  .gauge-unit { display: none; }
}
</style>
