import { computed } from 'vue'
import { useUIStore } from '@/stores/ui'

export function useDesignTokens() {
  const uiStore = useUIStore()
  const theme = computed(() => uiStore.theme)

  const baseColors = computed(() => {
    void theme.value // Force dependency
    return {
      bg: 'oklch(var(--b2))',
      panel: 'oklch(var(--b1))',
      border: 'oklch(var(--b3))',
      inputBg: 'oklch(var(--b2))',
      text: 'oklch(var(--bc))',
      textSecondary: 'oklch(var(--bc) / 0.7)',
      muted: 'oklch(var(--bc) / 0.5)',
    }
  })

  const semanticColors = computed(() => {
    void theme.value // Force dependency
    return {
      primary: 'oklch(var(--p))',
      primaryHover: 'oklch(var(--pf))',
      primaryActive: 'oklch(var(--p))',
      secondary: 'oklch(var(--s))',
      secondaryHover: 'oklch(var(--sf))',
      secondaryActive: 'oklch(var(--s))',
      accent: 'oklch(var(--a))',
      accentHover: 'oklch(var(--af))',
      success: 'oklch(var(--su))',
      successBg: 'oklch(var(--su) / 0.1)',
      successBorder: 'oklch(var(--su) / 0.3)',
      warning: 'oklch(var(--wa))',
      warningBg: 'oklch(var(--wa) / 0.1)',
      warningBorder: 'oklch(var(--wa) / 0.3)',
      error: 'oklch(var(--er))',
      errorBg: 'oklch(var(--er) / 0.1)',
      errorBorder: 'oklch(var(--er) / 0.3)',
      info: 'oklch(var(--in))',
      infoBg: 'oklch(var(--in) / 0.1)',
      infoBorder: 'oklch(var(--in) / 0.3)',
    }
  })

  const chartColors = computed(() => {
    void theme.value // Force dependency
    return {
      color1: 'oklch(var(--p))',
      color2: 'oklch(var(--s))',
      color3: 'oklch(var(--a))',
      color4: 'oklch(var(--n))',
      color5: 'oklch(var(--su))',
      color6: 'oklch(var(--wa))',
      color7: 'oklch(var(--er))',
      color8: 'oklch(var(--in))',
      color1Alpha30: 'oklch(var(--p) / 0.3)',
      color1Alpha05: 'oklch(var(--p) / 0.05)',
      color2Alpha30: 'oklch(var(--s) / 0.3)',
      color2Alpha05: 'oklch(var(--s) / 0.05)',
      color3Alpha30: 'oklch(var(--a) / 0.3)',
      color3Alpha05: 'oklch(var(--a) / 0.05)',
    }
  })

  const thresholdColors = computed(() => {
    void theme.value // Force dependency
    return {
      normal: 'oklch(var(--su))',
      normalBg: 'oklch(var(--su) / 0.1)',
      warning: 'oklch(var(--wa))',
      warningBg: 'oklch(var(--wa) / 0.1)',
      critical: 'oklch(var(--er))',
      criticalBg: 'oklch(var(--er) / 0.1)',
      severe: '#b91c1c',
      severeBg: 'rgba(185, 28, 28, 0.1)',
    }
  })

  const statusColors = computed(() => {
    void theme.value // Force dependency
    return {
      active: 'oklch(var(--su))',
      inactive: 'oklch(var(--n))',
      pending: 'oklch(var(--wa))',
      error: 'oklch(var(--er))',
      unknown: 'oklch(var(--bc) / 0.5)',
    }
  })

  const connectionColors = computed(() => {
    void theme.value // Force dependency
    return {
      connected: 'oklch(var(--su))',
      connecting: 'oklch(var(--wa))',
      reconnecting: 'oklch(var(--wa))',
      disconnected: 'oklch(var(--er))',
    }
  })

  const chartStyling = computed(() => {
    void theme.value // Force dependency
    return {
      grid: 'oklch(var(--b3))',
      axis: 'oklch(var(--bc) / 0.6)',
      tooltipBg: 'oklch(var(--b1) / 0.95)',
      tooltipBorder: 'oklch(var(--b3))',
      text: 'oklch(var(--bc))',
      textSecondary: 'oklch(var(--bc) / 0.7)',
      muted: 'oklch(var(--bc) / 0.5)',
      border: 'oklch(var(--b3))',
    }
  })

  function getChartColor(index: number): string {
    const map = [
      'oklch(var(--p))', 'oklch(var(--s))', 'oklch(var(--a))', 'oklch(var(--n))',
      'oklch(var(--su))', 'oklch(var(--wa))', 'oklch(var(--er))', 'oklch(var(--in))'
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

  function hexToRgba(hex: string, alpha: number): string {
    if (hex.startsWith('oklch') || hex.startsWith('var')) return hex 
    hex = hex.replace('#', '')
    const r = parseInt(hex.substring(0, 2), 16)
    const g = parseInt(hex.substring(2, 4), 16)
    const b = parseInt(hex.substring(4, 6), 16)
    return `rgba(${r}, ${g}, ${b}, ${alpha})`
  }

  function getColorWithAlpha(color: string, alpha: number): string {
    if (color.startsWith('oklch')) {
      if (color.includes('/')) return color.replace(/\/[^)]+\)/, `/ ${alpha})`)
      return color.replace(')', ` / ${alpha})`)
    }
    return hexToRgba(color, alpha)
  }

  function getToken(tokenName: string): string {
    return getComputedStyle(document.documentElement).getPropertyValue(tokenName).trim()
  }

  return {
    getToken, chartColors, semanticColors, thresholdColors, statusColors,
    connectionColors, chartStyling, baseColors, getChartColor, getChartColorArray,
    getThresholdColor, getConnectionColor, hexToRgba, getColorWithAlpha,
  }
}
