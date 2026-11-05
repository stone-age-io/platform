// ui/src/utils/leafletText.ts
//
// THE ONE RULE: never hand Leaflet a string for a tooltip or a popup.
//
// Leaflet assigns a string straight to innerHTML (DivOverlay._updateContent),
// and every label this app puts on a marker is data somebody else wrote -- a
// location or Thing name any member can set, a value a device wrote into KV, a
// marker label that arrived inside an imported dashboard. So a label goes in as
// an element whose text was set with textContent, and a popup is an element the
// caller built the same way.
//
// This lives in its own file because the rule used to be a comment inside
// useLeafletMap.ts, where only one of the two map composables could see it.
// useFloorPlan.ts was safe by accident rather than by intent -- it bound no
// tooltips at all and passed names through Leaflet's `title` option, which is
// an attribute -- so the first tooltip added there would have reintroduced the
// sink in a file that never got the warning. Both import this now.

export function textElement(s: string): HTMLElement {
  const span = document.createElement('span')
  span.textContent = s
  return span
}
