import { ref, shallowRef, onUnmounted } from 'vue'
import L from 'leaflet'
// Leaflet positions its panes and markers entirely from ITS OWN stylesheet
// (`.leaflet-pane { position: absolute }`). Without it every pane falls back
// to static and stacks in normal flow, so the marker pane lands BELOW the
// image overlay instead of over it -- markers render, hit the widget’s
// `overflow: hidden`, and simply vanish. Nothing throws and the map itself
// looks fine, which is why this was invisible for so long: the location
// detail view mounts a geographic map too, that one imports the stylesheet,
// and this file quietly rode on it. A dashboard holding only a floor plan
// widget has no such neighbour. leafletCss.spec.ts pins it.
import 'leaflet/dist/leaflet.css'
import type { Thing } from '@/types/pocketbase'
import { fixLeafletIcons } from '@/utils/leafletIcons'
import { textElement } from '@/utils/leafletText'

export interface RenderOptions {
  draggable: boolean
  onMove: (id: string, x: number, y: number) => void
  onClick: (id: string) => void
  /**
   * Build this Thing's icon. Absent -- or returning undefined -- keeps
   * Leaflet's default pin, which is what the Location detail view wants.
   *
   * It is an injected renderer rather than a mode flag because the two callers
   * want genuinely different markers: the Location page is editing where things
   * ARE, so a pin that reads as a pin is right; a dashboard is showing what
   * they are DOING, which a raster pin cannot express -- the default marker is
   * a PNG fetched from unpkg (utils/leafletIcons.ts) and cannot be tinted at
   * all. A divIcon can, which is how live state reaches a marker.
   */
  icon?: (thing: Thing) => L.DivIcon | undefined
  /** Tooltip text. Set through textContent, never as a string -- see below. */
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
  // Kept so the view can be returned to the whole plan after panning. The
  // bounds are the image, not the markers: a plan is a fixed sheet, and
  // fitting to the markers would reframe the building every time a device
  // is placed or removed.
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
      const pos = thing.floorplan_position
      if (!pos || typeof pos.x !== 'number' || typeof pos.y !== 'number') return

      // Leaflet uses [lat, lng] = [y, x] in CRS.Simple.
      const icon = opts.icon?.(thing)
      const marker = L.marker([pos.y, pos.x], {
        draggable: opts.draggable,
        title: thing.name,
        ...(icon ? { icon } : {}),
      })

      // Leaflet assigns a STRING tooltip straight to innerHTML
      // (DivOverlay._updateContent) and every label here is data somebody else
      // wrote -- a Thing name any member can set, a mapping label out of an
      // imported dashboard, a KV value a device wrote. So nothing in this file
      // hands Leaflet a string: the tooltip goes in as a span set with
      // textContent. useLeafletMap.ts carries the same rule for the same
      // reason; textElement lives in utils/leafletText.ts so neither file has
      // to remember it alone.
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
   * Paint this marker with a live state.
   *
   * The COLOUR is an inline style and the flags are classes, which is not an
   * inconsistency: a state colour comes from operator config and may be any
   * CSS colour at all, so there is no fixed set of classes it could map onto,
   * while blink and stale are a closed set. The icon's SVG uses
   * `currentColor`, so one style property repaints the whole glyph.
   *
   * Separate from applySelection, deliberately: selection is which marker the
   * operator is looking at, state is what the device is doing, and both are
   * live at once. One function owning both would make clicking a door repaint
   * its alarm. The classes are namespaced `fp-state-*` for the same reason --
   * so the sweep here cannot remove `marker-selected` or anything Leaflet put
   * on the element itself.
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

  /**
   * Update one marker tooltip in place.
   *
   * Needed because a tooltip built at render time goes STALE: the marker
   * recolours on every message while its hover text still names the state it
   * had when the layer was last drawn, which is a label that disagrees with
   * the colour beside it. Re-rendering every marker to fix one string is the
   * wrong trade at 200 of them.
   */
  const setMarkerTooltip = (id: string, text: string) => {
    const marker = markerInstances.get(id)
    if (!marker) return
    if (marker.getTooltip()) marker.setTooltipContent(textElement(text))
    else marker.bindTooltip(textElement(text), { direction: 'top', offset: [0, -12] })
  }

  /** Swap one marker's icon in place, without re-rendering the layer. */
  const setMarkerIcon = (id: string, icon: L.DivIcon) => {
    markerInstances.get(id)?.setIcon(icon)
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
    setMarkerIcon,
    fitToImage,
    getViewCenter,
    invalidateSize,
    cleanup,
  }
}
