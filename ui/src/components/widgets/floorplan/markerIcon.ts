// ui/src/components/widgets/floorplan/markerIcon.ts
import type L from 'leaflet'
import { elementIcon } from '@/utils/leafletIcon'

/**
 * The floor plan marker: one shape, coloured by live state.
 *
 * WHY NOT LEAFLET'S DEFAULT PIN. It is a raster PNG fetched from unpkg
 * (utils/leafletIcons.ts). It cannot be tinted, so it cannot express the one
 * thing this widget exists to show; and `dashboard` is an appliance login on an
 * unattended screen, which should not need a CDN round trip to draw its own
 * markers. Everything here is inline.
 *
 * WHY `currentColor`. The state colour is operator config and may be any CSS
 * colour, so it cannot be a class. `useFloorPlan.setMarkerState` sets one
 * inline `color` on the marker element and the whole glyph follows.
 *
 * WHY A DOT AND NOT A PIN. A pin points at a spot from above it, which is right
 * on a map, where the ground is what matters. On a plan the marker IS the
 * device, so it is centred on its own position -- hence `iconAnchor` at the
 * middle rather than at a tip.
 *
 * WHY IT IS BUILT FROM NODES. `divIcon`'s `html` is an innerHTML sink, banned
 * outside utils/leafletIcon.ts by leafletContentSinks.spec.ts. Passing an
 * element skips innerHTML entirely. The markup here is a constant with nothing
 * interpolated into it, so a string would have been safe TODAY -- and that is
 * exactly the argument whoever later wants a device name in the glyph will
 * make. There is no string to add it to. A label belongs in the tooltip, which
 * goes through utils/leafletText.ts.
 */

const SIZE = 26
const NS = 'http://www.w3.org/2000/svg'

function circle(attrs: Record<string, string>): SVGCircleElement {
  const el = document.createElementNS(NS, 'circle')
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  return el
}

function buildGlyph(): SVGSVGElement {
  const svg = document.createElementNS(NS, 'svg')
  svg.setAttribute('viewBox', `0 0 ${SIZE} ${SIZE}`)
  svg.setAttribute('width', String(SIZE))
  svg.setAttribute('height', String(SIZE))
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('focusable', 'false')

  // A filled disc, its ring, and a solid centre: readable at 26px against both
  // a pale and a dark floor plan, and all three follow `currentColor`.
  svg.appendChild(circle({ cx: '13', cy: '13', r: '11', fill: 'currentColor', 'fill-opacity': '0.22' }))
  svg.appendChild(circle({ cx: '13', cy: '13', r: '11', fill: 'none', stroke: 'currentColor', 'stroke-width': '2' }))
  svg.appendChild(circle({ cx: '13', cy: '13', r: '4.5', fill: 'currentColor' }))
  return svg
}

export function floorPlanMarkerIcon(): L.DivIcon {
  return elementIcon(buildGlyph(), {
    className: 'fp-marker',
    iconSize: [SIZE, SIZE],
    iconAnchor: [SIZE / 2, SIZE / 2],
    tooltipAnchor: [0, -SIZE / 2],
  })
}
