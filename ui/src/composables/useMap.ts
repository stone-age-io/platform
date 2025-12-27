import { ref, onUnmounted, nextTick } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { Location } from '@/types/pocketbase'

// Theme-aware tile providers
const TILE_URLS = {
  light: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
  dark: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
}

const TILE_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'

export function useMap() {
  const map = ref<L.Map | null>(null)
  const markersLayer = ref<L.LayerGroup | null>(null)
  const currentTileLayer = ref<L.TileLayer | null>(null)
  const mapInitialized = ref(false)

  /**
   * Fixes missing Leaflet markers in Vite/Webpack builds
   */
  const fixLeafletIcons = () => {
    // @ts-ignore
    delete L.Icon.Default.prototype._getIconUrl
    L.Icon.Default.mergeOptions({
      iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
      iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
      shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
    })
  }

  /**
   * Initialize Map
   */
  const initMap = (containerId: string, isDarkMode: boolean) => {
    if (map.value) return

    fixLeafletIcons()

    // 1. Create Map Instance
    map.value = L.map(containerId, {
      center: [39.8283, -98.5795], // Default US Center
      zoom: 4,
      zoomControl: false, // We'll add it manually to position it better if needed
    })

    L.control.zoom({ position: 'topright' }).addTo(map.value)

    // 2. Add Tile Layer
    updateTheme(isDarkMode)

    // 3. Create Layer Group for Markers
    markersLayer.value = L.layerGroup().addTo(map.value)

    // 4. Handle resize events
    window.addEventListener('resize', handleResize)
    
    // 5. Force a resize check after a tick to ensure container dimension is picked up
    nextTick(() => {
      map.value?.invalidateSize()
    })

    mapInitialized.value = true
  }

  /**
   * Update Tiles based on Theme
   */
  const updateTheme = (isDarkMode: boolean) => {
    if (!map.value) return

    if (currentTileLayer.value) {
      map.value.removeLayer(currentTileLayer.value)
    }

    const url = isDarkMode ? TILE_URLS.dark : TILE_URLS.light
    
    currentTileLayer.value = L.tileLayer(url, {
      attribution: TILE_ATTRIBUTION,
      maxZoom: 19
    }).addTo(map.value)
  }

  /**
   * Render Markers from PocketBase Locations
   */
  const renderMarkers = (locations: Location[]) => {
    if (!map.value || !markersLayer.value) return

    markersLayer.value.clearLayers()
    const validBounds: L.LatLngExpression[] = []

    locations.forEach(loc => {
      // PocketBase 'geoPoint' field structure check
      // It might be stored directly as coordinates object or raw fields depending on PB version
      // Adapting to standard { lat: number, lon: number } from your types
      if (!loc.coordinates || !loc.coordinates.lat || !loc.coordinates.lon) return

      const { lat, lon } = loc.coordinates
      validBounds.push([lat, lon])

      // Custom marker color based on type (simple hash logic or hardcoded)
      const marker = L.marker([lat, lon], {
        title: loc.name
      })

      // Popup Content
      const popupContent = `
        <div class="p-1">
          <h3 class="font-bold text-sm">${loc.name}</h3>
          <div class="text-xs text-gray-500 mb-1">${loc.expand?.type?.name || 'Unknown Type'}</div>
          ${loc.description ? `<p class="text-xs mb-2">${loc.description}</p>` : ''}
          <a href="/locations/${loc.id}" class="text-xs text-primary hover:underline">View Details</a>
        </div>
      `
      marker.bindPopup(popupContent)
      markersLayer.value!.addLayer(marker)
    })

    // Fit bounds if we have markers
    if (validBounds.length > 0) {
      map.value.fitBounds(validBounds, { padding: [50, 50], maxZoom: 15 })
    }
  }

  const handleResize = () => {
    map.value?.invalidateSize()
  }

  const cleanup = () => {
    window.removeEventListener('resize', handleResize)
    if (map.value) {
      map.value.remove()
      map.value = null
    }
  }

  onUnmounted(() => {
    cleanup()
  })

  return {
    map,
    initMap,
    renderMarkers,
    updateTheme,
    cleanup
  }
}
