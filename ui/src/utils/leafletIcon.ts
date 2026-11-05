// ui/src/utils/leafletIcon.ts
//
// The ONE place `L.divIcon` is called.
//
// `divIcon`'s `html` option is the same sink as a string tooltip: Leaflet
// assigns it with `div.innerHTML = options.html` (DivIcon.createIcon). It is
// banned everywhere else by leafletContentSinks.spec.ts, for the reason that
// file records -- member-named locations once ran script in the session of any
// owner who opened the map.
//
// Leaflet 1.9 takes an ELEMENT there as well as a string:
//
//     if (options.html instanceof Element) { empty(div); div.appendChild(options.html) }
//
// so a caller-built element skips innerHTML altogether. That is the only form
// this helper accepts, and it is why the guard allows this file and no other:
// the same shape as `textElement()` being the sanctioned argument for tooltips.
//
// A caller builds its element with createElement / createElementNS and sets any
// text with textContent. Never assemble markup as a string and parse it back --
// that is the sink again with extra steps.

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
    // Typed as string by @types/leaflet, which predates the Element branch
    // above. The cast is to the type definition, not past a check.
    html: el as unknown as string,
    className: opts.className,
    iconSize: opts.iconSize,
    iconAnchor: opts.iconAnchor,
    tooltipAnchor: opts.tooltipAnchor,
  })
}
