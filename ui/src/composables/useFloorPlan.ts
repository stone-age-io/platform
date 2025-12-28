import { ref, shallowRef } from 'vue'
import L from 'leaflet'

export function useFloorPlan() {
  const map = shallowRef<L.Map | null>(null)
  const markerLayer = shallowRef<L.LayerGroup | null>(null)
  const mapInitialized = ref(false)
  const bounds = ref<L.LatLngBounds | null>(null)

  const initFloorPlan = (containerId: string, imageUrl: string, width: number, height: number) => {
    if (map.value) map.value.remove()

    map.value = L.map(containerId, {
      crs: L.CRS.Simple, // Crucial for image pixels
      minZoom: -2,
      maxZoom: 2,
      attributionControl: false
    })

    const southWest = L.latLng(0, 0)
    const northEast = L.latLng(height, width)
    const imageBounds = L.latLngBounds(southWest, northEast)
    bounds.value = imageBounds

    L.imageOverlay(imageUrl, imageBounds).addTo(map.value)
    markerLayer.value = L.layerGroup().addTo(map.value)
    
    map.value.fitBounds(imageBounds)
    mapInitialized.value = true
  }

  const renderMarkers = (things: any[], isEditable: boolean, onMove: (id: string, x: number, y: number) => void) => {
    if (!markerLayer.value || !map.value) return
    markerLayer.value.clearLayers()

    things.forEach(thing => {
      // Use coordinates from metadata or center of map
      const x = thing.metadata?.x ?? (bounds.value?.getCenter().lng || 0)
      const y = thing.metadata?.y ?? (bounds.value?.getCenter().lat || 0)

      const marker = L.marker([y, x], {
        draggable: isEditable,
        title: thing.name
      })

      if (isEditable) {
        marker.on('dragend', (e) => {
          const { lat, lng } = e.target.getLatLng()
          onMove(thing.id, Math.round(lng), Math.round(lat))
        })
      }

      marker.bindPopup(`<strong>${thing.name}</strong><br>${thing.expand?.type?.name || ''}`)
      marker.addTo(markerLayer.value!)
    })
  }

  return { map, mapInitialized, initFloorPlan, renderMarkers }
}
