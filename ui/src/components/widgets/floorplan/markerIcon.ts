// ui/src/components/widgets/floorplan/markerIcon.ts
import type L from 'leaflet'
import { elementIcon } from '@/utils/leafletIcon'

/**
 * The floor plan marker: one shape, coloured by live state.
 *
 * THE BACKDROP IS NOT THE APP. This is the mistake the first version made and
 * the reason markers were invisible: it styled the glyph with theme tokens, so
 * an unbound marker drew in `var(--muted)` -- which on the dark theme is
 * near-white at half opacity. Perfectly legible against the console's own
 * surface, and completely invisible on the white sheet of an architectural
 * drawing, which is what a floor plan almost always is.
 *
 * A marker sits on an image the operator uploaded. Nothing about the theme
 * predicts what colour that image is, and the state colours an operator picks
 * are chosen against the UI too. So the glyph carries its own contrast and does
 * not depend on the state colour being visible at all:
 *
 *   - a SOLID body in the state colour (not a 22% tint, which washed out)
 *   - a white ring around it, so a dark state colour on a dark plan separates
 *   - a dark hairline outside that, so a light state colour on white paper does
 *
 * Both rings are constants, never `currentColor`. Whatever colour the state
 * resolves to -- including a neutral grey for "no data" -- the marker is a
 * visible object on any background. The same reason Leaflet's own pin is a
 * coloured body with a dark outline and a shadow.
 *
 * WHY NOT LEAFLET'S DEFAULT PIN. It is a raster PNG fetched from unpkg
 * (utils/leafletIcons.ts): it cannot be tinted, so it cannot show state at all,
 * and a `dashboard` appliance screen should not need a CDN round trip to draw
 * its own markers. Everything here is inline.
 *
 * WHY A DOT AND NOT A PIN. A pin points at a spot from above it, which is right
 * on a map where the ground is what matters. On a plan the marker IS the
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

const SIZE = 30
const NS = 'http://www.w3.org/2000/svg'

/** The halo and hairline are fixed: see the note above. */
const RING_LIGHT = 'rgba(255, 255, 255, 0.95)'
const RING_DARK = 'rgba(17, 17, 17, 0.55)'

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

  const c = SIZE / 2
  // Outermost first: the dark hairline, then the white halo, then the body.
  svg.appendChild(circle({ cx: `${c}`, cy: `${c}`, r: '12', fill: 'none', stroke: RING_DARK, 'stroke-width': '1.5' }))
  svg.appendChild(circle({ cx: `${c}`, cy: `${c}`, r: '10.5', fill: 'none', stroke: RING_LIGHT, 'stroke-width': '3' }))
  svg.appendChild(circle({ cx: `${c}`, cy: `${c}`, r: '9', fill: 'currentColor' }))
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
