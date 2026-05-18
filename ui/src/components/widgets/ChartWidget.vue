<!-- ui/src/components/widgets/ChartWidget.vue -->
<template>
  <div class="chart-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <v-chart
      v-if="hasData"
      :option="chartOption"
      :autoresize="true"
      class="chart"
    />
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
import { computed, watch } from 'vue'
import { use } from 'echarts/core'
import { LineChart, BarChart, PieChart, GaugeChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDesignTokens } from '@/composables/useDesignTokens'
import { useUIStore } from '@/stores/ui'
import WidgetStateOverlay from '@/components/dashboard/WidgetStateOverlay.vue'
import type { WidgetConfig } from '@/types/dashboard'

// Register ECharts components
use([
  LineChart,
  BarChart,
  PieChart,
  GaugeChart,
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
const uiStore = useUIStore()
const { chartColors, chartStyling, getChartColorArray } = useDesignTokens()

// Get buffered data
const buffer = computed(() => dataStore.getBuffer(props.config.id))

// Check if we have any data
const hasData = computed(() => buffer.value.length > 0)

// Get chart type (default to line)
const chartType = computed(() => props.config.chartConfig?.chartType || 'line')

/**
 * Generate chart option based on chart type
 */
const chartOption = computed(() => {
  const data = buffer.value

  switch (chartType.value) {
    case 'line':
      return generateLineChart(data)
    case 'bar':
      return generateBarChart(data)
    case 'gauge':
      return generateGaugeChart(data)
    case 'pie':
      return generatePieChart(data)
    default:
      return generateLineChart(data)
  }
})

function generateLineChart(data: any[]) {
  const colors = chartColors.value
  const styling = chartStyling.value
  
  return {
    animation: false,
    grid: {
      left: 50,
      right: 20,
      top: 30,
      bottom: 40,
    },
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
      type: 'category',
      data: data.map((m) => new Date(m.timestamp).toLocaleTimeString()),
      axisLine: { 
        lineStyle: { color: styling.axis }
      },
      axisLabel: { 
        color: styling.muted,
        fontSize: 11,
      },
    },
    yAxis: {
      type: 'value',
      axisLine: { 
        lineStyle: { color: styling.axis }
      },
      axisLabel: { 
        color: styling.muted,
        fontSize: 11,
      },
      splitLine: { 
        lineStyle: { color: styling.grid }
      },
    },
    series: [
      {
        data: data.map((m) => m.value),
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: {
          color: colors.color1,
          width: 2,
        },
        itemStyle: {
          color: colors.color1,
        },
        // Grug say: Explicitly tell chart to keep line visible on hover
        emphasis: {
          disabled: false,
          lineStyle: {
            width: 2,
            color: colors.color1
          },
          itemStyle: {
            color: colors.color1
          }
        }
      },
    ],
    ...props.config.chartConfig?.echartOptions,
  }
}

function generateBarChart(data: any[]) {
  const colors = chartColors.value
  const styling = chartStyling.value
  
  return {
    grid: {
      left: 50,
      right: 20,
      top: 30,
      bottom: 40,
    },
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
      type: 'category',
      data: data.map((m) => new Date(m.timestamp).toLocaleTimeString()),
      axisLine: { 
        lineStyle: { color: styling.axis }
      },
      axisLabel: { 
        color: styling.muted,
        fontSize: 11,
      },
    },
    yAxis: {
      type: 'value',
      axisLine: { 
        lineStyle: { color: styling.axis }
      },
      axisLabel: { 
        color: styling.muted,
        fontSize: 11,
      },
      splitLine: { 
        lineStyle: { color: styling.grid }
      },
    },
    series: [
      {
        data: data.map((m) => m.value),
        type: 'bar',
        itemStyle: {
          color: colors.color2,
        },
        emphasis: {
          itemStyle: {
            color: colors.color2
          }
        }
      },
    ],
    ...props.config.chartConfig?.echartOptions,
  }
}

function generateGaugeChart(data: any[]) {
  const colors = chartColors.value
  const styling = chartStyling.value
  const latestValue = data.length > 0 ? data[data.length - 1].value : 0

  return {
    series: [
      {
        type: 'gauge',
        startAngle: 200,
        endAngle: -20,
        min: 0,
        max: 100,
        splitNumber: 5,
        itemStyle: {
          color: colors.color1,
        },
        progress: {
          show: true,
          width: 18,
        },
        pointer: {
          show: false,
        },
        axisLine: {
          lineStyle: {
            width: 18,
            color: [
              [0.3, colors.color2],
              [0.7, colors.color3],
              [1, colors.color5],
            ],
          },
        },
        axisTick: {
          distance: -22,
          splitNumber: 5,
          lineStyle: {
            width: 1,
            color: styling.muted,
          },
        },
        splitLine: {
          distance: -28,
          length: 14,
          lineStyle: {
            width: 2,
            color: styling.muted,
          },
        },
        axisLabel: {
          distance: -20,
          color: styling.muted,
          fontSize: 12,
        },
        anchor: {
          show: false,
        },
        title: {
          show: false,
        },
        detail: {
          valueAnimation: true,
          width: '60%',
          lineHeight: 40,
          borderRadius: 8,
          offsetCenter: [0, '10%'],
          fontSize: 32,
          fontWeight: 'bolder',
          formatter: '{value}',
          color: styling.text,
        },
        data: [
          {
            value: latestValue,
          },
        ],
      },
    ],
    ...props.config.chartConfig?.echartOptions,
  }
}

function generatePieChart(data: any[]) {
  const styling = chartStyling.value
  const pieData: Record<string, number> = {}
  
  data.forEach((m) => {
    const val = String(m.value)
    pieData[val] = (pieData[val] || 0) + 1
  })
  
  const colorArray = getChartColorArray(Object.keys(pieData).length)

  return {
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: styling.tooltipBg,
      borderColor: styling.tooltipBorder,
      textStyle: { 
        color: styling.text,
        fontSize: 12,
      },
    },
    legend: {
      orient: 'vertical',
      left: 'left',
      textStyle: { 
        color: styling.muted,
      },
    },
    color: colorArray,
    series: [
      {
        type: 'pie',
        radius: '60%',
        data: Object.entries(pieData).map(([name, value]) => ({
          name,
          value,
        })),
        emphasis: {
          itemStyle: {
            shadowBlur: 10,
            shadowOffsetX: 0,
            shadowColor: 'rgba(0, 0, 0, 0.5)',
          },
        },
      },
    ],
    ...props.config.chartConfig?.echartOptions,
  }
}

// Watch for theme changes via UI Store
watch(() => uiStore.theme, () => {
  // Trigger reactivity for computed properties
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

.chart-widget.card-layout {
  padding: 4px;
}
</style>
