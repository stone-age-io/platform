import { ref, shallowRef, onUnmounted, nextTick } from 'vue'
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
  // Use shallowRef for external library instances to prevent deep reactivity overhead and type mismatches
  const map = shallowRef<L.Map | null>(null)
  const markersLayer = shallowRef<L.LayerGroup | null>(null)
  const currentTileLayer = shallowRef<L.TileLayer | null>(null)
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
    const mapInstance = L.map(containerId, {
      center: [39.8283, -98.5795], // Default US Center
      zoom: 4,
      zoomControl: false, 
    })

    // Store in shallow ref
    map.value = mapInstance

    L.control.zoom({ position: 'topright' }).addTo(mapInstance)

    // 2. Add Tile Layer
    updateTheme(isDarkMode)

    // 3. Create Layer Group for Markers
    const layerGroup = L.layerGroup()
    layerGroup.addTo(mapInstance)
    markersLayer.value = layerGroup

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
    
    const tileLayer = L.tileLayer(url, {
      attribution: TILE_ATTRIBUTION,
      maxZoom: 19
    })
    
    tileLayer.addTo(map.value)
    currentTileLayer.value = tileLayer
  }

  /**
   * Render Markers from PocketBase Locations
   */
  const renderMarkers = (locations: Location[]) => {
    if (!map.value || !markersLayer.value) return

    markersLayer.value.clearLayers()
    
    // Explicitly type as tuple array for fitBounds
    const validBounds: [number, number][] = []

    locations.forEach(loc => {
      if (!loc.coordinates || !loc.coordinates.lat || !loc.coordinates.lon) return

      const { lat, lon } = loc.coordinates
      validBounds.push([lat, lon])

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
