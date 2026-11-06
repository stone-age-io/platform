import L from 'leaflet'

/**
 * Leaflet's default marker icon URLs break when bundled by Vite, so point them
 * at the CDN. Idempotent (mutates the global L.Icon.Default); every map should
 * call it rather than rely on another map having run first.
 */
export function fixLeafletIcons() {
  // @ts-ignore - _getIconUrl is an internal Leaflet method
  delete L.Icon.Default.prototype._getIconUrl
  L.Icon.Default.mergeOptions({
    iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
    iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
    shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  })
}
