<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useLeafletMap, type MapMarkerInput } from '@/composables/useLeafletMap'
import type { Location } from '@/types/pocketbase'
import LocationMapDrawer, { type SiteDescendant } from '@/components/locations/LocationMapDrawer.vue'

const props = withDefaults(defineProps<{ searchQuery?: string }>(), {
  searchQuery: ''
})

const authStore = useAuthStore()
const uiStore = useUIStore()
const { initMap, renderMarkers, setSelectedMarker, updateTheme, fitAllMarkers, invalidateSize, cleanup } = useLeafletMap()

const loading = ref(true)
const locations = ref<Location[]>([])
const selectedLocations = ref<Location[]>([])
const isMobile = ref(false)
const mapContainerId = 'location-list-map-container'

function isMapped(l: Location): boolean {
  const c = l.coordinates
  return !!c && (c.lat !== 0 || c.lon !== 0)
}

const byId = computed(() => new Map(locations.value.map(l => [l.id, l])))

const childrenByParent = computed(() => {
  const map = new Map<string, Location[]>()
  for (const l of locations.value) {
    if (!l.parent) continue
    const siblings = map.get(l.parent)
    if (siblings) siblings.push(l)
    else map.set(l.parent, [l])
  }
  for (const siblings of map.values()) {
    siblings.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
  }
  return map
})

/**
 * The pin a location belongs to: its OUTERMOST mapped ancestor, or itself when
 * nothing above it has coordinates.
 *
 * This is the whole change. A campus, its buildings, their floors and their
 * rooms all carry coordinates within a few metres of each other, so one pin per
 * record is a pile at every address -- and "Room 302" is not a fact at map
 * scale anyway. Interior geography is what `floorplan` and `floorplan_position`
 * are for.
 *
 * Outermost rather than "no parent", because whether the root is mapped is a
 * tenant modelling choice we do not control. Put coordinates on the buildings
 * and not on the campus and a roots-only rule shows an empty map with no
 * explanation; this rule promotes the buildings instead. Intermediate ancestors
 * without coordinates are walked THROUGH, not stopped at, so a room under an
 * unmapped floor still folds onto its building.
 */
function siteIdFor(l: Location): string | null {
  let outermost: string | null = isMapped(l) ? l.id : null
  const seen = new Set<string>([l.id])
  let p = l.parent ? byId.value.get(l.parent) : undefined
  // The form refuses to re-parent a location under its own descendant, but a
  // cycle reaching this computed would hang the tab rather than misdraw a pin.
  while (p && !seen.has(p.id)) {
    seen.add(p.id)
    if (isMapped(p)) outermost = p.id
    p = p.parent ? byId.value.get(p.parent) : undefined
  }
  return outermost
}

/** Every location the map could draw, folded or not. */
const mappedLocations = computed(() => locations.value.filter(isMapped))

/** One marker each when idle: the locations that are their own site. */
const sites = computed(() =>
  mappedLocations.value.filter(l => siteIdFor(l) === l.id)
)

function subtreeOf(rootId: string): SiteDescendant[] {
  const out: SiteDescendant[] = []
  const seen = new Set<string>([rootId])
  const walk = (id: string, depth: number) => {
    for (const child of childrenByParent.value.get(id) ?? []) {
      if (seen.has(child.id)) continue
      seen.add(child.id)
      out.push({ location: child, depth })
      walk(child.id, depth + 1)
    }
  }
  walk(rootId, 0)
  return out
}

const foldedCount = computed(() =>
  sites.value.reduce((n, s) => n + subtreeOf(s.id).length, 0)
)

function matchesQuery(l: Location, q: string): boolean {
  const nameMatch = l.name?.toLowerCase().includes(q)
  const codeMatch = l.code?.toLowerCase().includes(q)
  const descMatch = l.description?.toLowerCase().includes(q)
  const typeMatch = l.expand?.type?.name?.toLowerCase().includes(q)
  const parentMatch = (l.expand?.parent as Location | undefined)?.name?.toLowerCase().includes(q)
  return !!(nameMatch || codeMatch || descMatch || typeMatch || parentMatch)
}

/**
 * Sites when idle; every matching mapped location when searching.
 *
 * The flatten mirrors the list view, which shows roots only until you type and
 * then searches the whole tree -- and the two share one search box on one
 * screen, so a map that folded while the list flattened would put two
 * disagreeing counts side by side. The stacking that reintroduces is exactly
 * what the clustering below is for.
 */
const markerLocations = computed(() => {
  const q = props.searchQuery.toLowerCase().trim()
  if (!q) return sites.value
  return mappedLocations.value.filter(l => matchesQuery(l, q))
})

const isSearching = computed(() => props.searchQuery.trim().length > 0)

const markersForMap = computed<MapMarkerInput[]>(() =>
  markerLocations.value.map(l => {
    // The folded count belongs on the tooltip only when the pin actually covers
    // those records. While searching it does not -- a matching sub-location is
    // its own pin -- so the number would be a claim about the map that is false.
    const folded = isSearching.value ? 0 : subtreeOf(l.id).length
    return {
      id: l.id,
      lat: l.coordinates!.lat,
      lon: l.coordinates!.lon,
      label: folded > 0 ? `${l.name} (${folded})` : l.name,
    }
  })
)

/** The subtree for the drawer's single-selection mode. */
const selectedDescendants = computed<SiteDescendant[]>(() =>
  selectedLocations.value.length === 1 ? subtreeOf(selectedLocations.value[0].id) : []
)

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

function handleMarkerClick(id: string) {
  if (selectedLocations.value.length === 1 && selectedLocations.value[0].id === id) {
    closeDrawer()
    return
  }
  const loc = byId.value.get(id)
  if (!loc) return
  selectedLocations.value = [loc]
  setSelectedMarker(id)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleClusterClick(ids: string[]) {
  const matched = ids
    .map(id => byId.value.get(id))
    .filter((l): l is Location => !!l)
  if (matched.length === 0) {
    closeDrawer()
    return
  }
  selectedLocations.value = matched
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleSelectFromList(locationId: string) {
  const loc = byId.value.get(locationId)
  if (!loc) return
  selectedLocations.value = [loc]
  setSelectedMarker(locationId)
}

function handleMapClick(event: MouseEvent) {
  if (isMobile.value) return
  if (selectedLocations.value.length === 0) return
  const target = event.target as HTMLElement
  if (target.closest('.leaflet-marker-icon') || target.closest('.location-map-drawer')) return
  closeDrawer()
}

function closeDrawer() {
  selectedLocations.value = []
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

async function loadData() {
  if (!authStore.currentOrgId) return

  loading.value = true
  try {
    // Every location, not just the mapped ones. Folding needs the full ancestor
    // chain -- an unmapped floor between a mapped room and its mapped building
    // has to be walked through -- and the drawer's subtree needs the unmapped
    // rooms, which are real places whether or not anyone gave them a latitude.
    locations.value = await pb.collection('locations').getFullList<Location>({
      expand: 'type,parent',
    })
  } catch (err) {
    console.error('Failed to load map data:', err)
  } finally {
    loading.value = false
  }
}

watch(() => uiStore.theme, (newTheme) => updateTheme(newTheme === 'dark'))
watch(() => authStore.currentOrgId, () => {
  closeDrawer()
  loadData()
})

watch(markersForMap, (next) => {
  renderMarkers(next, handleMarkerClick, { fitBounds: true })
  if (selectedLocations.value.length === 0) return
  const visibleIds = new Set(next.map(m => m.id))
  const stillVisible = selectedLocations.value.filter(l => visibleIds.has(l.id))
  if (stillVisible.length === 0) {
    closeDrawer()
  } else if (stillVisible.length !== selectedLocations.value.length) {
    selectedLocations.value = stillVisible
  }
})

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  // The basemap before the data, deliberately. The tiles do not depend on the
  // inventory, so awaiting loadData() first put a getFullList round trip (plus
  // two relation expansions) in front of MapLibre's own chain -- style, then
  // TileJSON, then glyphs and sprite, then tiles -- and the map sat as a flat
  // sheet of theme colour for all of it.
  //
  // onClusterClick both enables clustering and takes over the click: folding
  // handles pins stacked by hierarchy, and this handles pins stacked by
  // geography -- adjacent sites that no amount of folding will separate.
  initMap(mapContainerId, {
    isDarkMode: uiStore.theme === 'dark',
    onClusterClick: handleClusterClick,
    zoomControlPosition: 'bottomleft',
  })
  await loadData()
  renderMarkers(markersForMap.value, handleMarkerClick, { fitBounds: true })
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  cleanup()
})
</script>

<template>
  <div class="h-full flex flex-col relative isolate min-h-[600px] bg-base-300 rounded-xl overflow-hidden shadow-lg border border-base-300">

    <!-- Stats Overlay (Top Left) -->
    <div class="absolute top-4 left-4 z-[400]">
      <!-- Two counts, because one pin is no longer one location. Saying only
           "12 mapped" over a folded map would quietly redefine what the number
           counts; naming the folded records keeps the badge honest about what
           is on screen and what is behind it. -->
      <div class="badge badge-lg bg-base-100/90 backdrop-blur border-base-300 shadow-sm gap-2">
        <span>📍</span>
        <template v-if="isSearching">
          <span class="font-bold">{{ markerLocations.length }}</span>
          <span class="text-xs text-base-content/70">of {{ mappedLocations.length }} matched</span>
        </template>
        <template v-else>
          <span class="font-bold">{{ sites.length }}</span>
          <span class="text-xs text-base-content/70">
            {{ sites.length === 1 ? 'site' : 'sites' }}<template v-if="foldedCount > 0"> &middot; {{ foldedCount }} sub-locations</template>
          </span>
        </template>
      </div>
    </div>

    <!-- Map Controls (Top Right) -->
    <div v-if="markerLocations.length > 0" class="absolute top-[10px] right-[10px] z-[400] flex flex-col gap-2">
      <button
        class="btn btn-sm btn-square bg-base-100 border-base-300 shadow-sm hover:bg-base-200"
        @click="fitAllMarkers()"
        :title="isSearching ? 'Fit all matches' : 'Fit all sites'"
        :aria-label="isSearching ? 'Fit all matches in view' : 'Fit all sites in view'"
      >
        <span class="text-lg leading-none pb-1">⊡</span>
      </button>
    </div>

    <!-- Leaflet Target -->
    <div :id="mapContainerId" class="absolute inset-0 z-0" @click="handleMapClick"></div>

    <!-- Loading Overlay -->
    <div v-if="loading" class="absolute inset-0 z-10 bg-base-100/50 backdrop-blur-sm flex items-center justify-center">
      <span class="loading loading-spinner loading-lg text-primary"></span>
    </div>

    <!-- Empty State: no mapped locations at all -->
    <div v-if="!loading && mappedLocations.length === 0" class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
      <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
        <span class="text-4xl">🗺️</span>
        <h3 class="font-bold text-lg mt-2">No Mapped Locations</h3>
        <p class="text-sm text-base-content/70 mt-1">
          Add coordinates to your locations to view them on the map.
        </p>
      </div>
    </div>

    <!-- Empty State: search has no matches -->
    <div v-else-if="!loading && markerLocations.length === 0 && isSearching" class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
      <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
        <span class="text-4xl">🔍</span>
        <h3 class="font-bold text-lg mt-2">No matching locations</h3>
        <p class="text-sm text-base-content/70 mt-1">
          No mapped locations match your search.
        </p>
      </div>
    </div>

    <!-- Location Drawer (adapts: site list when a cluster, detail when one) -->
    <LocationMapDrawer
      v-if="selectedLocations.length > 0"
      :locations="selectedLocations"
      :descendants="selectedDescendants"
      :is-mobile="isMobile"
      class="location-map-drawer"
      @close="closeDrawer"
      @select="handleSelectFromList"
    />
  </div>
</template>

<style scoped>
:deep(.leaflet-popup-content-wrapper), :deep(.leaflet-popup-tip) {
  background-color: oklch(var(--b1));
  color: oklch(var(--bc));
  border-radius: 0.5rem;
}
:deep(.marker-selected) {
  filter: hue-rotate(180deg) saturate(1.5) drop-shadow(0 0 8px rgba(116, 128, 255, 0.8));
}
/* Cluster marker theme-aware overrides (three shades of primary). These shipped
   before clustering was ever switched on here and were inert until now. */
:deep(.marker-cluster) { background-color: oklch(var(--p) / 0.25); }
:deep(.marker-cluster div) { background-color: oklch(var(--p) / 0.7); color: oklch(var(--pc)); font-weight: 600; }
:deep(.marker-cluster-medium div) { background-color: oklch(var(--p) / 0.85); }
:deep(.marker-cluster-large div) { background-color: oklch(var(--p)); }
/* Ensure controls sit above map tiles */
.absolute { pointer-events: auto; }
</style>
