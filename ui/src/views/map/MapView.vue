<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useMap } from '@/composables/useMap'
import type { Location } from '@/types/pocketbase'

const authStore = useAuthStore()
const uiStore = useUIStore()
const { initMap, renderMarkers, updateTheme } = useMap()

const loading = ref(true)
const locations = ref<Location[]>([])
const mapContainerId = 'global-map-container'

// Data Loading
async function loadData() {
  if (!authStore.currentOrgId) return
  
  loading.value = true
  try {
    const result = await pb.collection('locations').getFullList<Location>({
      filter: 'coordinates != null', 
      expand: 'type',
    })
    locations.value = result
    renderMarkers(locations.value)
  } catch (err) {
    console.error('Failed to load map data:', err)
  } finally {
    loading.value = false
  }
}

// Watchers
watch(() => uiStore.theme, (newTheme) => {
  updateTheme(newTheme === 'dark')
})

watch(() => authStore.currentOrgId, () => {
  loadData()
})

onMounted(async () => {
  await loadData()
  // Initialize map
  initMap(mapContainerId, uiStore.theme === 'dark')
  renderMarkers(locations.value)
})
</script>

<template>
  <!-- Added z-0 to ensure map stays under mobile sidebar -->
  <div class="h-full flex flex-col gap-4 z-0">
    <!-- Header / Stats -->
    <div class="flex justify-between items-end px-1">
      <div>
        <h1 class="text-3xl font-bold">Map</h1>
        <p class="text-base-content/70 text-sm">
          {{ locations.length }} active location{{ locations.length !== 1 ? 's' : '' }}
        </p>
      </div>
      
      <div class="flex gap-2">
        <button @click="loadData" class="btn btn-sm btn-ghost" title="Refresh">
          🔄
        </button>
      </div>
    </div>

    <!-- Map Container Wrapper -->
    <!-- 
      FIX for Mobile: 
      Added 'min-h-[60vh]'. On mobile, flex-1 inside a scroll container often collapses to 0. 
      This forces a height. 'lg:min-h-0' resets it on desktop to use full flex height.
    -->
    <div class="flex-1 w-full relative min-h-[60vh] lg:min-h-0 bg-base-300 rounded-xl overflow-hidden shadow-lg border border-base-300">
      
      <!-- Leaflet Target -->
      <div :id="mapContainerId" class="absolute inset-0 z-0"></div>

      <!-- Loading Overlay -->
      <div 
        v-if="loading" 
        class="absolute inset-0 z-10 bg-base-100/50 backdrop-blur-sm flex items-center justify-center"
      >
        <span class="loading loading-spinner loading-lg text-primary"></span>
      </div>

      <!-- Empty State Overlay -->
      <div 
        v-if="!loading && locations.length === 0" 
        class="absolute inset-0 z-10 flex items-center justify-center pointer-events-none"
      >
        <div class="bg-base-100 p-6 rounded-lg shadow-xl text-center border border-base-200 pointer-events-auto max-w-sm">
          <span class="text-4xl">🗺️</span>
          <h3 class="font-bold text-lg mt-2">No Mapped Locations</h3>
          <p class="text-sm text-base-content/70 mt-1 mb-4">
            Add coordinates to your locations to see them here.
          </p>
          <router-link to="/locations" class="btn btn-primary btn-sm">
            Manage Locations
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
/* Leaflet Dark Mode Overrides */
.leaflet-popup-content-wrapper, .leaflet-popup-tip {
  background-color: oklch(var(--b1));
  color: oklch(var(--bc));
  border-radius: 0.5rem;
}
/* Ensure attribution doesn't overlap bottom mobile nav bars if present */
.leaflet-bottom {
  z-index: 1000;
}
</style>
