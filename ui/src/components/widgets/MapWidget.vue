<template>
  <div class="map-widget">
    <!-- Map container -->
    <div 
      :id="mapContainerId" 
      class="map-container"
      @click="handleMapClick"
    />
    
    <!-- Loading overlay -->
    <div v-if="!mapReady" class="map-loading">
      <div class="loading-spinner"></div>
      <span>Loading map...</span>
    </div>
    
    <!-- No markers hint -->
    <div v-if="mapReady && markers.length === 0" class="no-markers-hint">
      <span class="hint-icon">📍</span>
      <span class="hint-text">No markers configured</span>
    </div>
    
    <!-- Map Controls -->
    <div v-if="mapReady && markers.length > 1" class="map-controls">
      <button 
        class="map-control-btn" 
        @click="handleFitAll"
        title="Fit all markers"
      >
        ⊡
      </button>
    </div>
    
    <!-- Marker Detail Panel -->
    <MarkerDetailPanel
      v-if="selectedMarker"
      :marker="selectedMarker"
      :is-mobile="isMobile"
      @close="closePanel"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useLeafletMap } from '@/composables/useLeafletMap'
import { useUIStore } from '@/stores/ui' // CHANGED
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
import MarkerDetailPanel from './map/MarkerDetailPanel.vue'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  isFullscreen?: boolean
}>(), {
  isFullscreen: false
})

const uiStore = useUIStore() // CHANGED
const dataStore = useWidgetDataStore()
const dashboardStore = useDashboardStore()

const { 
  initMap, 
  updateTheme, 
  renderMarkers, 
  setSelectedMarker,
  updateMarkerPositions,
  fitAllMarkers,
  invalidateSize, 
  cleanup 
} = useLeafletMap()

const mapContainerId = computed(() => {
  const suffix = props.isFullscreen ? '-fullscreen' : ''
  return `map-${props.config.id}${suffix}`
})

const mapReady = ref(false)
const selectedMarkerId = ref<string | null>(null)
const isMobile = ref(false)

const markers = computed(() => props.config.mapConfig?.markers || [])
const mapCenter = computed(() => props.config.mapConfig?.center || { lat: 39.8283, lon: -98.5795 })
const mapZoom = computed(() => props.config.mapConfig?.zoom || 4)

const buffer = computed(() => dataStore.getBuffer(props.config.id))

const selectedMarker = computed(() => {
  if (!selectedMarkerId.value) return null
  return markers.value.find(m => m.id === selectedMarkerId.value) || null
})

let initTimeout: number | null = null

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

async function initializeMap() {
  if (initTimeout) {
    clearTimeout(initTimeout)
    initTimeout = null
  }

  await nextTick()
  
  initTimeout = window.setTimeout(() => {
    const container = document.getElementById(mapContainerId.value)
    if (!container) return

    initMap(
      mapContainerId.value,
      mapCenter.value,
      mapZoom.value,
      uiStore.theme === 'dark' // CHANGED
    )
    
    renderMarkers(markers.value, handleMarkerClick)
    mapReady.value = true
    initTimeout = null
    
    if (buffer.value.length > 0) {
      updateMarkerPositions(markers.value, buffer.value, dashboardStore.currentVariableValues)
    }
  }, 50)
}

function handleMarkerClick(markerId: string) {
  if (selectedMarkerId.value === markerId) {
    closePanel()
    return
  }
  
  selectedMarkerId.value = markerId
  setSelectedMarker(markerId)
  
  if (!isMobile.value) {
    nextTick(() => {
      invalidateSize()
    })
  }
}

function handleMapClick(event: MouseEvent) {
  if (isMobile.value) return
  if (!selectedMarkerId.value) return
  
  const target = event.target as HTMLElement
  const isMapClick = target.closest('.leaflet-container') && 
                     !target.closest('.leaflet-marker-icon') &&
                     !target.closest('.marker-detail-panel')
  
  if (isMapClick) {
    closePanel()
  }
}

function handleFitAll() {
  fitAllMarkers()
}

function closePanel() {
  selectedMarkerId.value = null
  setSelectedMarker(null)
  
  if (!isMobile.value) {
    nextTick(() => {
      invalidateSize()
    })
  }
}

function updateMarkers() {
  if (!mapReady.value) return
  
  renderMarkers(markers.value, handleMarkerClick)
  
  if (selectedMarkerId.value) {
    const stillExists = markers.value.some(m => m.id === selectedMarkerId.value)
    if (stillExists) {
      setSelectedMarker(selectedMarkerId.value)
    } else {
      closePanel()
    }
  }
  
  if (buffer.value.length > 0) {
    updateMarkerPositions(markers.value, buffer.value, dashboardStore.currentVariableValues)
  }
}

let resizeObserver: ResizeObserver | null = null
let resizeObserverTimeout: number | null = null

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  
  initializeMap()
  
  resizeObserver = new ResizeObserver(() => {
    if (mapReady.value) invalidateSize()
  })
  
  resizeObserverTimeout = window.setTimeout(() => {
    const container = document.getElementById(mapContainerId.value)
    if (container?.parentElement && resizeObserver) {
      resizeObserver.observe(container.parentElement)
    }
  }, 100)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  
  if (initTimeout) clearTimeout(initTimeout)
  if (resizeObserverTimeout !== null) clearTimeout(resizeObserverTimeout)
  if (resizeObserver) resizeObserver.disconnect()
  
  cleanup()
})

// Watch for theme changes
watch(() => uiStore.theme, (newTheme) => {
  updateTheme(newTheme === 'dark')
})

watch(markers, updateMarkers, { deep: true })

watch([mapCenter, mapZoom], () => {
  if (initTimeout) clearTimeout(initTimeout)
  cleanup()
  mapReady.value = false
  selectedMarkerId.value = null
  initializeMap()
}, { deep: true })

watch(buffer, (newMessages) => {
  if (mapReady.value && newMessages.length > 0) {
    updateMarkerPositions(markers.value, newMessages, dashboardStore.currentVariableValues)
  }
}, { deep: true })

watch(() => dashboardStore.currentVariableValues, () => {
  if (mapReady.value && buffer.value.length > 0) {
    updateMarkerPositions(markers.value, buffer.value, dashboardStore.currentVariableValues)
  }
}, { deep: true })
</script>

<style scoped>
.map-widget {
  height: 100%;
  width: 100%;
  position: relative;
  background: var(--widget-bg);
  border-radius: 4px;
  overflow: hidden;
}

.map-container {
  position: absolute;
  inset: 0;
  z-index: 0;
}

.map-container :deep(.leaflet-pane),
.map-container :deep(.leaflet-control-container) {
  z-index: 0 !important;
}

.map-container :deep(.leaflet-top),
.map-container :deep(.leaflet-bottom) {
  z-index: 1 !important;
}

.map-container :deep(.marker-selected) {
  filter: hue-rotate(180deg) saturate(1.5) drop-shadow(0 0 8px rgba(116, 128, 255, 0.8));
}

.map-loading {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: var(--widget-bg);
  color: var(--muted);
  font-size: 14px;
}

.loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border);
  border-top-color: var(--color-accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.no-markers-hint {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 5;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-size: 12px;
  color: var(--muted);
  pointer-events: none;
}

.hint-icon {
  font-size: 14px;
}

.map-controls {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.map-control-btn {
  width: 32px;
  height: 32px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  font-size: 16px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}

.map-control-btn:hover {
  background: var(--color-info-bg);
  border-color: var(--color-info-border);
}

.map-control-btn:active {
  transform: scale(0.95);
}
</style>
