import L from 'leaflet'

/**
 * Leaflet's default marker icon URLs break when bundled (Vite rewrites the paths
 * it auto-detects from the CSS), which renders markers as broken-image icons —
 * e.g. question marks on iOS Safari. Point them at the CDN explicitly.
 *
 * Mutates the global L.Icon.Default singleton, so it's idempotent and safe to
 * call from every map composable. Each map should call it so its markers render
 * without depending on another map having initialized first.
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
