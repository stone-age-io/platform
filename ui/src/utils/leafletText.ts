// Never hand Leaflet a string for a tooltip or popup: it assigns strings to
// innerHTML, and labels are user data. Pass an element built with textContent.
// leafletContentSinks.spec.ts enforces it.

export function textElement(s: string): HTMLElement {
  const span = document.createElement('span')
  span.textContent = s
  return span
}
