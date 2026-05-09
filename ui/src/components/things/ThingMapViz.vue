<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useMap } from '@/composables/useMap'
import type { Thing, Location } from '@/types/pocketbase'
import ThingMapDrawer from '@/components/things/ThingMapDrawer.vue'

const props = withDefaults(defineProps<{ searchQuery?: string }>(), {
  searchQuery: ''
})

const authStore = useAuthStore()
const uiStore = useUIStore()
const { initMap, renderMarkers, setSelectedMarker, updateTheme, fitToMarkers, invalidateSize, cleanup } = useMap()

const loading = ref(true)
const things = ref<Thing[]>([])
const selectedThings = ref<Thing[]>([])
const isMobile = ref(false)
const mapContainerId = 'thing-list-map-container'

// Things whose parent location has valid coordinates
const mappableThings = computed(() => {
  return things.value.filter(t => {
    const c = (t.expand?.location as Location | undefined)?.coordinates
    return !!c && (c.lat !== 0 || c.lon !== 0)
  })
})

// Apply search filter on top of the mappable set
const filteredThings = computed(() => {
  const q = props.searchQuery.toLowerCase().trim()
  if (!q) return mappableThings.value
  return mappableThings.value.filter(t => {
    const nameMatch = t.name?.toLowerCase().includes(q)
    const codeMatch = t.code?.toLowerCase().includes(q)
    const descMatch = t.description?.toLowerCase().includes(q)
    const typeMatch = t.expand?.type?.name?.toLowerCase().includes(q)
    const locMatch = (t.expand?.location as Location | undefined)?.name?.toLowerCase().includes(q)
    return nameMatch || codeMatch || descMatch || typeMatch || locMatch
  })
})

// Synthesize Location-shaped markers from things (useMap is Location-typed).
// We always pass an onMarkerClick, so the default popup branch is unused.
const markersForMap = computed<Location[]>(() => {
  return filteredThings.value.map(t => ({
    id: t.id,
    name: t.name,
    coordinates: (t.expand!.location as Location).coordinates,
    created: t.created,
    updated: t.updated,
  } as Location))
})

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

function handleMarkerClick(marker: Location) {
  // marker.id is the Thing id we stamped in via useMap
  if (selectedThings.value.length === 1 && selectedThings.value[0].id === marker.id) {
    closeDrawer()
    return
  }
  const thing = things.value.find(t => t.id === marker.id)
  if (!thing) return
  selectedThings.value = [thing]
  setSelectedMarker(marker.id)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleClusterClick(ids: string[]) {
  const matched = ids
    .map(id => things.value.find(t => t.id === id))
    .filter((t): t is Thing => !!t)
  if (matched.length === 0) {
    closeDrawer()
    return
  }
  selectedThings.value = matched
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleSelectFromList(thingId: string) {
  const thing = things.value.find(t => t.id === thingId)
  if (!thing) return
  selectedThings.value = [thing]
  setSelectedMarker(thingId)
}

function handleMapClick(event: MouseEvent) {
  if (isMobile.value) return
  if (selectedThings.value.length === 0) return
  const target = event.target as HTMLElement
  if (target.closest('.leaflet-marker-icon') || target.closest('.thing-map-drawer')) return
  closeDrawer()
}

function closeDrawer() {
  selectedThings.value = []
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

async function loadData() {
  if (!authStore.currentOrgId) return

  loading.value = true
  try {
    things.value = await pb.collection('things').getFullList<Thing>({
      expand: 'type,location',
    })
  } catch (err) {
    console.error('Failed to load things for map:', err)
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
  renderMarkers(next, handleMarkerClick)
  if (selectedThings.value.length === 0) return
  const visibleIds = new Set(next.map(m => m.id))
  const stillVisible = selectedThings.value.filter(t => visibleIds.has(t.id))
  if (stillVisible.length === 0) {
    closeDrawer()
  } else if (stillVisible.length !== selectedThings.value.length) {
    selectedThings.value = stillVisible
  }
})

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  await loadData()
  initMap(mapContainerId, uiStore.theme === 'dark', handleClusterClick)
  renderMarkers(markersForMap.value, handleMarkerClick)
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
      <div class="badge badge-lg bg-base-100/90 backdrop-blur border-base-300 shadow-sm gap-2">
        <span>📦</span>
        <span class="font-bold">{{ filteredThings.length }}</span>
        <span v-if="props.searchQuery && filteredThings.length !== mappableThings.length" class="text-xs opacity-70">of {{ mappableThings.length }} mapped</span>
        <span v-else class="text-xs opacity-70">mapped</span>
      </div>
    </div>

    <!-- Map Controls (Top Right) -->
    <div v-if="filteredThings.length > 0" class="absolute top-[80px] right-[10px] z-[400] flex flex-col gap-2">
      <button
        class="btn btn-sm btn-square bg-base-100 border-base-300 shadow-sm hover:bg-base-200"
        @click="fitToMarkers"
        title="Fit all things"
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

    <!-- Empty State: no mappable things at all -->
    <div v-if="!loading && mappableThings.length === 0" class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
      <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
        <span class="text-4xl">🗺️</span>
        <h3 class="font-bold text-lg mt-2">No Mapped Things</h3>
        <p class="text-sm text-base-content/70 mt-1">
          Things appear here when assigned to a location with coordinates.
        </p>
      </div>
    </div>

    <!-- Empty State: search has no matches -->
    <div v-else-if="!loading && filteredThings.length === 0 && props.searchQuery" class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
      <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
        <span class="text-4xl">🔍</span>
        <h3 class="font-bold text-lg mt-2">No matching things</h3>
        <p class="text-sm text-base-content/70 mt-1">
          No mapped things match your search.
        </p>
      </div>
    </div>

    <!-- Thing Drawer (adapts: list when many, detail when one) -->
    <ThingMapDrawer
      v-if="selectedThings.length > 0"
      :things="selectedThings"
      :is-mobile="isMobile"
      class="thing-map-drawer"
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
/* Cluster marker theme-aware overrides (three shades of primary) */
:deep(.marker-cluster) { background-color: oklch(var(--p) / 0.25); }
:deep(.marker-cluster div) { background-color: oklch(var(--p) / 0.7); color: oklch(var(--pc)); font-weight: 600; }
:deep(.marker-cluster-medium div) { background-color: oklch(var(--p) / 0.85); }
:deep(.marker-cluster-large div) { background-color: oklch(var(--p)); }
.absolute { pointer-events: auto; }
</style>
