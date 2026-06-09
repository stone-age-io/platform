================================================
FILE: ui/src/views/locations/LocationDetailView.vue
================================================
<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useLeafletMap } from '@/composables/useLeafletMap'
import { usePagination } from '@/composables/usePagination'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { formatDate } from '@/utils/format'
import JsonViewer from '@/components/common/JsonViewer.vue'
import type { Location, Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import FloorPlanMap from '@/components/map/FloorPlanMap.vue'
import KvDashboard from '@/components/nats/KvDashboard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

async function copyMetadata() {
  if (!location.value?.metadata) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(location.value.metadata, null, 2))
    toast.success('Metadata copied')
  } catch {
    toast.error('Failed to copy')
  }
}
const uiStore = useUIStore()
const natsStore = useNatsStore()
const { initMap, renderMarkers, updateTheme, invalidateSize, cleanup: cleanupMap } = useLeafletMap()

// --- State ---
const location = ref<Location | null>(null)
const loading = ref(true)
const mapContainerId = 'mini-map-container'
const activeTab = ref<'floorplan' | 'coordinates'>('floorplan')
const miniMapInitialized = ref(false)

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
  miniMapInitialized.value = false
  loading.value = true

  try {
    // 1. Load Location Details
    location.value = await pb.collection('locations').getOne<Location>(id, {
      expand: 'type,parent.parent.parent.parent',
    })

    // Pick the default tab based on what data exists. Floorplan wins when both are set.
    if (location.value?.floorplan) activeTab.value = 'floorplan'
    else if (location.value?.coordinates) activeTab.value = 'coordinates'

    // 2. Load Lists (Paginated)
    await Promise.all([
      refreshSubLocs(),
      refreshThings(),
      // 3. Load Map Data (All items, strictly for coordinates)
      pb.collection('things').getFullList<Thing>({
        filter: `location = "${id}"`,
        fields: 'id,name,description,code,floorplan_position,expand.type.name,expand.location.id,expand.location.name', // optimize fetch
        expand: 'type,location'
      }).then(res => allThingsForMap.value = res)
    ])

  } catch (err: any) {
    toast.error(err.message || 'Failed to load location')
    router.push('/locations')
    return
  } finally {
    loading.value = false
  }

}

// Lazily init the mini-map once the coordinates tab is actually in the DOM.
// The outer detail region is gated by v-else-if="location" (unmounts while `loading`),
// and Leaflet needs both a mounted container AND a non-hidden panel to render.
watch([() => location.value, activeTab, loading], async ([loc, tab, isLoading]) => {
  if (isLoading || !loc?.coordinates?.lat || tab !== 'coordinates') return
  await nextTick()
  if (!miniMapInitialized.value) {
    initMap(mapContainerId, {
      isDarkMode: uiStore.theme === 'dark',
      center: { lat: loc.coordinates.lat, lon: loc.coordinates.lon },
      zoom: 15,
    })
    const typeName = (loc.expand?.type as { name?: string } | undefined)?.name || 'Unknown Type'
    const popupHtml = `
      <div class="p-1">
        <h3 class="font-bold text-sm">${loc.name}</h3>
        <div class="text-xs text-gray-500 mb-1">${typeName}</div>
        ${loc.description ? `<p class="text-xs mb-2">${loc.description}</p>` : ''}
        <a href="/locations/${loc.id}" class="text-xs text-primary hover:underline">View Details</a>
      </div>
    `
    renderMarkers([{
      id: loc.id,
      lat: loc.coordinates.lat,
      lon: loc.coordinates.lon,
      label: loc.name,
      popupHtml,
    }])
    miniMapInitialized.value = true
  } else {
    invalidateSize()
  }
}, { immediate: true })

// Helpers to refresh lists with current filters
async function refreshSubLocs() {
  await loadSubLocs({ filter: subLocFilter.value, expand: 'type', sort: 'name' })
}

async function refreshThings() {
  await loadThings({ filter: thingFilter.value, expand: 'type', sort: 'name' })
}

/**
 * Persist a thing's floor-plan position. A null position un-maps it from the plan.
 */
async function handleUpdatePosition({ id, position }: { id: string, position: { x: number, y: number } | null }) {
  try {
    // Update local map data
    const thing = allThingsForMap.value.find(t => t.id === id)
    if (!thing) return

    await pb.collection('things').update(id, { floorplan_position: position })
    thing.floorplan_position = position ?? undefined

    toast.success(position ? `${thing.name} position saved` : `${thing.name} removed from plan`)
  } catch (err: any) {
    toast.error('Failed to save position')
  }
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
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ location.description || '-' }}</dd>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Code</dt>
                  <dd class="mt-1">
                    <code v-if="location.code" class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ location.code }}</code>
                    <span v-else class="text-sm text-base-content/40">-</span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Parent</dt>
                  <dd class="mt-1">
                    <router-link v-if="location.expand?.parent" :to="`/locations/${location.parent}`" class="link link-primary hover:no-underline flex items-center gap-1">
                      📍 {{ location.expand.parent.name }}
                    </router-link>
                    <span v-else class="text-sm text-base-content/40">Root location</span>
                  </dd>
                </div>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(location.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <!-- Metadata -->
          <BaseCard v-if="location.metadata && Object.keys(location.metadata).length > 0">
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Metadata</h3>
                <button @click="copyMetadata" class="btn btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100" title="Copy raw JSON">
                  📋 Copy
                </button>
              </div>
            </template>

            <div class="bg-base-200 rounded-lg p-4 border border-base-300 overflow-hidden">
              <div class="max-h-[500px] overflow-y-auto overflow-x-auto custom-scrollbar">
                <JsonViewer :data="location.metadata" class="text-sm leading-relaxed" />
              </div>
            </div>
          </BaseCard>

        </div>

        <!-- Right Column: Combined Location Map card (floorplan and/or coordinates) -->
        <div class="lg:col-span-7 flex flex-col gap-6">
          <BaseCard class="flex flex-col h-full min-h-[550px]" :no-padding="true">
            <div class="p-4 border-b border-base-300 flex justify-between items-center bg-base-200/30 gap-3">
              <!-- Tabs when both exist; static label otherwise -->
              <div v-if="location.floorplan && location.coordinates" role="tablist" class="tabs tabs-bordered -mb-[17px]">
                <a
                  role="tab"
                  class="tab text-xs sm:text-sm"
                  :class="{ 'tab-active font-bold': activeTab === 'floorplan' }"
                  @click="activeTab = 'floorplan'"
                >🖼️ Floor Plan</a>
                <a
                  role="tab"
                  class="tab text-xs sm:text-sm"
                  :class="{ 'tab-active font-bold': activeTab === 'coordinates' }"
                  @click="activeTab = 'coordinates'"
                >📍 Geo Map</a>
              </div>
              <h2 v-else class="font-bold uppercase text-[10px] tracking-widest opacity-60 flex items-center gap-2">
                {{ location.floorplan ? '🖼️ Floor Plan' : location.coordinates ? '📍 Geo Location' : '🗺️ Location' }}
              </h2>
            </div>

            <!-- Floorplan panel (overlay controls + drawers live inside FloorPlanMap) -->
            <div v-show="location.floorplan && activeTab === 'floorplan'" class="flex-grow flex flex-col">
              <FloorPlanMap
                v-if="location.floorplan"
                :location="location"
                :things="allThingsForMap"
                @update-position="handleUpdatePosition"
              />
            </div>

            <!-- Coordinates panel -->
            <div v-show="location.coordinates && activeTab === 'coordinates'" class="p-4 flex-grow flex flex-col gap-4">
              <div v-if="location.coordinates" class="grid grid-cols-2 gap-2 text-center">
                <div class="bg-base-200 rounded p-2">
                  <span class="block text-[10px] uppercase opacity-50 font-bold">Latitude</span>
                  <span class="font-mono text-xs">{{ location.coordinates.lat }}</span>
                </div>
                <div class="bg-base-200 rounded p-2">
                  <span class="block text-[10px] uppercase opacity-50 font-bold">Longitude</span>
                  <span class="font-mono text-xs">{{ location.coordinates.lon }}</span>
                </div>
              </div>
              <div class="flex-grow min-h-[300px] w-full rounded-lg relative z-0 border border-base-300 overflow-hidden">
                <div :id="mapContainerId" class="absolute inset-0 rounded-lg"></div>
                <div v-if="location.coordinates" class="absolute top-[10px] right-[10px] z-[400]">
                  <button
                    @click="openNavigation"
                    class="btn btn-sm bg-base-100 border-base-300 shadow-sm hover:bg-base-200 gap-1"
                    title="Open in Google Maps"
                  >
                    🧭 <span class="hidden sm:inline">Navigate</span>
                  </button>
                </div>
              </div>
            </div>

            <!-- Empty state: neither floorplan nor coordinates -->
            <div v-if="!location.floorplan && !location.coordinates" class="flex-grow flex flex-col items-center justify-center p-8 text-center">
              <span class="text-6xl mb-4 opacity-30">🗺️</span>
              <h3 class="font-bold opacity-60">No Map or Floor Plan</h3>
              <p class="text-sm opacity-50 mb-4 max-w-sm">Add geographic coordinates or upload a floor plan image to visualize this location.</p>
              <router-link :to="`/locations/${location.id}/edit`" class="btn btn-sm btn-primary">
                Edit Location
              </router-link>
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
/* Highlight the floor-plan marker for the selected thing (mirrors the map-viz views) */
:deep(.marker-selected) {
  filter: hue-rotate(180deg) saturate(1.5) drop-shadow(0 0 8px rgba(116, 128, 255, 0.8));
}
</style>
