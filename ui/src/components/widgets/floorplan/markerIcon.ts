import type L from 'leaflet'
import { elementIcon } from '@/utils/leafletIcon'

/**
 * The floor plan marker: a dot coloured by live state, centred on its position.
 *
 * It sits on the operator's uploaded image, so no theme colour can be trusted
 * to show: a solid body in the state colour (`currentColor`), a white ring for
 * dark plans and a dark hairline for white paper. Both rings are constants.
 *
 * Not Leaflet's default pin, an untintable PNG. Built from DOM nodes, so there
 * is no markup string to inject into; a label belongs in the tooltip.
 */

const SIZE = 30
const NS = 'http://www.w3.org/2000/svg'

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
