import { computed } from 'vue'
import { useUIStore } from '@/stores/ui'

/**
 * Design Tokens Composable
 * 
 * Maps Stone Age (DaisyUI) CSS variables to the NATS Dashboard token system.
 * Resolves CSS variables to explicit RGB values for Canvas/ECharts compatibility.
 */

// Helper to resolve CSS color string to computed RGB string
function resolveColor(colorString: string): string {
  if (typeof document === 'undefined') return colorString
  
  // Create a temporary element to resolve the color
  const el = document.createElement('div')
  el.style.color = colorString
  el.style.display = 'none'
  document.body.appendChild(el)
  const computed = window.getComputedStyle(el).color
  document.body.removeChild(el)
  
  return computed || colorString
}

// Helper to add alpha to an RGB string
function resolveAlpha(colorString: string, alpha: number): string {
  const rgb = resolveColor(colorString)
  // Handle "rgb(r, g, b)" -> "rgba(r, g, b, a)"
  if (rgb.startsWith('rgb(')) {
    return rgb.replace('rgb(', 'rgba(').replace(')', `, ${alpha})`)
  }
  // Handle "rgba(r, g, b, a)" -> replace a
  if (rgb.startsWith('rgba(')) {
    return rgb.replace(/, [\d\.]+\)/, `, ${alpha})`)
  }
  return rgb
}

export function useDesignTokens() {
  const uiStore = useUIStore()

  // Helper to ensure reactivity
  const theme = computed(() => uiStore.theme)

  /**
   * Base Colors
   */
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

  /**
   * Semantic Colors
   */
  const semanticColors = computed(() => {
    void theme.value
    return {
      primary: resolveColor('oklch(var(--p))'),
      primaryHover: resolveColor('oklch(var(--pf))'),
      primaryActive: resolveColor('oklch(var(--p))'),
      
      secondary: resolveColor('oklch(var(--s))'),
      secondaryHover: resolveColor('oklch(var(--sf))'),
      secondaryActive: resolveColor('oklch(var(--s))'),
      
      accent: resolveColor('oklch(var(--a))'),
      accentHover: resolveColor('oklch(var(--af))'),
      
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

  /**
   * Chart Colors
   * Resolved to RGB for ECharts
   */
  const chartColors = computed(() => {
    void theme.value
    return {
      color1: resolveColor('oklch(var(--p))'),
      color2: resolveColor('oklch(var(--s))'),
      color3: resolveColor('oklch(var(--a))'),
      color4: resolveColor('oklch(var(--n))'),
      color5: resolveColor('oklch(var(--su))'),
      color6: resolveColor('oklch(var(--wa))'),
      color7: resolveColor('oklch(var(--er))'),
      color8: resolveColor('oklch(var(--in))'),
      
      color1Alpha30: resolveAlpha('oklch(var(--p))', 0.3),
      color1Alpha05: resolveAlpha('oklch(var(--p))', 0.05),
      color2Alpha30: resolveAlpha('oklch(var(--s))', 0.3),
      color2Alpha05: resolveAlpha('oklch(var(--s))', 0.05),
      color3Alpha30: resolveAlpha('oklch(var(--a))', 0.3),
      color3Alpha05: resolveAlpha('oklch(var(--a))', 0.05),
    }
  })

  /**
   * Threshold Colors
   */
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

  /**
   * Status Colors
   */
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

  /**
   * Connection Status Colors
   */
  const connectionColors = computed(() => {
    void theme.value
    return {
      connected: resolveColor('oklch(var(--su))'),
      connecting: resolveColor('oklch(var(--wa))'),
      reconnecting: resolveColor('oklch(var(--wa))'),
      disconnected: resolveColor('oklch(var(--er))'),
    }
  })

  /**
   * Chart Styling
   */
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

  // --- Helpers ---

  function getChartColor(index: number): string {
    const map = [
      resolveColor('oklch(var(--p))'),
      resolveColor('oklch(var(--s))'),
      resolveColor('oklch(var(--a))'),
      resolveColor('oklch(var(--n))'),
      resolveColor('oklch(var(--su))'),
      resolveColor('oklch(var(--wa))'),
      resolveColor('oklch(var(--er))'),
      resolveColor('oklch(var(--in))')
    ]
    return map[(index - 1) % map.length]
  }

  function getChartColorArray(count: number): string[] {
    const colors: string[] = []
    for (let i = 1; i <= count; i++) {
      colors.push(getChartColor(i))
    }
    return colors
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

  // Legacy helper compatibility
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
    chartColors,
    semanticColors,
    thresholdColors,
    statusColors,
    connectionColors,
    chartStyling,
    baseColors,
    getChartColor,
    getChartColorArray,
    getThresholdColor,
    getConnectionColor,
    hexToRgba,
    getColorWithAlpha,
  }
}
