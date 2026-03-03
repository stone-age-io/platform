// src/composables/useLeafletMap.ts
import { ref, shallowRef } from 'vue'
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
import type { BufferedMessage } from '@/stores/widgetData'

/**
 * Leaflet Map Composable
 *
 * Manages Leaflet map lifecycle:
 * - Map initialization and cleanup
 * - Theme switching (light/dark tiles)
 * - Marker rendering with selection state
 * - Dynamic marker position updates (NATS subject-based)
 * - KV-driven dynamic markers (auto-generated from KV bucket)
 * - Optional marker clustering
 */

// Tile providers
const TILE_URLS = {
  light: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
  dark: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
}

const TILE_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'

// Fix Leaflet default icon paths
function fixLeafletIcons() {
  // @ts-ignore
  delete L.Icon.Default.prototype._getIconUrl
  L.Icon.Default.mergeOptions({
    iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
    iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
    shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  })
}

export function useLeafletMap() {
  const map = shallowRef<L.Map | null>(null)
  const markersLayer = shallowRef<L.LayerGroup | null>(null)
  const tileLayer = shallowRef<L.TileLayer | null>(null)
  const initialized = ref(false)

  // Track marker instances for updates (static markers)
  const markerInstances = new Map<string, L.Marker>()

  // Track dynamic KV marker instances (keyed by KV key)
  const dynamicMarkerInstances = new Map<string, L.Marker>()

  // Whether clustering is enabled for this map instance
  let clusteringEnabled = false

  // Track selected marker ID
  let selectedMarkerId: string | null = null

  /**
   * Initialize the Leaflet map
   */
  function initMap(
    containerId: string,
    center: { lat: number; lon: number },
    zoom: number,
    isDarkMode: boolean,
    enableClustering: boolean = false
  ) {
    // Cleanup existing map
    if (map.value) {
      map.value.remove()
      map.value = null
    }
    markerInstances.clear()
    dynamicMarkerInstances.clear()
    selectedMarkerId = null
    clusteringEnabled = enableClustering

    fixLeafletIcons()

    const container = document.getElementById(containerId)
    if (!container) return

    // Clear any stale Leaflet state
    if ((container as any)._leaflet_id) {
      (container as any)._leaflet_id = null
    }

    const mapInstance = L.map(containerId, {
      center: [center.lat, center.lon],
      zoom: zoom,
      zoomControl: true,
      attributionControl: true,
    })

    map.value = mapInstance
    updateTheme(isDarkMode)

    // Use MarkerClusterGroup when clustering is enabled, otherwise plain LayerGroup
    const layerGroup = clusteringEnabled
      ? (L as any).markerClusterGroup({
          chunkedLoading: true,
          maxClusterRadius: 50,
          spiderfyOnMaxZoom: true,
          showCoverageOnHover: false,
          zoomToBoundsOnClick: true,
        })
      : L.layerGroup()
    layerGroup.addTo(mapInstance)
    markersLayer.value = layerGroup

    initialized.value = true

    // Ensure proper sizing after init
    setTimeout(() => {
      mapInstance.invalidateSize()
    }, 100)
  }

  /**
   * Switch between light and dark tile layers
   */
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

  /**
   * Render static markers on the map
   */
  function renderMarkers(
    markers: MapMarker[],
    onMarkerClick: (markerId: string) => void
  ) {
    if (!map.value || !markersLayer.value) return

    // Remove existing static markers from layer
    markerInstances.forEach((marker) => {
      markersLayer.value!.removeLayer(marker)
    })
    markerInstances.clear()

    markers.forEach(markerConfig => {
      const { id, lat, lon, label } = markerConfig

      const marker = L.marker([lat, lon], {
        title: label
      })

      marker.on('click', () => {
        onMarkerClick(id)
      })

      marker.bindTooltip(label, {
        permanent: false,
        direction: 'top',
        offset: [-15, -15]
      })

      markerInstances.set(id, marker)
      markersLayer.value!.addLayer(marker)
    })

    // Re-apply selection if there was one
    if (selectedMarkerId) {
      applySelectionClass(selectedMarkerId)
    }
  }

  /**
   * Update dynamic KV markers from KV watcher rows.
   * Diffs against existing dynamic markers: adds new, removes deleted, repositions moved.
   */
  function updateDynamicMarkers(
    rows: Map<string, KvRow>,
    source: DynamicMarkerSource,
    onMarkerClick: (kvKey: string) => void
  ) {
    if (!map.value || !markersLayer.value) return

    const currentKeys = new Set(dynamicMarkerInstances.keys())
    let count = 0

    // Add or update markers
    for (const [key, row] of rows) {
      if (count >= MAP_LIMITS.MAX_DYNAMIC_MARKERS) break

      const lat = extractNumericValue(row.data, source.latPath)
      const lon = extractNumericValue(row.data, source.lonPath)
      if (!isValidCoord(lat) || !isValidCoord(lon)) continue

      const label = extractLabel(row, source.labelPath)
      const existing = dynamicMarkerInstances.get(key)

      if (existing) {
        // Update position if moved
        const current = existing.getLatLng()
        if (Math.abs(current.lat - lat) > 0.000001 || Math.abs(current.lng - lon) > 0.000001) {
          existing.setLatLng([lat, lon])
        }
        // Update tooltip if label changed
        existing.setTooltipContent(label)
      } else {
        // Create new marker
        const marker = L.marker([lat, lon], { title: label })
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

    // Remove markers for deleted KV keys
    for (const key of currentKeys) {
      const marker = dynamicMarkerInstances.get(key)
      if (marker) {
        markersLayer.value!.removeLayer(marker)
        dynamicMarkerInstances.delete(key)
      }
    }

    // Re-apply selection
    if (selectedMarkerId) {
      applySelectionClass(selectedMarkerId)
    }
  }

  /**
   * Clear all dynamic markers
   */
  function clearDynamicMarkers() {
    if (!markersLayer.value) return
    dynamicMarkerInstances.forEach((marker) => {
      markersLayer.value!.removeLayer(marker)
    })
    dynamicMarkerInstances.clear()
  }

  /**
   * Extract a label string from a KV row using the configured path
   */
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

  /**
   * Apply selection CSS class to a marker's icon element
   */
  function applySelectionClass(markerId: string | null) {
    // Remove class from all markers first
    const allMarkers = [...markerInstances.values(), ...dynamicMarkerInstances.values()]
    allMarkers.forEach((marker) => {
      const iconEl = marker.getElement()
      if (iconEl) {
        iconEl.classList.remove('marker-selected')
      }
    })

    // Add class to selected marker (check both static and dynamic)
    if (markerId) {
      const marker = markerInstances.get(markerId) || dynamicMarkerInstances.get(markerId)
      if (marker) {
        const iconEl = marker.getElement()
        if (iconEl) {
          iconEl.classList.add('marker-selected')
        }
      }
    }
  }

  /**
   * Update selected marker visual state
   */
  function setSelectedMarker(markerId: string | null) {
    selectedMarkerId = markerId
    applySelectionClass(markerId)
  }

  /**
   * Update marker positions from incoming messages
   * Used for static markers with dynamic position config
   */
  function updateMarkerPositions(
    markersConfig: MapMarker[],
    messages: BufferedMessage[],
    variables: Record<string, string>
  ) {
    if (!map.value || messages.length === 0) return

    // Filter for dynamic markers
    const dynamicMarkers = markersConfig.filter(m =>
      m.positionConfig?.mode === 'dynamic' && m.positionConfig.subject
    )

    if (dynamicMarkers.length === 0) return

    // For each dynamic marker, find the latest relevant message
    dynamicMarkers.forEach(marker => {
      const pos = marker.positionConfig!
      const resolvedSubject = resolveTemplate(pos.subject, variables)

      // Find latest message matching this subject (iterate backwards)
      let match: BufferedMessage | undefined
      for (let i = messages.length - 1; i >= 0; i--) {
        if (messages[i].subject === resolvedSubject) {
          match = messages[i]
          break
        }
      }

      if (match) {
        const lat = extractNumericValue(match.value, pos.latJsonPath)
        const lon = extractNumericValue(match.value, pos.lonJsonPath)

        if (isValidCoord(lat) && isValidCoord(lon)) {
          const leafletMarker = markerInstances.get(marker.id)
          if (leafletMarker) {
            const current = leafletMarker.getLatLng()
            // Only update if moved significantly
            if (Math.abs(current.lat - lat) > 0.000001 || Math.abs(current.lng - lon) > 0.000001) {
              leafletMarker.setLatLng([lat, lon])
            }
          }
        }
      }
    })
  }

  /**
   * Extract numeric value from data using JSONPath
   */
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

  /**
   * Check if a value is a valid coordinate
   */
  function isValidCoord(val: number): boolean {
    return typeof val === 'number' && !isNaN(val)
  }

  /**
   * Pan and zoom to specific location
   */
  function setView(lat: number, lon: number, zoom: number) {
    if (!map.value) return
    map.value.setView([lat, lon], zoom)
  }

  /**
   * Fit map view to show all markers (static + dynamic)
   * Returns false if no markers to fit
   */
  function fitAllMarkers(padding: number = 50): boolean {
    if (!map.value) return false

    const totalMarkers = markerInstances.size + dynamicMarkerInstances.size
    if (totalMarkers === 0) return false

    const bounds = L.latLngBounds([])

    markerInstances.forEach((marker) => {
      bounds.extend(marker.getLatLng())
    })

    dynamicMarkerInstances.forEach((marker) => {
      bounds.extend(marker.getLatLng())
    })

    if (bounds.isValid()) {
      map.value.fitBounds(bounds, { padding: [padding, padding] })
      return true
    }

    return false
  }

  /**
   * Force map to recalculate its size
   * Call after container resize
   */
  function invalidateSize() {
    if (!map.value) return
    map.value.invalidateSize()
  }

  /**
   * Get a marker instance by ID (checks both static and dynamic)
   */
  function getMarker(markerId: string): L.Marker | undefined {
    return markerInstances.get(markerId) || dynamicMarkerInstances.get(markerId)
  }

  /**
   * Get total marker count (static + dynamic)
   */
  function getTotalMarkerCount(): number {
    return markerInstances.size + dynamicMarkerInstances.size
  }

  /**
   * Cleanup map instance
   */
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
    clusteringEnabled = false
    initialized.value = false
  }

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
    setView,
    fitAllMarkers,
    invalidateSize,
    getMarker,
    getTotalMarkerCount,
    cleanup,
  }
}
