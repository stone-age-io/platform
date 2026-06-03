import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
import type { Thing } from '@/types/pocketbase'

export interface RenderOptions {
  draggable: boolean
  onMove: (id: string, x: number, y: number) => void
  onClick: (id: string) => void
}

/**
 * Floor-plan map composable: a Leaflet map in CRS.Simple (pixel) space with an
 * image overlay. Mirrors the marker bookkeeping/selection of useLeafletMap, but
 * kept separate because it renders an image instead of geographic tiles.
 */
export function useFloorPlan() {
  const map = shallowRef<L.Map | null>(null)
  const markerLayer = shallowRef<L.LayerGroup | null>(null)
  const mapInitialized = ref(false)

  const markerInstances = new Map<string, L.Marker>()
  let selectedId: string | null = null

  const initFloorPlan = (containerId: string, imageUrl: string, width: number, height: number) => {
    if (map.value) map.value.remove()
    markerInstances.clear()
    selectedId = null

    map.value = L.map(containerId, {
      crs: L.CRS.Simple, // pixel coordinate space (1 unit = 1 image pixel)
      minZoom: -2,
      maxZoom: 2,
      attributionControl: false,
    })

    const imageBounds = L.latLngBounds(L.latLng(0, 0), L.latLng(height, width))
    L.imageOverlay(imageUrl, imageBounds).addTo(map.value)
    markerLayer.value = L.layerGroup().addTo(map.value)
    map.value.fitBounds(imageBounds)
    mapInitialized.value = true
  }

  // Renders only placed things (those with a floorplan_position). Unplaced things
  // live in the positioning drawer until explicitly placed.
  const renderMarkers = (things: Thing[], opts: RenderOptions) => {
    if (!markerLayer.value || !map.value) return
    markerLayer.value.clearLayers()
    markerInstances.clear()

    things.forEach(thing => {
      const pos = thing.floorplan_position
      if (!pos || typeof pos.x !== 'number' || typeof pos.y !== 'number') return

      // Leaflet uses [lat, lng] = [y, x] in CRS.Simple.
      const marker = L.marker([pos.y, pos.x], {
        draggable: opts.draggable,
        title: thing.name,
      })

      marker.on('click', () => opts.onClick(thing.id))
      if (opts.draggable) {
        marker.on('dragend', (e) => {
          const { lat, lng } = e.target.getLatLng()
          opts.onMove(thing.id, Math.round(lng), Math.round(lat))
        })
      }

      markerInstances.set(thing.id, marker)
      marker.addTo(markerLayer.value!)
    })

    applySelection(selectedId)
  }

  const applySelection = (id: string | null) => {
    markerInstances.forEach(m => m.getElement()?.classList.remove('marker-selected'))
    if (id) markerInstances.get(id)?.getElement()?.classList.add('marker-selected')
  }

  const setSelected = (id: string | null) => {
    selectedId = id
    applySelection(id)
  }

  // Current viewport center as floor-plan pixel coords — where a newly placed marker drops.
  const getViewCenter = (): { x: number; y: number } => {
    const c = map.value?.getCenter()
    return { x: Math.round(c?.lng ?? 0), y: Math.round(c?.lat ?? 0) }
  }

  const invalidateSize = () => map.value?.invalidateSize()

  const cleanup = () => {
    if (map.value) {
      map.value.remove()
      map.value = null
    }
    markerLayer.value = null
    markerInstances.clear()
    selectedId = null
    mapInitialized.value = false
  }

  onUnmounted(cleanup)

  return { map, mapInitialized, initFloorPlan, renderMarkers, setSelected, getViewCenter, invalidateSize, cleanup }
}
