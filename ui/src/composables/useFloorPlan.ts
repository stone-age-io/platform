import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
// Required: without Leaflet's stylesheet the panes stack in normal flow and the
// markers land below the image, out of sight. leafletCss.spec.ts pins it.
import 'leaflet/dist/leaflet.css'
import type { Thing } from '@/types/pocketbase'
import { fixLeafletIcons } from '@/utils/leafletIcons'
import { textElement } from '@/utils/leafletText'
import { isPlaced } from '@/utils/floorplanBindings'

/** Levels below the fitted zoom you can zoom out to. */
const ZOOM_OUT_BELOW_FIT = 1
/** Levels above the fitted zoom you can zoom in to (each level is 2x). */
const ZOOM_IN_ABOVE_FIT = 5
/** Never cap below 4x the image's own pixels, however small the image. */
const MIN_MAX_ZOOM = 2

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
      // Fractional zoom. With Leaflet's default snap of 1 every wheel tick is
      // a whole level (2x), and fitBounds rounds DOWN to a whole level, which
      // leaves up to half the widget empty around a "fitted" plan.
      zoomSnap: 0,
      zoomDelta: 0.5,          // the +/- buttons and keyboard
      wheelPxPerZoomLevel: 120, // about half a level per wheel tick
      attributionControl: false,
    })

    imageBounds = L.latLngBounds(L.latLng(0, 0), L.latLng(height, width))
    L.imageOverlay(imageUrl, imageBounds).addTo(map.value)
    markerLayer.value = L.layerGroup().addTo(map.value)
    applyZoomRange(true)
    map.value.fitBounds(imageBounds)
    mapInitialized.value = true
  }

  /**
   * The zoom range is relative to the zoom that fits the plan, not fixed: in
   * CRS.Simple zoom 0 is one image pixel per screen pixel, so a fixed range
   * means a large image in a small widget fits near the bottom of it and has
   * only a level or two of zoom left. `reset` replaces the range; otherwise it
   * only widens, so a resize never yanks the current view.
   */
  const applyZoomRange = (reset = false) => {
    const m = map.value
    if (!m || !imageBounds) return
    // Not getBoundsZoom: that clamps to the range being computed here (and the
    // default minimum is 0). In CRS.Simple the scale at zoom z is 2^z.
    const size = m.getSize()
    const img = imageBounds.getNorthEast()
    if (!size.x || !size.y || !img.lng || !img.lat) return // not laid out yet
    const fit = Math.log2(Math.min(size.x / img.lng, size.y / img.lat))
    const min = fit - ZOOM_OUT_BELOW_FIT
    const max = Math.max(fit + ZOOM_IN_ABOVE_FIT, MIN_MAX_ZOOM)
    m.setMinZoom(reset ? min : Math.min(m.getMinZoom(), min))
    m.setMaxZoom(reset ? max : Math.max(m.getMaxZoom(), max))
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
    if (!map.value || !imageBounds) return
    applyZoomRange()
    map.value.fitBounds(imageBounds)
  }
  const invalidateSize = () => {
    if (!map.value) return
    map.value.invalidateSize()
    applyZoomRange()
  }

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
