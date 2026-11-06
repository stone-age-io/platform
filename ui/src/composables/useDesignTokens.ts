import { computed } from 'vue'
import { useUIStore } from '@/stores/ui'

/**
 * Resolves DaisyUI CSS variables to explicit RGB strings, because Canvas and
 * ECharts cannot read CSS variables.
 */

function resolveColor(colorString: string): string {
  if (typeof document === 'undefined') return colorString

  const el = document.createElement('div')
  el.style.color = colorString
  el.style.display = 'none'
  document.body.appendChild(el)
  const computed = window.getComputedStyle(el).color
  document.body.removeChild(el)
  
  return computed || colorString
}

/**
 * Categorical chart series ramp.
 *
 * Not taken from the DaisyUI theme: its secondary and neutral are near-grays,
 * and primary is the colour of every control. Each mode is stepped and
 * validated for its own surface (#FFFFFF light, #12161D dark) for CVD
 * separation and contrast.
 *
 * In light mode aqua, yellow and magenta are under 3:1 against white, so a
 * chart from this ramp needs a legend or direct labels (ChartWidget has one).
 *
 * Do not reorder: colour follows the series, so dropping one must not repaint
 * the rest.
 */
const CHART_SERIES: Record<'light' | 'dark', readonly string[]> = {
  //        blue       orange     aqua       yellow     magenta    green      violet     red
  light: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#4a3aa7', '#e34948'],
  dark:  ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'],
}

function resolveAlpha(colorString: string, alpha: number): string {
  const rgb = resolveColor(colorString)
  if (rgb.startsWith('rgb(')) {
    return rgb.replace('rgb(', 'rgba(').replace(')', `, ${alpha})`)
  }
  if (rgb.startsWith('rgba(')) {
    return rgb.replace(/, [\d\.]+\)/, `, ${alpha})`)
  }
  return rgb
}

export function useDesignTokens() {
  const uiStore = useUIStore()

  // The CSS variables are not reactive; each computed reads `theme` so it
  // re-resolves when the theme changes.
  const theme = computed(() => uiStore.theme)

  const baseColors = computed(() => {
    void theme.value 
    return {
      bg: resolveColor('oklch(var(--b2))'),
      panel: resolveColor('oklch(var(--b1))'),
      border: resolveColor('oklch(var(--b3))'),
      inputBg: resolveColor('oklch(var(--b2))'),
      text: resolveColor('oklch(var(--bc))'),
      textSecondary: resolveAlpha('oklch(var(--bc))', 0.7),
      muted: resolveAlpha('oklch(var(--bc))', 0.5),
    }
  })

  const semanticColors = computed(() => {
    void theme.value
    return {
      // DaisyUI v4 has no --pf/--sf/--af focus vars, so hover is an alpha step.
      primary: resolveColor('oklch(var(--p))'),
      primaryHover: resolveAlpha('oklch(var(--p))', 0.85),
      primaryActive: resolveColor('oklch(var(--p))'),

      secondary: resolveColor('oklch(var(--s))'),
      secondaryHover: resolveAlpha('oklch(var(--s))', 0.85),
      secondaryActive: resolveColor('oklch(var(--s))'),

      accent: resolveColor('oklch(var(--a))'),
      accentHover: resolveAlpha('oklch(var(--a))', 0.85),

      success: resolveColor('oklch(var(--su))'),
      successBg: resolveAlpha('oklch(var(--su))', 0.15),
      successBorder: resolveAlpha('oklch(var(--su))', 0.4),
      
      warning: resolveColor('oklch(var(--wa))'),
      warningBg: resolveAlpha('oklch(var(--wa))', 0.15),
      warningBorder: resolveAlpha('oklch(var(--wa))', 0.4),
      
      error: resolveColor('oklch(var(--er))'),
      errorBg: resolveAlpha('oklch(var(--er))', 0.15),
      errorBorder: resolveAlpha('oklch(var(--er))', 0.4),
      
      info: resolveColor('oklch(var(--in))'),
      infoBg: resolveAlpha('oklch(var(--in))', 0.15),
      infoBorder: resolveAlpha('oklch(var(--in))', 0.4),
    }
  })

  const chartColors = computed(() => {
    const ramp = CHART_SERIES[theme.value]
    return {
      color1: ramp[0],
      color2: ramp[1],
      color3: ramp[2],
      color4: ramp[3],
      color5: ramp[4],
      color6: ramp[5],
      color7: ramp[6],
      color8: ramp[7],

      color1Alpha30: resolveAlpha(ramp[0], 0.3),
      color1Alpha05: resolveAlpha(ramp[0], 0.05),
      color2Alpha30: resolveAlpha(ramp[1], 0.3),
      color2Alpha05: resolveAlpha(ramp[1], 0.05),
      color3Alpha30: resolveAlpha(ramp[2], 0.3),
      color3Alpha05: resolveAlpha(ramp[2], 0.05),
    }
  })

  const thresholdColors = computed(() => {
    void theme.value
    return {
      normal: resolveColor('oklch(var(--su))'),
      normalBg: resolveAlpha('oklch(var(--su))', 0.15),
      
      warning: resolveColor('oklch(var(--wa))'),
      warningBg: resolveAlpha('oklch(var(--wa))', 0.15),
      
      critical: resolveColor('oklch(var(--er))'),
      criticalBg: resolveAlpha('oklch(var(--er))', 0.15),
      
      severe: '#b91c1c',
      severeBg: 'rgba(185, 28, 28, 0.15)',
    }
  })

  const statusColors = computed(() => {
    void theme.value
    return {
      active: resolveColor('oklch(var(--su))'),
      inactive: resolveColor('oklch(var(--n))'),
      pending: resolveColor('oklch(var(--wa))'),
      error: resolveColor('oklch(var(--er))'),
      unknown: resolveAlpha('oklch(var(--bc))', 0.5),
    }
  })

  const connectionColors = computed(() => {
    void theme.value
    return {
      connected: resolveColor('oklch(var(--su))'),
      connecting: resolveColor('oklch(var(--wa))'),
      reconnecting: resolveColor('oklch(var(--wa))'),
      disconnected: resolveColor('oklch(var(--er))'),
    }
  })

  const chartStyling = computed(() => {
    void theme.value
    return {
      grid: resolveColor('oklch(var(--b3))'),
      axis: resolveAlpha('oklch(var(--bc))', 0.6),
      tooltipBg: resolveAlpha('oklch(var(--b1))', 0.95),
      tooltipBorder: resolveColor('oklch(var(--b3))'),
      text: resolveColor('oklch(var(--bc))'),
      textSecondary: resolveAlpha('oklch(var(--bc))', 0.7),
      muted: resolveAlpha('oklch(var(--bc))', 0.5),
      border: resolveColor('oklch(var(--b3))'),
    }
  })

  /**
   * Series colour for a 1-based index. Past eight it wraps rather than
   * generating an unvalidated hue; fix too many series in the data instead.
   */
  function getChartColor(index: number): string {
    const ramp = CHART_SERIES[theme.value]
    return ramp[(index - 1) % ramp.length]
  }


  function getThresholdColor(
    value: number,
    thresholds: { warning?: number; critical?: number; severe?: number }
  ): string {
    const { warning, critical, severe } = thresholds
    if (severe !== undefined && value >= severe) return thresholdColors.value.severe
    if (critical !== undefined && value >= critical) return thresholdColors.value.critical
    if (warning !== undefined && value >= warning) return thresholdColors.value.warning
    return thresholdColors.value.normal
  }

  function getConnectionColor(status: string): string {
    switch (status.toLowerCase()) {
      case 'connected': return connectionColors.value.connected
      case 'connecting': return connectionColors.value.connecting
      case 'reconnecting': return connectionColors.value.reconnecting
      case 'disconnected':
      default: return connectionColors.value.disconnected
    }
  }

  // Legacy aliases of resolveAlpha
  function hexToRgba(hex: string, alpha: number): string {
    return resolveAlpha(hex, alpha)
  }

  function getColorWithAlpha(color: string, alpha: number): string {
    return resolveAlpha(color, alpha)
  }

  function getToken(tokenName: string): string {
    return getComputedStyle(document.documentElement).getPropertyValue(tokenName).trim()
  }

  return {
    getToken,
    resolveColor,
    chartColors,
    semanticColors,
    thresholdColors,
    statusColors,
    connectionColors,
    chartStyling,
    baseColors,
    getChartColor,
    getThresholdColor,
    getConnectionColor,
    hexToRgba,
    getColorWithAlpha,
  }
}
