================================================
FILE: ui/src/views/locations/LocationDetailView.vue
================================================
<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useMap } from '@/composables/useMap'
import { usePagination } from '@/composables/usePagination'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { formatDate } from '@/utils/format'
import type { Location, Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import FloorPlanMap from '@/components/map/FloorPlanMap.vue'
import KvDashboard from '@/components/nats/KvDashboard.vue'
import OccupancyList from '@/components/locations/OccupancyList.vue' // NEW IMPORT

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const { initMap, renderMarkers, updateTheme, cleanup: cleanupMap } = useMap()

// --- State ---
const location = ref<Location | null>(null)
const loading = ref(true)
const isPositioningMode = ref(false)
const mapContainerId = 'mini-map-container'

// --- Pagination & Search: Sub-Locations ---
const subLocSearch = ref('')
const {
  items: subLocations,
  page: subLocPage,
  totalPages: subLocTotalPages,
  totalItems: subLocTotalItems,
  loading: subLocLoading,
  load: loadSubLocs,
  nextPage: nextSubLocPage,
  prevPage: prevSubLocPage,
} = usePagination<Location>('locations', 5)

// --- Pagination & Search: Things ---
const thingSearch = ref('')
const {
  items: things,
  page: thingPage,
  totalPages: thingTotalPages,
  totalItems: thingTotalItems,
  loading: thingLoading,
  load: loadThings,
  nextPage: nextThingPage,
  prevPage: prevThingPage,
} = usePagination<Thing>('things', 10)

// --- List Columns ---
const subLocColumns: Column<Location>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

const thingColumns: Column<Thing>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

/**
 * Breadcrumbs
 */
const breadcrumbs = computed(() => {
  const path: { id: string; name: string }[] = []
  if (!location.value) return path
  path.unshift({ id: location.value.id, name: location.value.name || 'Unnamed' })
  let current = location.value
  while (current.expand?.parent) {
    current = current.expand.parent as Location
    path.unshift({ id: current.id, name: current.name || 'Unnamed' })
  }
  return path
})

/**
 * Filter Logic
 */
const subLocFilter = computed(() => {
  if (!location.value) return ''
  let f = `parent = "${location.value.id}"`
  const q = subLocSearch.value.trim().toLowerCase()
  if (q) {
    f += ` && (name ~ "${q}" || code ~ "${q}")`
  }
  return f
})

const thingFilter = computed(() => {
  if (!location.value) return ''
  let f = `location = "${location.value.id}"`
  const q = thingSearch.value.trim().toLowerCase()
  if (q) {
    f += ` && (name ~ "${q}" || code ~ "${q}")`
  }
  return f
})

// All things (unpaginated) for the Floor Plan map
const allThingsForMap = ref<Thing[]>([])

/**
 * Main Load Function
 */
async function loadData(id: string) {
  cleanupMap()
  loading.value = true
  
  try {
    // 1. Load Location Details
    location.value = await pb.collection('locations').getOne<Location>(id, {
      expand: 'type,parent.parent.parent.parent',
    })

    // 2. Load Lists (Paginated)
    await Promise.all([
      refreshSubLocs(),
      refreshThings(),
      // 3. Load Map Data (All items, strictly for coordinates)
      pb.collection('things').getFullList<Thing>({
        filter: `location = "${id}"`,
        fields: 'id,name,metadata,expand.type.name', // optimize fetch
        expand: 'type'
      }).then(res => allThingsForMap.value = res)
    ])

  } catch (err: any) {
    toast.error(err.message || 'Failed to load location')
    router.push('/locations')
    return
  } finally {
    loading.value = false
  }

  if (location.value?.coordinates?.lat) {
    await nextTick()
    initMap(mapContainerId, uiStore.theme === 'dark')
    renderMarkers([location.value])
  }
}

// Helpers to refresh lists with current filters
async function refreshSubLocs() {
  await loadSubLocs({ filter: subLocFilter.value, expand: 'type', sort: 'name' })
}

async function refreshThings() {
  await loadThings({ filter: thingFilter.value, expand: 'type', sort: 'name' })
}

/**
 * Handle Floor Plan Coordinate Updates
 */
async function handleThingMoved({ id, x, y }: { id: string, x: number, y: number }) {
  try {
    // Update local map data
    const thing = allThingsForMap.value.find(t => t.id === id)
    if (!thing) return

    const metadata = { ...(thing.metadata || {}), x, y }
    await pb.collection('things').update(id, { metadata })
    thing.metadata = metadata
    
    toast.success(`${thing.name} position saved`)
  } catch (err: any) {
    toast.error('Failed to save position')
  }
}

function handleFloorplanUploaded(updatedLoc: Location) {
  location.value = updatedLoc
  toast.success('Floor plan uploaded')
}

function openNavigation() {
  if (!location.value?.coordinates) return
  const { lat, lon } = location.value.coordinates
  window.open(`https://www.google.com/maps/search/?api=1&query=${lat},${lon}`, '_blank')
}

async function handleDelete() {
  if (!location.value) return
  const confirmed = await confirm({
    title: 'Delete Location',
    message: `Are you sure you want to delete "${location.value.name}"?`,
    details: 'Sub-locations and things at this location will not be deleted.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    await pb.collection('locations').delete(location.value.id)
    toast.success('Location deleted')
    router.push('/locations')
  } catch (err: any) {
    toast.error(err.message)
  }
}

// Watchers for Search
let subLocTimeout: any
watch(subLocSearch, () => {
  clearTimeout(subLocTimeout)
  subLocTimeout = setTimeout(() => {
    subLocPage.value = 1
    refreshSubLocs()
  }, 300)
})

let thingTimeout: any
watch(thingSearch, () => {
  clearTimeout(thingTimeout)
  thingTimeout = setTimeout(() => {
    thingPage.value = 1
    refreshThings()
  }, 300)
})

watch(() => uiStore.theme, (newTheme) => updateTheme(newTheme === 'dark'))
watch(() => route.params.id, (newId) => { if (newId) loadData(newId as string) })

onMounted(() => loadData(route.params.id as string))
onUnmounted(() => cleanupMap())
</script>

<template>
  <div class="space-y-6">
    <!-- Loading -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="location">
      <!-- Standard Detail Header -->
      <div class="flex flex-col gap-4">
        <div class="text-sm breadcrumbs">
          <ul>
            <li><router-link to="/locations">Locations</router-link></li>
            <li v-for="(crumb, index) in breadcrumbs" :key="crumb.id">
              <span v-if="index === breadcrumbs.length - 1" class="font-bold">{{ crumb.name }}</span>
              <router-link v-else :to="`/locations/${crumb.id}`">{{ crumb.name }}</router-link>
            </li>
          </ul>
        </div>

        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ location.name || 'Unnamed' }}</h1>
            <span v-if="location.expand?.type" class="badge badge-lg badge-ghost">{{ location.expand.type.name }}</span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/locations/${location.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <!-- Layout Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        
        <!-- Left Column (Metadata & Geo Map) -->
        <div class="lg:col-span-5 flex flex-col gap-6">
          <BaseCard title="Basic Information">
            <div class="grid grid-cols-2 gap-y-4 text-sm">
              <div class="col-span-2">
                <p class="opacity-50 uppercase text-xs font-bold mb-1">Description</p>
                <p class="text-base">{{ location.description || 'No description provided' }}</p>
              </div>
              <div>
                <p class="opacity-50 uppercase text-xs font-bold mb-1">Code</p>
                <code class="bg-base-300 px-2 py-1 rounded font-mono text-xs">{{ location.code || 'N/A' }}</code>
              </div>
              <div>
                <p class="opacity-50 uppercase text-xs font-bold mb-1">Parent</p>
                <p v-if="location.expand?.parent" class="flex items-center gap-1">
                   <span class="text-primary text-xs">📍</span>
                   <router-link :to="`/locations/${location.parent}`" class="link link-primary font-medium hover:no-underline">
                     {{ location.expand.parent.name }}
                   </router-link>
                </p>
                <p v-else class="opacity-40 italic">Root Location</p>
              </div>
              <div class="col-span-2">
                <p class="opacity-50 uppercase text-xs font-bold mb-1">Created</p>
                <p>{{ formatDate(location.created) }}</p>
              </div>
            </div>
          </BaseCard>

          <BaseCard :no-padding="true" class="overflow-hidden">
            <div class="p-4 border-b border-base-300 flex justify-between items-center bg-base-200/30">
              <h2 class="font-bold uppercase text-[10px] tracking-widest opacity-60">Geo Location</h2>
              <button v-if="location.coordinates" @click="openNavigation" class="btn btn-xs btn-primary btn-outline">
                Navigate
              </button>
            </div>

            <div v-if="location.coordinates" class="p-4 space-y-4">
              <div class="grid grid-cols-2 gap-2 text-center">
                <div class="bg-base-200 rounded p-2">
                  <span class="block text-[10px] uppercase opacity-50 font-bold">Latitude</span>
                  <span class="font-mono text-xs">{{ location.coordinates.lat }}</span>
                </div>
                <div class="bg-base-200 rounded p-2">
                  <span class="block text-[10px] uppercase opacity-50 font-bold">Longitude</span>
                  <span class="font-mono text-xs">{{ location.coordinates.lon }}</span>
                </div>
              </div>
              <div class="h-44 w-full rounded-lg relative z-0 border border-base-300">
                <div :id="mapContainerId" class="absolute inset-0 rounded-lg"></div>
              </div>
            </div>
            <div v-else class="p-8 text-center opacity-40 italic text-sm">
              No geographic coordinates set.
            </div>
          </BaseCard>
        </div>

        <!-- Right Column (Floor Plan Visualizer) -->
        <div class="lg:col-span-7 flex flex-col gap-6">
          <BaseCard class="flex flex-col h-full min-h-[550px]" :no-padding="true">
            <div class="p-4 border-b border-base-300 flex justify-between items-center bg-base-200/30">
              <h2 class="font-bold uppercase text-[10px] tracking-widest opacity-60 flex items-center gap-2">
                 🖼️ Floor Plan Map
              </h2>
              <button 
                v-if="location.floorplan"
                @click="isPositioningMode = !isPositioningMode" 
                class="btn btn-xs sm:btn-sm"
                :class="isPositioningMode ? 'btn-success' : 'btn-ghost bg-base-300'"
              >
                {{ isPositioningMode ? '💾 Save Positions' : '🛠️ Position Things' }}
              </button>
            </div>
            
            <div class="flex-grow">
              <FloorPlanMap 
                :location="location"
                :things="allThingsForMap" 
                :editable="isPositioningMode"
                @thing-moved="handleThingMoved"
                @uploaded="handleFloorplanUploaded"
              />
            </div>
          </BaseCard>
        </div>
      </div>

      <!-- Lower Section: Lists & Digital Twin -->
      <div class="space-y-6">
        
        <!-- 2. Sub-Locations List -->
        <BaseCard :no-padding="true" v-if="subLocations.length > 0 || subLocSearch || subLocLoading">
          <template #header>
            <div class="flex justify-between items-center mb-2">
              <h3 class="card-title">Sub-Locations</h3>
              <!-- Search Input -->
              <input 
                v-model="subLocSearch"
                type="text"
                placeholder="Search..."
                class="input input-xs input-bordered w-32 md:w-48"
              />
            </div>
          </template>

          <ResponsiveList 
            :items="subLocations" 
            :columns="subLocColumns" 
            :loading="subLocLoading"
            @row-click="(i) => router.push(`/locations/${i.id}`)" 
          />

          <!-- Pagination -->
          <div v-if="subLocTotalItems > 0" class="flex justify-between items-center p-3 border-t border-base-200 bg-base-100/50">
            <span class="text-xs text-base-content/60">
              {{ subLocTotalItems }} items
            </span>
            <div class="join">
              <button class="join-item btn btn-xs" :disabled="subLocPage === 1" @click="prevSubLocPage({ filter: subLocFilter, expand: 'type', sort: 'name' })">«</button>
              <button class="join-item btn btn-xs cursor-default">Page {{ subLocPage }}</button>
              <button class="join-item btn btn-xs" :disabled="subLocPage === subLocTotalPages" @click="nextSubLocPage({ filter: subLocFilter, expand: 'type', sort: 'name' })">»</button>
            </div>
          </div>
        </BaseCard>

        <!-- 3. Associated Things List -->
        <BaseCard :no-padding="true" v-if="things.length > 0 || thingSearch || thingLoading">
          <template #header>
            <div class="flex justify-between items-center mb-2">
              <h3 class="card-title">Associated Things</h3>
              <!-- Search Input -->
              <input 
                v-model="thingSearch"
                type="text"
                placeholder="Search..."
                class="input input-xs input-bordered w-32 md:w-48"
              />
            </div>
          </template>

          <ResponsiveList 
            :items="things" 
            :columns="thingColumns" 
            :loading="thingLoading"
            @row-click="(i) => router.push(`/things/${i.id}`)"
          >
            <template #cell-code="{ item }"><code class="text-xs font-mono">{{ item.code || '-' }}</code></template>
          </ResponsiveList>

          <!-- Pagination -->
          <div v-if="thingTotalItems > 0" class="flex justify-between items-center p-3 border-t border-base-200 bg-base-100/50">
            <span class="text-xs text-base-content/60">
              {{ thingTotalItems }} items
            </span>
            <div class="join">
              <button class="join-item btn btn-xs" :disabled="thingPage === 1" @click="prevThingPage({ filter: thingFilter, expand: 'type', sort: 'name' })">«</button>
              <button class="join-item btn btn-xs cursor-default">Page {{ thingPage }}</button>
              <button class="join-item btn btn-xs" :disabled="thingPage === thingTotalPages" @click="nextThingPage({ filter: thingFilter, expand: 'type', sort: 'name' })">»</button>
            </div>
          </div>
        </BaseCard>

        <!-- Live Occupancy -->
        <template v-if="location.code">
          <div v-if="natsStore.isConnected" class="mt-6">
            <OccupancyList :location-code="location.code" />
          </div>
          <div v-else class="alert shadow-sm border border-base-300 bg-base-100">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
            <div class="text-xs">
              <div class="font-bold">Occupancy Offline</div>
              <span class="text-base-content/70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live data for this location.</span>
            </div>
          </div>
        </template>

        <!-- Digital Twin / KV Dashboard -->
        <template v-if="location.code">
          <div v-if="natsStore.isConnected" class="mt-6">
            <KvDashboard
              :key="location.code"
              :base-key="`location.${location.code}`"
            />
          </div>
          <div v-else class="alert shadow-sm border border-base-300 bg-base-100">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
            <div class="text-xs">
              <div class="font-bold">Digital Twin Offline</div>
              <span class="text-base-content/70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live data for this location.</span>
            </div>
          </div>
        </template>

      </div>
    </template>
  </div>
</template>

<style scoped>
/* Grug UX: Invert floorplan colors in dark mode for better visual cohesion */
:deep([data-theme='dark']) .leaflet-image-layer {
  filter: invert(0.9) hue-rotate(180deg) brightness(0.8) contrast(1.2);
}
</style>
