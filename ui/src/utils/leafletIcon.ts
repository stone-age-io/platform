// The one place `L.divIcon` is called (leafletContentSinks.spec.ts bans it
// elsewhere). Its `html` option is an innerHTML sink for a string, but Leaflet
// 1.9 appends an Element, so this takes only an element. Build it with
// createElement / createElementNS and textContent.

import L from 'leaflet'

export interface ElementIconOptions {
  /** A class on the wrapping div, for styling and for state classes. */
  className?: string
  iconSize: [number, number]
  iconAnchor?: [number, number]
  tooltipAnchor?: [number, number]
}

export function elementIcon(el: HTMLElement | SVGElement, opts: ElementIconOptions): L.DivIcon {
  return L.divIcon({
    // @types/leaflet types html as string only; Leaflet itself accepts an Element.
    html: el as unknown as string,
    className: opts.className,
    iconSize: opts.iconSize,
    iconAnchor: opts.iconAnchor,
    tooltipAnchor: opts.tooltipAnchor,
  })
}
