// src/composables/useLeafletMap.ts
import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import 'leaflet.markercluster'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import 'leaflet.markercluster/dist/MarkerCluster.Default.css'
import type { MapMarker, DynamicMarkerSource } from '@/types/dashboard'
import { MAP_LIMITS } from '@/types/dashboard'
import type { KvRow } from '@/composables/useNatsKvWatcher'
import { JSONPath } from 'jsonpath-plus'
import { resolveTemplate } from '@/utils/variables'
import { fixLeafletIcons } from '@/utils/leafletIcons'
import type { BufferedMessage } from '@/stores/widgetData'

/**
 * Unified Leaflet map composable.
 *
 * Covers list/detail views (popup or click handler), dashboard widgets
 * (clustered + KV-driven dynamic markers), and cluster-click drawers.
 */

const TILE_URLS = {
  light: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
  dark: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
}

const TILE_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'

const DEFAULT_CENTER = { lat: 39.8283, lon: -98.5795 }
const DEFAULT_ZOOM = 4

export interface MapMarkerInput {
  id: string
  lat: number
  lon: number
  label?: string
  popupHtml?: string
}

export type ZoomControlPosition = 'topleft' | 'topright' | 'bottomleft' | 'bottomright' | 'none'

export interface InitMapOptions {
  isDarkMode: boolean
  center?: { lat: number; lon: number }
  zoom?: number
  enableClustering?: boolean
  // When provided, clustering is forced on, default zoom/spiderfy is suppressed,
  // and the handler receives the child marker ids.
  onClusterClick?: (markerIds: string[]) => void
  // Position of Leaflet's built-in zoom control. 'none' hides it. Default 'topleft'.
  zoomControlPosition?: ZoomControlPosition
}

export interface FitOptions {
  padding?: number
  maxZoom?: number
}

export function useLeafletMap() {
  const map = shallowRef<L.Map | null>(null)
  const markersLayer = shallowRef<L.LayerGroup | null>(null)
  const tileLayer = shallowRef<L.TileLayer | null>(null)
  const initialized = ref(false)

  const markerInstances = new Map<string, L.Marker>()
  const dynamicMarkerInstances = new Map<string, L.Marker>()

  let selectedMarkerId: string | null = null

  function initMap(containerId: string, opts: InitMapOptions) {
    if (map.value) {
      map.value.remove()
      map.value = null
    }
    markerInstances.clear()
    dynamicMarkerInstances.clear()
    selectedMarkerId = null

    fixLeafletIcons()

    const container = document.getElementById(containerId)
    if (!container) return

    if ((container as any)._leaflet_id) {
      (container as any)._leaflet_id = null
    }

    const center = opts.center ?? DEFAULT_CENTER
    const zoom = opts.zoom ?? DEFAULT_ZOOM

    const zoomControlPosition = opts.zoomControlPosition ?? 'topleft'

    const mapInstance = L.map(containerId, {
      center: [center.lat, center.lon],
      zoom,
      zoomControl: false,
      attributionControl: true,
    })

    if (zoomControlPosition !== 'none') {
      L.control.zoom({ position: zoomControlPosition }).addTo(mapInstance)
    }

    map.value = mapInstance
    updateTheme(opts.isDarkMode)

    const useCluster = opts.enableClustering || !!opts.onClusterClick
    const customClusterClick = !!opts.onClusterClick

    const layerGroup = useCluster
      ? (L as any).markerClusterGroup({
          chunkedLoading: true,
          maxClusterRadius: 50,
          spiderfyOnMaxZoom: !customClusterClick,
          showCoverageOnHover: false,
          zoomToBoundsOnClick: !customClusterClick,
        })
      : L.layerGroup()

    if (customClusterClick) {
      layerGroup.on('clusterclick', (e: any) => {
        const children = e.layer.getAllChildMarkers() as L.Marker[]
        const ids = children
          .map(m => (m as any).__rrId as string | undefined)
          .filter((id): id is string => !!id)
        opts.onClusterClick!(ids)
      })
    }

    layerGroup.addTo(mapInstance)
    markersLayer.value = layerGroup

    initialized.value = true

    setTimeout(() => mapInstance.invalidateSize(), 100)
  }

  function updateTheme(isDarkMode: boolean) {
    if (!map.value) return

    if (tileLayer.value) {
      map.value.removeLayer(tileLayer.value)
    }

    const url = isDarkMode ? TILE_URLS.dark : TILE_URLS.light
    const newTileLayer = L.tileLayer(url, {
      attribution: TILE_ATTRIBUTION,
      maxZoom: 19
    })
    newTileLayer.addTo(map.value)
    tileLayer.value = newTileLayer
  }

  function renderMarkers(
    markers: MapMarkerInput[],
    onMarkerClick?: (id: string) => void,
    opts: { fitBounds?: boolean } = {}
  ) {
    if (!map.value || !markersLayer.value) return

    markerInstances.forEach((marker) => {
      markersLayer.value!.removeLayer(marker)
    })
    markerInstances.clear()

    markers.forEach(({ id, lat, lon, label, popupHtml }) => {
      const marker = L.marker([lat, lon], { title: label })
      ;(marker as any).__rrId = id

      if (onMarkerClick) {
        marker.on('click', () => onMarkerClick(id))
      } else if (popupHtml) {
        marker.bindPopup(popupHtml)
      }

      if (label) {
        marker.bindTooltip(label, {
          permanent: false,
          direction: 'top',
          offset: [-15, -15]
        })
      }

      markerInstances.set(id, marker)
      markersLayer.value!.addLayer(marker)
    })

    if (selectedMarkerId) {
      applySelectionClass(selectedMarkerId)
    }

    if (opts.fitBounds) {
      fitAllMarkers()
    }
  }

  /**
   * Diff KV-watcher rows against existing dynamic markers:
   * adds new, removes deleted, repositions moved.
   */
  function updateDynamicMarkers(
    rows: Map<string, KvRow>,
    source: DynamicMarkerSource,
    onMarkerClick: (kvKey: string) => void
  ) {
    if (!map.value || !markersLayer.value) return

    const currentKeys = new Set(dynamicMarkerInstances.keys())
    let count = 0

    for (const [key, row] of rows) {
      if (count >= MAP_LIMITS.MAX_DYNAMIC_MARKERS) break

      const lat = extractNumericValue(row.data, source.latPath)
      const lon = extractNumericValue(row.data, source.lonPath)
      if (!isValidCoord(lat) || !isValidCoord(lon)) continue

      const label = extractLabel(row, source.labelPath)
      const existing = dynamicMarkerInstances.get(key)

      if (existing) {
        const current = existing.getLatLng()
        if (Math.abs(current.lat - lat) > 0.000001 || Math.abs(current.lng - lon) > 0.000001) {
          existing.setLatLng([lat, lon])
        }
        existing.setTooltipContent(label)
      } else {
        const marker = L.marker([lat, lon], { title: label })
        ;(marker as any).__rrId = key
        marker.on('click', () => onMarkerClick(key))
        marker.bindTooltip(label, {
          permanent: false,
          direction: 'top',
          offset: [-15, -15]
        })
        dynamicMarkerInstances.set(key, marker)
        markersLayer.value!.addLayer(marker)
      }

      currentKeys.delete(key)
      count++
    }

    for (const key of currentKeys) {
      const marker = dynamicMarkerInstances.get(key)
      if (marker) {
        markersLayer.value!.removeLayer(marker)
        dynamicMarkerInstances.delete(key)
      }
    }

    if (selectedMarkerId) {
      applySelectionClass(selectedMarkerId)
    }
  }

  function clearDynamicMarkers() {
    if (!markersLayer.value) return
    dynamicMarkerInstances.forEach((marker) => {
      markersLayer.value!.removeLayer(marker)
    })
    dynamicMarkerInstances.clear()
  }

  function extractLabel(row: KvRow, labelPath: string): string {
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

  function applySelectionClass(markerId: string | null) {
    const allMarkers = [...markerInstances.values(), ...dynamicMarkerInstances.values()]
    allMarkers.forEach((marker) => {
      const iconEl = marker.getElement()
      if (iconEl) iconEl.classList.remove('marker-selected')
    })

    if (markerId) {
      const marker = markerInstances.get(markerId) || dynamicMarkerInstances.get(markerId)
      if (marker) {
        const iconEl = marker.getElement()
        if (iconEl) iconEl.classList.add('marker-selected')
      }
    }
  }

  function setSelectedMarker(markerId: string | null) {
    selectedMarkerId = markerId
    applySelectionClass(markerId)
  }

  /**
   * Update positions of static markers configured with dynamic position bindings,
   * using the latest matching message from the widget buffer.
   */
  function updateMarkerPositions(
    markersConfig: MapMarker[],
    messages: BufferedMessage[],
    variables: Record<string, string>
  ) {
    if (!map.value || messages.length === 0) return

    const dynamicMarkers = markersConfig.filter(m =>
      m.positionConfig?.mode === 'dynamic' && m.positionConfig.subject
    )
    if (dynamicMarkers.length === 0) return

    dynamicMarkers.forEach(marker => {
      const pos = marker.positionConfig!
      const resolvedSubject = resolveTemplate(pos.subject, variables)

      let match: BufferedMessage | undefined
      for (let i = messages.length - 1; i >= 0; i--) {
        if (messages[i].subject === resolvedSubject) {
          match = messages[i]
          break
        }
      }
      if (!match) return

      const lat = extractNumericValue(match.value, pos.latJsonPath)
      const lon = extractNumericValue(match.value, pos.lonJsonPath)
      if (!isValidCoord(lat) || !isValidCoord(lon)) return

      const leafletMarker = markerInstances.get(marker.id)
      if (!leafletMarker) return

      const current = leafletMarker.getLatLng()
      if (Math.abs(current.lat - lat) > 0.000001 || Math.abs(current.lng - lon) > 0.000001) {
        leafletMarker.setLatLng([lat, lon])
      }
    })
  }

  function extractNumericValue(data: any, path?: string): number {
    if (!path) return NaN
    try {
      let json = data
      if (typeof data === 'string') {
        try { json = JSON.parse(data) } catch { return NaN }
      }
      const result = JSONPath({ path, json, wrap: false })
      return parseFloat(result)
    } catch {
      return NaN
    }
  }

  function isValidCoord(val: number): boolean {
    return typeof val === 'number' && !isNaN(val)
  }

  /**
   * Fit map view to all current markers (static + dynamic).
   * Returns false if there's nothing to fit.
   */
  function fitAllMarkers(opts: FitOptions = {}): boolean {
    if (!map.value) return false

    const totalMarkers = markerInstances.size + dynamicMarkerInstances.size
    if (totalMarkers === 0) return false

    const bounds = L.latLngBounds([])
    markerInstances.forEach((m) => bounds.extend(m.getLatLng()))
    dynamicMarkerInstances.forEach((m) => bounds.extend(m.getLatLng()))
    if (!bounds.isValid()) return false

    const padding = opts.padding ?? 50
    const maxZoom = opts.maxZoom ?? 15
    map.value.fitBounds(bounds, { padding: [padding, padding], maxZoom })
    return true
  }

  function invalidateSize() {
    map.value?.invalidateSize()
  }

  function cleanup() {
    if (map.value) {
      map.value.remove()
      map.value = null
    }
    markersLayer.value = null
    tileLayer.value = null
    markerInstances.clear()
    dynamicMarkerInstances.clear()
    selectedMarkerId = null
    initialized.value = false
  }

  onUnmounted(cleanup)

  return {
    map,
    initialized,
    initMap,
    updateTheme,
    renderMarkers,
    updateDynamicMarkers,
    clearDynamicMarkers,
    setSelectedMarker,
    updateMarkerPositions,
    fitAllMarkers,
    invalidateSize,
    cleanup,
  }
}
