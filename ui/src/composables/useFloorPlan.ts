import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
// Required: without Leaflet's stylesheet the panes stack in normal flow and the
// markers land below the image, out of sight. leafletCss.spec.ts pins it.
import 'leaflet/dist/leaflet.css'
import type { Thing } from '@/types/pocketbase'
import { fixLeafletIcons } from '@/utils/leafletIcons'
import { textElement } from '@/utils/leafletText'
import { isPlaced } from '@/utils/floorplanBindings'

export interface RenderOptions {
  draggable: boolean
  onMove: (id: string, x: number, y: number) => void
  onClick: (id: string) => void
  /** This Thing's icon. Absent keeps Leaflet's default pin (the Location page). */
  icon?: (thing: Thing) => L.DivIcon | undefined
  /** Tooltip text. Always set through textElement, never as an HTML string. */
  tooltip?: (thing: Thing) => string | undefined
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
  // Fit-to-plan fits the image, not the markers: a plan is a fixed sheet.
  let imageBounds: L.LatLngBounds | null = null

  // Detach handlers and drop all markers — used on re-render, re-init, and cleanup.
  // off() releases the click/dragend closures (which capture the parent's emit).
  const clearMarkers = () => {
    markerInstances.forEach(m => m.off())
    markerLayer.value?.clearLayers()
    markerInstances.clear()
  }

  const initFloorPlan = (containerId: string, imageUrl: string, width: number, height: number) => {
    clearMarkers()
    if (map.value) map.value.remove()
    selectedId = null

    fixLeafletIcons() // ensure default marker icons resolve (broken on iOS otherwise)

    map.value = L.map(containerId, {
      crs: L.CRS.Simple, // pixel coordinate space (1 unit = 1 image pixel)
      minZoom: -2,
      maxZoom: 2,
      attributionControl: false,
    })

    imageBounds = L.latLngBounds(L.latLng(0, 0), L.latLng(height, width))
    L.imageOverlay(imageUrl, imageBounds).addTo(map.value)
    markerLayer.value = L.layerGroup().addTo(map.value)
    map.value.fitBounds(imageBounds)
    mapInitialized.value = true
  }

  // Renders only placed things (those with a floorplan_position). Unplaced things
  // live in the positioning drawer until explicitly placed.
  const renderMarkers = (things: Thing[], opts: RenderOptions) => {
    if (!markerLayer.value || !map.value) return
    clearMarkers()

    things.forEach(thing => {
      if (!isPlaced(thing)) return
      const pos = thing.floorplan_position

      // Leaflet uses [lat, lng] = [y, x] in CRS.Simple.
      const icon = opts.icon?.(thing)
      const marker = L.marker([pos.y, pos.x], {
        draggable: opts.draggable,
        title: thing.name,
        ...(icon ? { icon } : {}),
      })

      const tip = opts.tooltip?.(thing)
      if (tip) {
        marker.bindTooltip(textElement(tip), { direction: 'top', offset: [0, -12] })
      }

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

  /**
   * Paint one marker with a live state: the colour as an inline style (the SVG
   * uses currentColor), blink and stale as `fp-state-*` classes. Kept apart
   * from selection, so only `fp-state-*` classes are swept here.
   */
  const setMarkerState = (
    id: string,
    state: { color?: string; blink?: boolean; stale?: boolean } | null,
  ) => {
    const el = markerInstances.get(id)?.getElement()
    if (!el) return
    el.classList.forEach(c => {
      if (c.startsWith('fp-state-')) el.classList.remove(c)
    })
    el.style.color = state?.color || ''
    if (state?.blink) el.classList.add('fp-state-blink')
    if (state?.stale) el.classList.add('fp-state-stale')
  }

  /** Update one marker's tooltip in place, so it keeps up with its colour. */
  const setMarkerTooltip = (id: string, text: string) => {
    const marker = markerInstances.get(id)
    if (!marker) return
    if (marker.getTooltip()) marker.setTooltipContent(textElement(text))
    else marker.bindTooltip(textElement(text), { direction: 'top', offset: [0, -12] })
  }

  // Current viewport center as floor-plan pixel coords — where a newly placed marker drops.
  const getViewCenter = (): { x: number; y: number } => {
    const c = map.value?.getCenter()
    return { x: Math.round(c?.lng ?? 0), y: Math.round(c?.lat ?? 0) }
  }

  /** Return the view to the whole plan. */
  const fitToImage = () => {
    if (map.value && imageBounds) map.value.fitBounds(imageBounds)
  }
  const invalidateSize = () => map.value?.invalidateSize()

  const cleanup = () => {
    clearMarkers()
    if (map.value) {
      map.value.remove()
      map.value = null
    }
    markerLayer.value = null
    imageBounds = null
    selectedId = null
    mapInitialized.value = false
  }

  onUnmounted(cleanup)

  return {
    map,
    mapInitialized,
    initFloorPlan,
    renderMarkers,
    setSelected,
    setMarkerState,
    setMarkerTooltip,
    fitToImage,
    getViewCenter,
    invalidateSize,
    cleanup,
  }
}
