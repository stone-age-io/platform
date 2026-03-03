<!-- ui/src/components/widgets/MapWidget.vue -->
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
    <div v-if="mapReady && totalMarkerCount === 0 && !dynamicLoading" class="no-markers-hint">
      <span class="hint-icon">📍</span>
      <span class="hint-text">No markers configured</span>
    </div>

    <!-- Dynamic markers loading -->
    <div v-if="mapReady && dynamicLoading" class="no-markers-hint">
      <span class="hint-icon">⏳</span>
      <span class="hint-text">Loading dynamic markers...</span>
    </div>

    <!-- Map Controls -->
    <div v-if="mapReady && totalMarkerCount > 1" class="map-controls">
      <button
        class="map-control-btn"
        @click="handleFitAll"
        title="Fit all markers"
      >
        ⊡
      </button>
    </div>

    <!-- Static Marker Detail Panel -->
    <MarkerDetailPanel
      v-if="selectedStaticMarker"
      :marker="selectedStaticMarker"
      :is-mobile="isMobile"
      @close="closePanel"
    />

    <!-- Dynamic Marker Detail Panel -->
    <DynamicMarkerPanel
      v-if="selectedDynamicRow"
      :row="selectedDynamicRow"
      :popup-fields="dynamicPopupFields"
      :label="selectedDynamicLabel"
      :is-mobile="isMobile"
      @close="closePanel"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useLeafletMap } from '@/composables/useLeafletMap'
import { useNatsKvWatcher, type KvRow } from '@/composables/useNatsKvWatcher'
import { useUIStore } from '@/stores/ui'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
import { resolveTemplate } from '@/utils/variables'
import { JSONPath } from 'jsonpath-plus'
import MarkerDetailPanel from './map/MarkerDetailPanel.vue'
import DynamicMarkerPanel from './map/DynamicMarkerPanel.vue'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  isFullscreen?: boolean
}>(), {
  isFullscreen: false
})

const uiStore = useUIStore()
const dataStore = useWidgetDataStore()
const dashboardStore = useDashboardStore()
const {
  initMap,
  updateTheme,
  renderMarkers,
  updateDynamicMarkers,
  clearDynamicMarkers,
  setSelectedMarker,
  updateMarkerPositions,
  fitAllMarkers,
  invalidateSize,
  getTotalMarkerCount,
  cleanup
} = useLeafletMap()

const mapContainerId = computed(() => {
  const suffix = props.isFullscreen ? '-fullscreen' : ''
  return `map-${props.config.id}${suffix}`
})

const mapReady = ref(false)
const selectedMarkerId = ref<string | null>(null)
const selectedMarkerType = ref<'static' | 'dynamic' | null>(null)
const isMobile = ref(false)

const markers = computed(() => props.config.mapConfig?.markers || [])
const mapCenter = computed(() => props.config.mapConfig?.center || { lat: 39.8283, lon: -98.5795 })
const mapZoom = computed(() => props.config.mapConfig?.zoom || 4)
const enableClustering = computed(() => props.config.mapConfig?.enableClustering ?? false)
const fitBoundsOnLoad = computed(() => props.config.mapConfig?.fitBoundsOnLoad ?? false)

const buffer = computed(() => dataStore.getBuffer(props.config.id))

// --- Dynamic KV markers ---
const dynamicCfg = computed(() => props.config.mapConfig?.dynamicMarkers)
const hasDynamic = computed(() => !!dynamicCfg.value?.kvBucket)

const resolvedDynBucket = computed(() =>
  hasDynamic.value
    ? resolveTemplate(dynamicCfg.value!.kvBucket, dashboardStore.currentVariableValues)
    : ''
)
const resolvedDynPattern = computed(() =>
  hasDynamic.value
    ? resolveTemplate(dynamicCfg.value!.keyPattern, dashboardStore.currentVariableValues)
    : ''
)

const { rows: dynamicRows, loading: dynamicLoading } = useNatsKvWatcher(
  () => resolvedDynBucket.value,
  () => resolvedDynPattern.value
)

const dynamicPopupFields = computed(() => dynamicCfg.value?.popupFields || [])

// --- Selection state ---
const selectedStaticMarker = computed(() => {
  if (selectedMarkerType.value !== 'static' || !selectedMarkerId.value) return null
  return markers.value.find(m => m.id === selectedMarkerId.value) || null
})

const selectedDynamicRow = computed<KvRow | null>(() => {
  if (selectedMarkerType.value !== 'dynamic' || !selectedMarkerId.value) return null
  return dynamicRows.value.get(selectedMarkerId.value) || null
})

const selectedDynamicLabel = computed(() => {
  if (!selectedDynamicRow.value || !dynamicCfg.value) return ''
  return extractDynamicLabel(selectedDynamicRow.value, dynamicCfg.value.labelPath)
})

const totalMarkerCount = computed(() => getTotalMarkerCount())

function extractDynamicLabel(row: KvRow, labelPath: string): string {
  if (labelPath === '__key__') return row.key
  if (labelPath === '__key_suffix__') return row.keySuffix
  if (labelPath === '__revision__') return String(row.revision)
  try {
    const result = JSONPath({ path: labelPath, json: row.data, wrap: false })
    return result != null ? String(result) : row.keySuffix
  } catch {
    return row.keySuffix
  }
}

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

/**
 * Centralized Position Sync for static markers with dynamic position config
 */
function syncMarkerPositions() {
  if (mapReady.value && buffer.value.length > 0) {
    updateMarkerPositions(markers.value, buffer.value, dashboardStore.currentVariableValues)
  }
}

async function initializeMap() {
  await nextTick()

  const tryInit = (attempts = 0) => {
    const container = document.getElementById(mapContainerId.value)

    if (!container || (container.clientHeight === 0 && attempts < 10)) {
      window.setTimeout(() => tryInit(attempts + 1), 50)
      return
    }

    initMap(
      mapContainerId.value,
      mapCenter.value,
      mapZoom.value,
      uiStore.theme === 'dark',
      enableClustering.value
    )

    // Render static markers
    renderMarkers(markers.value, handleStaticMarkerClick)

    mapReady.value = true

    window.setTimeout(() => invalidateSize(), 100)
    window.setTimeout(() => invalidateSize(), 300)
  }

  window.setTimeout(() => tryInit(), 50)
}

// --- Watchers ---

// When map becomes ready, sync static marker positions + handle initial dynamic markers
watch(mapReady, (ready) => {
  if (ready) {
    syncMarkerPositions()
    // If dynamic markers are already loaded, render them
    if (hasDynamic.value && dynamicRows.value.size > 0 && dynamicCfg.value) {
      updateDynamicMarkers(dynamicRows.value, dynamicCfg.value, handleDynamicMarkerClick)
    }
    // Fit bounds on load if configured
    if (fitBoundsOnLoad.value) {
      window.setTimeout(() => fitAllMarkers(), 500)
    }
  }
})

// Live updates for static dynamic-position markers
watch(buffer, () => {
  syncMarkerPositions()
}, { deep: true })

// Dashboard variable changes
watch(() => dashboardStore.currentVariableValues, () => {
  syncMarkerPositions()
}, { deep: true })

// Dynamic KV marker updates
watch(dynamicRows, (newRows) => {
  if (mapReady.value && dynamicCfg.value) {
    updateDynamicMarkers(newRows, dynamicCfg.value, handleDynamicMarkerClick)
  }
}, { deep: true })

// When dynamic config changes (e.g., variable substitution changes bucket/pattern)
watch([resolvedDynBucket, resolvedDynPattern], () => {
  if (mapReady.value) {
    clearDynamicMarkers()
    // Close panel if a dynamic marker was selected
    if (selectedMarkerType.value === 'dynamic') {
      closePanel()
    }
  }
})

// Standard UI watchers
watch(() => uiStore.theme, (newTheme) => updateTheme(newTheme === 'dark'))

watch(markers, () => {
  if (mapReady.value) {
    renderMarkers(markers.value, handleStaticMarkerClick)
    syncMarkerPositions()
  }
}, { deep: true })

// Reinitialize map when center/zoom/clustering changes
watch([mapCenter, mapZoom, enableClustering], () => {
  cleanup()
  mapReady.value = false
  selectedMarkerId.value = null
  selectedMarkerType.value = null
  initializeMap()
}, { deep: true })

// --- Click handlers ---

function handleStaticMarkerClick(markerId: string) {
  if (selectedMarkerId.value === markerId && selectedMarkerType.value === 'static') {
    closePanel()
    return
  }
  selectedMarkerId.value = markerId
  selectedMarkerType.value = 'static'
  setSelectedMarker(markerId)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleDynamicMarkerClick(kvKey: string) {
  if (selectedMarkerId.value === kvKey && selectedMarkerType.value === 'dynamic') {
    closePanel()
    return
  }
  selectedMarkerId.value = kvKey
  selectedMarkerType.value = 'dynamic'
  setSelectedMarker(kvKey)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleMapClick(event: MouseEvent) {
  if (isMobile.value) return
  if (!selectedMarkerId.value) return
  const target = event.target as HTMLElement
  if (target.closest('.leaflet-container') && !target.closest('.leaflet-marker-icon') && !target.closest('.marker-detail-panel')) {
    closePanel()
  }
}

function handleFitAll() { fitAllMarkers() }

function closePanel() {
  selectedMarkerId.value = null
  selectedMarkerType.value = null
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  initializeMap()

  window.setTimeout(() => {
    resizeObserver = new ResizeObserver(() => {
      if (mapReady.value) invalidateSize()
    })

    const container = document.getElementById(mapContainerId.value)
    if (container?.parentElement) resizeObserver.observe(container.parentElement)
  }, 100)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  if (resizeObserver) resizeObserver.disconnect()
  cleanup()
})
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
.map-container { position: absolute; inset: 0; z-index: 0; }
.map-container :deep(.leaflet-pane), .map-container :deep(.leaflet-control-container) { z-index: 0 !important; }
.map-container :deep(.leaflet-top), .map-container :deep(.leaflet-bottom) { z-index: 1 !important; }
.map-container :deep(.marker-selected) { filter: hue-rotate(180deg) saturate(1.5) drop-shadow(0 0 8px rgba(116, 128, 255, 0.8)); }
.map-loading { position: absolute; inset: 0; z-index: 10; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; background: var(--widget-bg); color: var(--muted); font-size: 14px; }
.loading-spinner { width: 32px; height: 32px; border: 3px solid var(--border); border-top-color: var(--color-accent); border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.no-markers-hint { position: absolute; bottom: 12px; left: 50%; transform: translateX(-50%); z-index: 5; display: flex; align-items: center; gap: 6px; padding: 6px 12px; background: var(--panel); border: 1px solid var(--border); border-radius: 4px; font-size: 12px; color: var(--muted); pointer-events: none; }
.hint-icon { font-size: 14px; }
.map-controls { position: absolute; top: 10px; right: 10px; z-index: 5; display: flex; flex-direction: column; gap: 6px; }
.map-control-btn { width: 32px; height: 32px; background: var(--panel); border: 1px solid var(--border); border-radius: 4px; color: var(--text); font-size: 16px; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s; box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2); }
.map-control-btn:hover { background: var(--color-info-bg); border-color: var(--color-info-border); }
.map-control-btn:active { transform: scale(0.95); }
</style>
