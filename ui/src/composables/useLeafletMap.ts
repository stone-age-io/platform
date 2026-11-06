import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { setWorkerUrl } from 'maplibre-gl'
// maplibre-gl 6 loads its tile worker as a separate file, relative to its own
// module URL, which Vite never emits. Without setWorkerUrl() below the map is a
// flat sheet of colour with no error.
//
// `?worker&url`, not `?url`: the worker imports `./maplibre-gl-shared.mjs`, and
// `?url` copies the file alone, so that import 404s just as silently.
// `?worker&url` bundles the worker with its imports into one emitted file.
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import 'maplibre-gl/dist/maplibre-gl.css'
// Side-effect import: registers L.maplibreGL and augments the leaflet module types.
import '@maplibre/maplibre-gl-leaflet'
import 'leaflet.markercluster'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import 'leaflet.markercluster/dist/MarkerCluster.Default.css'
import type { MapMarker, DynamicMarkerSource } from '@/types/dashboard'
import { MAP_LIMITS } from '@/types/dashboard'
import type { KvRow } from '@/composables/useNatsKvWatcher'
import { JSONPath } from 'jsonpath-plus'
import { resolveTemplate } from '@/utils/variables'
import { fixLeafletIcons } from '@/utils/leafletIcons'
import { textElement } from '@/utils/leafletText'
import type { BufferedMessage } from '@/stores/widgetData'

/**
 * Leaflet map for list/detail views, dashboard widgets (clustered, KV-driven
 * markers) and cluster-click drawers.
 */

/**
 * OpenFreeMap basemap styles: keyless, commercial use allowed, self-hostable.
 * They are MapLibre style documents, not raster templates, so the basemap is
 * an L.maplibreGL WebGL layer under ordinary Leaflet markers and needs WebGL.
 *
 * Do not add a brightness filter for dark mode in assets/main.css: fiord is
 * already dark, and the GL canvas sits in .leaflet-tile-pane.
 */
const STYLE_URLS = {
  light: 'https://tiles.openfreemap.org/styles/bright',
  dark: 'https://tiles.openfreemap.org/styles/fiord'
}

/**
 * ODbL requires a credit and OpenFreeMap's styles carry none, so it is added to
 * the Leaflet attribution control at init. Kept short, and assets/main.css
 * truncates it to one line, so it cannot cover the zoom control in a widget.
 */
const TILE_ATTRIBUTION = '&copy; <a href="https://openfreemap.org">OpenFreeMap</a> &middot; <a href="https://www.openmaptiles.org/">OpenMapTiles</a> &middot; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'

const DEFAULT_CENTER = { lat: 39.8283, lon: -98.5795 }
const DEFAULT_ZOOM = 4

// Never hand Leaflet a string (it uses innerHTML): labels go through
// textElement (utils/leafletText.ts) and a popup is a caller-built element.
// Deliberately no `popupHtml` option.
export interface MapMarkerInput {
  id: string
  lat: number
  lon: number
  label?: string
  popup?: HTMLElement
}

export type ZoomControlPosition = 'topleft' | 'topright' | 'bottomleft' | 'bottomright' | 'none'

export interface InitMapOptions {
  isDarkMode: boolean
  center?: { lat: number; lon: number }
  zoom?: number
  enableClustering?: boolean
  // Forces clustering on, suppresses zoom/spiderfy, and receives the child marker ids.
  onClusterClick?: (markerIds: string[]) => void
  // Default 'topleft'; 'none' hides it.
  zoomControlPosition?: ZoomControlPosition
}

export interface FitOptions {
  padding?: number
  maxZoom?: number
}

// Load-bearing (see the import note). Once per module: it is a global, read when
// MapLibre starts its worker pool, and this is the only MapLibre caller.
setWorkerUrl(maplibreWorkerUrl)

export function useLeafletMap() {
  const map = shallowRef<L.Map | null>(null)
  const markersLayer = shallowRef<L.LayerGroup | null>(null)
  const basemapLayer = shallowRef<L.MaplibreGL | null>(null)
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
      // The GL layer supplies no zoom bounds; without this zoom is unlimited.
      maxZoom: 19,
      zoomControl: false,
      attributionControl: true,
    })

    // Drop the "Leaflet" prefix: widgets cannot spare the width, and only the
    // data credit is required.
    mapInstance.attributionControl.setPrefix(false)
    mapInstance.attributionControl.addAttribution(TILE_ATTRIBUTION)

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

    if (basemapLayer.value) {
      map.value.removeLayer(basemapLayer.value)
    }

    // Security: STYLE_URLS must stay constants. The binding copies a style's
    // source `attribution` into Leaflet's attribution control via innerHTML,
    // unsanitized, so a configurable style URL (e.g. a self-hosted tile server)
    // is a script-injection path. Sanitize before making it configurable.
    //
    // attributionControl: false hides MapLibre's own credit so only
    // TILE_ATTRIBUTION shows. It does not protect the innerHTML path above.
    const newLayer = L.maplibreGL({
      style: isDarkMode ? STYLE_URLS.dark : STYLE_URLS.light,
      attributionControl: false,
    })
    newLayer.addTo(map.value)
    basemapLayer.value = newLayer
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

    markers.forEach(({ id, lat, lon, label, popup }) => {
      const marker = L.marker([lat, lon], { title: label })
      ;(marker as any).__rrId = id

      if (onMarkerClick) {
        marker.on('click', () => onMarkerClick(id))
      } else if (popup) {
        marker.bindPopup(popup)
      }

      if (label) {
        marker.bindTooltip(textElement(label), {
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

  /** Diffs KV-watcher rows against the dynamic markers: add, remove, move. */
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
        existing.setTooltipContent(textElement(label))
      } else {
        const marker = L.marker([lat, lon], { title: label })
        ;(marker as any).__rrId = key
        marker.on('click', () => onMarkerClick(key))
        marker.bindTooltip(textElement(label), {
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

  /** Moves configured markers with a dynamic position to their latest matching message. */
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

  /** Fits the view to all markers; false if there is nothing to fit. */
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
    basemapLayer.value = null
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
