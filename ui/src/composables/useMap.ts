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
  const map = shallowRef<L.Map | null>(null)
  const markersLayer = shallowRef<L.LayerGroup | null>(null)
  const currentTileLayer = shallowRef<L.TileLayer | null>(null)
  const mapInitialized = ref(false)
  
  // NEW: Store bounds to allow resetting view
  const markersBounds = shallowRef<L.LatLngBounds | null>(null)

  const fixLeafletIcons = () => {
    // @ts-ignore
    delete L.Icon.Default.prototype._getIconUrl
    L.Icon.Default.mergeOptions({
      iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
      iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
      shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
    })
  }

  const initMap = (containerId: string, isDarkMode: boolean) => {
    if (map.value) return

    fixLeafletIcons()

    const mapInstance = L.map(containerId, {
      center: [39.8283, -98.5795],
      zoom: 4,
      zoomControl: false, // We usually hide this or move it, standard is TopRight
    })

    map.value = mapInstance

    // Add zoom control explicitly if not present
    L.control.zoom({ position: 'topright' }).addTo(mapInstance)

    updateTheme(isDarkMode)

    const layerGroup = L.layerGroup()
    layerGroup.addTo(mapInstance)
    markersLayer.value = layerGroup

    window.addEventListener('resize', handleResize)
    
    nextTick(() => {
      map.value?.invalidateSize()
    })

    mapInitialized.value = true
  }

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

  const renderMarkers = (locations: Location[]) => {
    if (!map.value || !markersLayer.value) return

    markersLayer.value.clearLayers()
    
    // Collect coordinates
    const latLngs: [number, number][] = []

    locations.forEach(loc => {
      if (!loc.coordinates || !loc.coordinates.lat || !loc.coordinates.lon) return

      const { lat, lon } = loc.coordinates
      latLngs.push([lat, lon])

      const marker = L.marker([lat, lon], { title: loc.name })

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

    // Calculate and store bounds
    if (latLngs.length > 0) {
      const bounds = L.latLngBounds(latLngs)
      markersBounds.value = bounds
      fitToMarkers()
    } else {
      markersBounds.value = null
    }
  }

  // NEW: Fit map to stored bounds
  const fitToMarkers = () => {
    if (map.value && markersBounds.value) {
      map.value.fitBounds(markersBounds.value, { padding: [50, 50], maxZoom: 15 })
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
    markersBounds.value = null
  }

  onUnmounted(() => {
    cleanup()
  })

  return {
    map,
    initMap,
    renderMarkers,
    updateTheme,
    fitToMarkers, // Exported
    cleanup
  }
}
