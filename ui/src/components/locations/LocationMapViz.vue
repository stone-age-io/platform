<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useMap } from '@/composables/useMap'
import type { Location } from '@/types/pocketbase'
import LocationMapDrawer from '@/components/locations/LocationMapDrawer.vue'

const authStore = useAuthStore()
const uiStore = useUIStore()
const { initMap, renderMarkers, setSelectedMarker, updateTheme, fitToMarkers, invalidateSize, cleanup } = useMap()

const loading = ref(true)
const locations = ref<Location[]>([])
const selectedLocation = ref<Location | null>(null)
const isMobile = ref(false)
const mapContainerId = 'location-list-map-container'

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

function handleMarkerClick(location: Location) {
  if (selectedLocation.value?.id === location.id) {
    closeDrawer()
    return
  }
  selectedLocation.value = location
  setSelectedMarker(location.id)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleMapClick(event: MouseEvent) {
  if (isMobile.value) return
  if (!selectedLocation.value) return
  const target = event.target as HTMLElement
  if (target.closest('.leaflet-marker-icon') || target.closest('.location-map-drawer')) return
  closeDrawer()
}

function closeDrawer() {
  selectedLocation.value = null
  setSelectedMarker(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

async function loadData() {
  if (!authStore.currentOrgId) return

  loading.value = true
  try {
    const result = await pb.collection('locations').getFullList<Location>({
      filter: 'coordinates != ""',
      expand: 'type',
    })

    locations.value = result.filter(l => {
      const lat = l.coordinates?.lat ?? 0
      const lon = l.coordinates?.lon ?? 0
      return lat !== 0 || lon !== 0
    })

    renderMarkers(locations.value, handleMarkerClick)
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

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  await loadData()
  initMap(mapContainerId, uiStore.theme === 'dark')
  renderMarkers(locations.value, handleMarkerClick)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  cleanup()
})
</script>

<template>
  <div class="h-full flex flex-col relative min-h-[600px] bg-base-300 rounded-xl overflow-hidden shadow-lg border border-base-300">

    <!-- Stats Overlay (Top Left) -->
    <div class="absolute top-4 left-4 z-[400]">
      <div class="badge badge-lg bg-base-100/90 backdrop-blur border-base-300 shadow-sm gap-2">
        <span>📍</span>
        <span class="font-bold">{{ locations.length }}</span>
        <span class="text-xs opacity-70">mapped</span>
      </div>
    </div>

    <!-- Map Controls (Top Right) -->
    <div v-if="locations.length > 0" class="absolute top-[80px] right-[10px] z-[400] flex flex-col gap-2">
      <button
        class="btn btn-sm btn-square bg-base-100 border-base-300 shadow-sm hover:bg-base-200"
        @click="fitToMarkers"
        title="Fit all locations"
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

    <!-- Empty State -->
    <div v-if="!loading && locations.length === 0" class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
      <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
        <span class="text-4xl">🗺️</span>
        <h3 class="font-bold text-lg mt-2">No Mapped Locations</h3>
        <p class="text-sm text-base-content/70 mt-1">
          Add coordinates to your locations to view them on the map.
        </p>
      </div>
    </div>

    <!-- Location Drawer -->
    <LocationMapDrawer
      v-if="selectedLocation"
      :location="selectedLocation"
      :is-mobile="isMobile"
      class="location-map-drawer"
      @close="closeDrawer"
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
/* Ensure controls sit above map tiles */
.absolute { pointer-events: auto; }
</style>
