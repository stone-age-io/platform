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
import { use, graphic } from 'echarts/core'
import { LineChart, BarChart, CustomChart } from 'echarts/charts'
import { formatDistanceStrict } from 'date-fns'
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
import { useUIStore } from '@/stores/ui'
import { useDesignTokens } from '@/composables/useDesignTokens'
import { useThresholds } from '@/composables/useThresholds'
import WidgetStateOverlay from '@/components/dashboard/WidgetStateOverlay.vue'
import { parseDurationMs } from '@/utils/duration'
import { resolveTemplate } from '@/utils/variables'
import { seriesPoints, numericPoints, buildSegments, stableColorIndex } from '@/utils/chartSeries'
import type { WidgetConfig, ChartSeries } from '@/types/dashboard'

use([
  LineChart,
  BarChart,
  CustomChart,
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
const uiStore = useUIStore()
const { chartStyling, getChartColor, resolveColor } = useDesignTokens()
const { matchThreshold } = useThresholds()

const PALETTE_SIZE = 8 // CHART_SERIES in useDesignTokens

const buffer = computed(() => dataStore.getBuffer(props.config.id))
const hasData = computed(() => buffer.value.length > 0)

// pie and gauge were removed; anything unrecognised draws as line.
const chartType = computed(() => {
  const t = props.config.chartConfig?.chartType
  return t === 'bar' || t === 'timeline' ? t : 'line'
})
const windowMs = computed(() => parseDurationMs(props.config.chartConfig?.window))

// A chart saved before multi-series has no list: it is one series read from
// the value the widget's jsonPath already extracted (null = that series).
const seriesList = computed<(ChartSeries | null)[]>(() => {
  const s = props.config.chartConfig?.series
  return s && s.length > 0 ? s : [null]
})

// --- Clock ---
// With a window the axis must keep sliding while no messages arrive, or an
// idle chart freezes on the last moment anything happened. A timeline needs it
// regardless: its last segment runs to "now".
const now = ref(Date.now())
let timer: number | undefined

watch(() => !!windowMs.value || chartType.value === 'timeline', (ticking) => {
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
  if (ticking) timer = window.setInterval(() => { now.value = Date.now() }, 1000)
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

const chartOption = computed(() =>
  chartType.value === 'timeline' ? timelineOption() : xyOption()
)

// --- Line / bar ---
function xyOption() {
  const styling = chartStyling.value
  const vars = dashboardStore.currentVariableValues
  const resolve = (subject: string) => resolveTemplate(subject, vars)
  const multi = seriesList.value.length > 1
  const ms = windowMs.value

  const series = seriesList.value.map((s, i) => {
    const color = getChartColor(i + 1) // 1-based
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
}

// --- State timeline ---
// One row per series, consecutive equal values merged into a segment (the
// merging and its edge cases live in utils/chartSeries, where they are tested).

interface TimelineItem {
  row: number
  start: number
  end: number
  text: string
  color: string
}

// Rule colours are CSS variables (var(--color-error)), which canvas cannot
// read. Resolve each once per theme, not once per segment per second.
const ruleColors = computed(() => {
  void uiStore.theme
  const out = new Map<string, string>()
  for (const r of props.config.chartConfig?.thresholds ?? []) out.set(r.id, resolveColor(r.color))
  return out
})

// Values come from device payloads and the tooltip is HTML.
function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!))
}

function timelineOption() {
  const styling = chartStyling.value
  const vars = dashboardStore.currentVariableValues
  const resolve = (subject: string) => resolveTemplate(subject, vars)
  const rules = props.config.chartConfig?.thresholds
  const ms = windowMs.value
  const labels = seriesList.value.map(seriesLabel)

  const items: TimelineItem[] = []
  seriesList.value.forEach((s, row) => {
    for (const seg of buildSegments(seriesPoints(buffer.value, s, resolve), now.value)) {
      const rule = matchThreshold(seg.value, rules)
      const raw = String(seg.value)
      items.push({
        row,
        start: seg.start,
        end: seg.end,
        text: rule?.label?.trim() || raw,
        color: rule ? ruleColors.value.get(rule.id) ?? rule.color : getChartColor(stableColorIndex(raw, PALETTE_SIZE) + 1),
      })
    }
  })

  return {
    animation: false,
    grid: { left: 90, right: 16, top: 8, bottom: 30 },
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: styling.tooltipBg,
      borderColor: styling.tooltipBorder,
      textStyle: { color: styling.text, fontSize: 12 },
      formatter: (p: { dataIndex: number }) => {
        const it = items[p.dataIndex]
        if (!it) return ''
        const from = new Date(it.start).toLocaleTimeString()
        const to = new Date(it.end).toLocaleTimeString()
        return `${escapeHtml(labels[it.row])}<br/><b>${escapeHtml(it.text)}</b><br/>`
          + `${from} – ${to} (${formatDistanceStrict(it.start, it.end)})`
      },
    },
    xAxis: {
      type: 'time',
      min: ms ? now.value - ms : undefined,
      max: now.value,
      axisLine: { lineStyle: { color: styling.axis } },
      axisLabel: { color: styling.muted, fontSize: 11, hideOverlap: true },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'category',
      data: labels,
      inverse: true, // first series on top, the way it is listed in the config
      axisLine: { lineStyle: { color: styling.axis } },
      axisTick: { show: false },
      axisLabel: { color: styling.muted, fontSize: 11, width: 80, overflow: 'truncate' },
    },
    series: [{
      type: 'custom',
      encode: { x: [1, 2], y: 0 },
      data: items.map(it => [it.row, it.start, it.end]),
      renderItem: (params: any, api: any) => {
        const it = items[params.dataIndex]
        const from = api.coord([api.value(1), api.value(0)])
        const to = api.coord([api.value(2), api.value(0)])
        const height = api.size([0, 1])[1] * 0.7
        const cs = params.coordSys
        const shape = graphic.clipRectByRect(
          { x: from[0], y: from[1] - height / 2, width: to[0] - from[0], height },
          { x: cs.x, y: cs.y, width: cs.width, height: cs.height },
        )
        if (!shape) return null
        return {
          type: 'rect',
          shape,
          style: { fill: it.color },
          // White text with a dark outline reads on every palette colour
          // without computing a contrast per fill.
          textContent: shape.width > 28 ? {
            style: {
              text: it.text,
              fill: '#fff',
              fontSize: 11,
              textBorderColor: 'rgba(0,0,0,0.45)',
              textBorderWidth: 2,
              overflow: 'truncate',
              width: shape.width - 6,
            },
          } : undefined,
          textConfig: { position: 'inside' },
        }
      },
    }],
    ...props.config.chartConfig?.echartOptions,
  }
}
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
