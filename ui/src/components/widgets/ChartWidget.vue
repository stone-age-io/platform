<!-- ui/src/components/widgets/ChartWidget.vue -->
<template>
  <div class="chart-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <template v-if="hasData">
      <v-chart
        :option="chartOption"
        :autoresize="true"
        class="chart"
      />
      <!-- The buffer size caps memory even with a time window. When the cap is
           what bounds the chart, say so: otherwise a "last hour" chart quietly
           shows the last ten minutes. -->
      <div v-if="bufferFull" class="buffer-hint">
        buffer full: showing last {{ buffer.length }} messages
      </div>
    </template>
    <WidgetStateOverlay
      v-else
      state="empty"
      icon="📈"
      message="Waiting for data..."
      :compact="layoutMode === 'card'"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { use } from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
import { useDesignTokens } from '@/composables/useDesignTokens'
import WidgetStateOverlay from '@/components/dashboard/WidgetStateOverlay.vue'
import { parseDurationMs } from '@/utils/duration'
import { resolveTemplate } from '@/utils/variables'
import { seriesPoints, numericPoints } from '@/utils/chartSeries'
import type { WidgetConfig, ChartSeries } from '@/types/dashboard'

use([
  LineChart,
  BarChart,
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  CanvasRenderer,
])

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dataStore = useWidgetDataStore()
const dashboardStore = useDashboardStore()
const { chartStyling, getChartColor } = useDesignTokens()

const buffer = computed(() => dataStore.getBuffer(props.config.id))
const hasData = computed(() => buffer.value.length > 0)

const chartType = computed(() => props.config.chartConfig?.chartType === 'bar' ? 'bar' : 'line')
const windowMs = computed(() => parseDurationMs(props.config.chartConfig?.window))

// A chart saved before multi-series has no list: it is one series read from
// the value the widget's jsonPath already extracted (null = that series).
const seriesList = computed<(ChartSeries | null)[]>(() => {
  const s = props.config.chartConfig?.series
  return s && s.length > 0 ? s : [null]
})

// --- Clock ---
// With a window the axis must keep sliding while no messages arrive, or an
// idle chart freezes on the last moment anything happened.
const now = ref(Date.now())
let timer: number | undefined

watch(windowMs, (ms) => {
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
  if (ms) timer = window.setInterval(() => { now.value = Date.now() }, 1000)
}, { immediate: true })

onUnmounted(() => {
  if (timer !== undefined) window.clearInterval(timer)
})

// --- Buffer-full hint ---
const bufferFull = computed(() => {
  const ms = windowMs.value
  if (!ms) return false
  const cap = Math.min(props.config.buffer.maxCount, dataStore.MEMORY_LIMITS.MAX_SINGLE_BUFFER)
  if (buffer.value.length < cap) return false
  let oldest = Infinity
  for (const m of buffer.value) if (m.timestamp < oldest) oldest = m.timestamp
  return oldest > now.value - ms
})

function seriesLabel(s: ChartSeries | null, i: number): string {
  if (!s) return 'Value'
  return s.label.trim() || s.path || `Series ${i + 1}`
}

const chartOption = computed(() => {
  const styling = chartStyling.value
  const vars = dashboardStore.currentVariableValues
  const resolve = (subject: string) => resolveTemplate(subject, vars)
  const multi = seriesList.value.length > 1
  const ms = windowMs.value

  const series = seriesList.value.map((s, i) => {
    const color = getChartColor(i)
    const data = numericPoints(seriesPoints(buffer.value, s, resolve))
    const common = {
      name: seriesLabel(s, i),
      data,
      itemStyle: { color },
    }
    if (chartType.value === 'bar') {
      return { ...common, type: 'bar', barMaxWidth: 12, emphasis: { itemStyle: { color } } }
    }
    return {
      ...common,
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 6,
      showSymbol: data.length <= 50,
      lineStyle: { color, width: 2 },
      // Grug say: Explicitly tell chart to keep line visible on hover
      emphasis: { disabled: false, lineStyle: { width: 2, color }, itemStyle: { color } },
    }
  })

  return {
    animation: false,
    grid: {
      left: 50,
      right: 20,
      top: multi ? 36 : 30,
      bottom: 40,
    },
    // The palette requires a legend once colour is what tells series apart
    // (see CHART_SERIES in useDesignTokens). One series needs none.
    legend: multi
      ? { top: 0, type: 'scroll', textStyle: { color: styling.muted, fontSize: 11 } }
      : undefined,
    tooltip: {
      trigger: 'axis',
      confine: true,
      backgroundColor: styling.tooltipBg,
      borderColor: styling.tooltipBorder,
      textStyle: {
        color: styling.text,
        fontSize: 12,
      },
    },
    xAxis: {
      type: 'time',
      min: ms ? now.value - ms : undefined,
      max: ms ? now.value : undefined,
      axisLine: { lineStyle: { color: styling.axis } },
      axisLabel: { color: styling.muted, fontSize: 11, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: styling.axis } },
      axisLabel: { color: styling.muted, fontSize: 11 },
      splitLine: { lineStyle: { color: styling.grid } },
    },
    series,
    ...props.config.chartConfig?.echartOptions,
  }
})
</script>

<style scoped>
.chart-widget {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  padding: 8px;
  background: var(--widget-bg);
  border-radius: 8px;
}

.chart {
  flex: 1;
  min-height: 0;
}

.buffer-hint {
  font-size: 11px;
  color: var(--muted);
  text-align: right;
  padding-top: 2px;
}

.chart-widget.card-layout {
  padding: 4px;
}
</style>
