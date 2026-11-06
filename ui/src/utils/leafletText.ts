// ui/src/utils/leafletText.ts
//
// Never hand Leaflet a string for a tooltip or popup: it assigns strings to
// innerHTML, and every label here is data someone else wrote (a Thing name, a
// KV value, an imported dashboard). Pass an element whose text was set with
// textContent. leafletContentSinks.spec.ts enforces it.

export function textElement(s: string): HTMLElement {
  const span = document.createElement('span')
  span.textContent = s
  return span
}
