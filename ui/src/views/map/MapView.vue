<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { Location } from '@/types/pocketbase'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'

const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()

// State
const mapContainer = ref<HTMLElement | null>(null)
const locations = ref<Location[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

// Leaflet map instance
let map: L.Map | null = null
const markers: L.Marker[] = []

/**
 * Initialize Leaflet map
 */
function initMap() {
  if (!mapContainer.value || map) return
  
  try {
    // Create map centered on USA (you can change this default)
    map = L.map(mapContainer.value, {
      center: [39.8283, -98.5795], // Center of USA
      zoom: 4,
      zoomControl: true,
      scrollWheelZoom: true,
    })
    
    // Add OpenStreetMap tiles
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
      maxZoom: 19,
    }).addTo(map)
    
    // Fix icon paths for Leaflet (Vite issue)
    // @ts-ignore
    delete L.Icon.Default.prototype._getIconUrl
    L.Icon.Default.mergeOptions({
      iconRetinaUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-icon-2x.png',
      iconUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-icon.png',
      shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-shadow.png',
    })
  } catch (err: any) {
    console.error('Failed to initialize map:', err)
    error.value = 'Failed to initialize map'
    toast.error('Failed to initialize map')
  }
}

/**
 * Clear all markers from map
 */
function clearMarkers() {
  if (!map) return
  
  markers.forEach(marker => {
    map?.removeLayer(marker)
  })
  markers.length = 0
}

/**
 * Add markers for all locations with coordinates
 */
function updateMarkers() {
  if (!map) return
  
  // Clear existing markers
  clearMarkers()
  
  // Filter locations that have coordinates
  const locationsWithCoords = locations.value.filter(
    loc => loc.coordinates && loc.coordinates.lat && loc.coordinates.lon
  )
  
  if (locationsWithCoords.length === 0) return
  
  // Add markers for each location
  locationsWithCoords.forEach(location => {
    const { lat, lon } = location.coordinates!
    
    // Create marker
    const marker = L.marker([lat, lon])
      .addTo(map!)
      .bindPopup(`
        <div class="p-2">
          <h3 class="font-bold text-base mb-1">${location.name || 'Unnamed Location'}</h3>
          ${location.description ? `<p class="text-sm text-gray-600 mb-2">${location.description}</p>` : ''}
          ${location.expand?.type ? `<p class="text-xs text-gray-500 mb-2">Type: ${location.expand.type.name}</p>` : ''}
          <button 
            onclick="window.location.href='/locations/${location.id}'" 
            class="text-xs text-blue-600 hover:text-blue-800 font-medium"
          >
            View Details →
          </button>
        </div>
      `)
    
    markers.push(marker)
  })
  
  // Fit map bounds to show all markers
  if (locationsWithCoords.length > 0) {
    const bounds = L.latLngBounds(
      locationsWithCoords.map(loc => [loc.coordinates!.lat, loc.coordinates!.lon])
    )
    
    // Add padding to bounds
    map.fitBounds(bounds, {
      padding: [50, 50],
      maxZoom: 15, // Don't zoom in too far for single locations
    })
  }
}

/**
 * Load locations from API
 * Backend automatically filters by organization
 */
async function loadLocations() {
  if (!authStore.currentOrgId) {
    error.value = 'No organization selected'
    loading.value = false
    return
  }
  
  loading.value = true
  error.value = null
  
  try {
    // Load all locations with coordinates
    // Backend filters by organization automatically
    const result = await pb.collection('locations').getFullList<Location>({
      expand: 'type',
      sort: 'name',
    })
    
    locations.value = result
    
    // Update map markers
    updateMarkers()
  } catch (err: any) {
    console.error('Failed to load locations:', err)
    error.value = err.message || 'Failed to load locations'
    toast.error('Failed to load locations')
  } finally {
    loading.value = false
  }
}

/**
 * Handle organization change event
 */
function handleOrgChange() {
  loadLocations()
}

/**
 * Cleanup on unmount
 */
function cleanup() {
  clearMarkers()
  if (map) {
    map.remove()
    map = null
  }
}

/**
 * Navigate to create location form
 */
function createLocation() {
  router.push('/locations/new')
}

/**
 * Initialize on mount
 */
onMounted(() => {
  initMap()
  loadLocations()
  window.addEventListener('organization-changed', handleOrgChange)
})

/**
 * Cleanup on unmount
 */
onUnmounted(() => {
  cleanup()
  window.removeEventListener('organization-changed', handleOrgChange)
})

/**
 * Get count of locations with coordinates
 */
const locationsWithCoords = ref(0)
const locationsWithoutCoords = ref(0)

// Update counts when locations change
function updateCounts() {
  const withCoords = locations.value.filter(
    loc => loc.coordinates && loc.coordinates.lat && loc.coordinates.lon
  )
  locationsWithCoords.value = withCoords.length
  locationsWithoutCoords.value = locations.value.length - withCoords.length
}

// Watch locations and update counts
import { watch } from 'vue'
watch(locations, updateCounts, { immediate: true })
</script>

<template>
  <div class="space-y-6 h-full flex flex-col">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Location Map</h1>
        <p class="text-base-content/70 mt-1">
          Visual overview of all locations with coordinates
        </p>
      </div>
      <button @click="createLocation" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Location</span>
      </button>
    </div>
    
    <!-- Stats Bar -->
    <div v-if="!loading && !error" class="stats stats-horizontal shadow w-full">
      <div class="stat">
        <div class="stat-figure text-primary">
          <span class="text-3xl">📍</span>
        </div>
        <div class="stat-title">On Map</div>
        <div class="stat-value text-primary">{{ locationsWithCoords }}</div>
        <div class="stat-desc">Locations with coordinates</div>
      </div>
      
      <div class="stat">
        <div class="stat-figure text-secondary">
          <span class="text-3xl">📦</span>
        </div>
        <div class="stat-title">Total</div>
        <div class="stat-value text-secondary">{{ locations.length }}</div>
        <div class="stat-desc">All locations</div>
      </div>
      
      <div v-if="locationsWithoutCoords > 0" class="stat">
        <div class="stat-figure text-warning">
          <span class="text-3xl">⚠️</span>
        </div>
        <div class="stat-title">Missing Coords</div>
        <div class="stat-value text-warning">{{ locationsWithoutCoords }}</div>
        <div class="stat-desc">
          <router-link to="/locations" class="link link-hover">
            Add coordinates
          </router-link>
        </div>
      </div>
    </div>
    
    <!-- Loading State -->
    <div v-if="loading" class="flex-1 flex items-center justify-center bg-base-200 rounded-lg">
      <div class="text-center">
        <span class="loading loading-spinner loading-lg"></span>
        <p class="mt-4 text-base-content/70">Loading locations...</p>
      </div>
    </div>
    
    <!-- Error State -->
    <div v-else-if="error" class="flex-1 flex items-center justify-center bg-base-200 rounded-lg">
      <div class="text-center max-w-md px-4">
        <span class="text-6xl">⚠️</span>
        <h3 class="text-xl font-bold mt-4">Failed to Load Map</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadLocations" class="btn btn-primary mt-4">
          Try Again
        </button>
      </div>
    </div>
    
    <!-- Empty State - No Locations -->
    <div 
      v-else-if="locations.length === 0" 
      class="flex-1 flex items-center justify-center bg-base-200 rounded-lg"
    >
      <div class="text-center max-w-md px-4">
        <span class="text-6xl">🗺️</span>
        <h3 class="text-xl font-bold mt-4">No Locations Yet</h3>
        <p class="text-base-content/70 mt-2">
          Create your first location to see it on the map
        </p>
        <button @click="createLocation" class="btn btn-primary mt-4">
          Create Location
        </button>
      </div>
    </div>
    
    <!-- Empty State - No Coordinates -->
    <div 
      v-else-if="locationsWithCoords === 0" 
      class="flex-1 flex items-center justify-center bg-base-200 rounded-lg"
    >
      <div class="text-center max-w-md px-4">
        <span class="text-6xl">📍</span>
        <h3 class="text-xl font-bold mt-4">No Coordinates Set</h3>
        <p class="text-base-content/70 mt-2">
          You have {{ locations.length }} location{{ locations.length === 1 ? '' : 's' }}, 
          but none have coordinates set yet.
        </p>
        <router-link to="/locations" class="btn btn-primary mt-4">
          Add Coordinates to Locations
        </router-link>
      </div>
    </div>
    
    <!-- Map Container -->
    <div 
      v-else
      ref="mapContainer" 
      class="flex-1 w-full rounded-lg shadow-xl border border-base-300"
      style="min-height: 500px"
    ></div>
    
    <!-- Map Controls Info -->
    <div v-if="!loading && !error && locationsWithCoords > 0" class="alert alert-info">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
      </svg>
      <div>
        <span class="font-medium">Map Controls:</span>
        <span class="ml-2">Click markers for details • Scroll to zoom • Drag to pan</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Ensure leaflet popup styling works with dark mode */
:deep(.leaflet-popup-content-wrapper) {
  background: white;
  color: black;
}

:deep(.leaflet-popup-tip) {
  background: white;
}

/* Dark mode adjustments if needed */
[data-theme="dark"] :deep(.leaflet-popup-content-wrapper) {
  background: #1f2937;
  color: white;
}

[data-theme="dark"] :deep(.leaflet-popup-tip) {
  background: #1f2937;
}

/* Ensure map takes full height */
.flex-1 {
  min-height: 0;
}
</style>
