<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useMap } from '@/composables/useMap'
import { useUIStore } from '@/stores/ui'
import { formatDate } from '@/utils/format'
import type { Location, Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import FloorPlanMap from '@/components/map/FloorPlanMap.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const uiStore = useUIStore()
const { initMap, renderMarkers, updateTheme, cleanup: cleanupMap } = useMap()

// State
const location = ref<Location | null>(null)
const subLocations = ref<Location[]>([])
const things = ref<Thing[]>([])
const loading = ref(true)
const isPositioningMode = ref(false)

const mapContainerId = 'mini-map-container'

// List Columns
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
 * Load Location Data
 */
async function loadData(id: string) {
  cleanupMap()
  loading.value = true
  
  try {
    location.value = await pb.collection('locations').getOne<Location>(id, {
      expand: 'type,parent.parent.parent.parent',
    })

    subLocations.value = await pb.collection('locations').getFullList<Location>({
      filter: `parent = "${id}"`,
      expand: 'type',
      sort: 'name',
    })

    things.value = await pb.collection('things').getFullList<Thing>({
      filter: `location = "${id}"`,
      expand: 'type',
      sort: 'name',
    })

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

/**
 * Handle Floor Plan Coordinate Updates
 */
async function handleThingMoved({ id, x, y }: { id: string, x: number, y: number }) {
  try {
    const thing = things.value.find(t => t.id === id)
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
  if (!location.value || !confirm(`Delete "${location.value.name}"?`)) return
  try {
    await pb.collection('locations').delete(location.value.id)
    toast.success('Location deleted')
    router.push('/locations')
  } catch (err: any) {
    toast.error(err.message)
  }
}

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
      
      <!-- Layout Grid (12 Columns) -->
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        
        <!-- Left Column (Metadata & Geo Map) -->
        <div class="lg:col-span-5 flex flex-col gap-6">
          <!-- Basic Information -->
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
                   <span class="text-primary text-xs">📍</span> {{ location.expand.parent.name }}
                </p>
                <p v-else class="opacity-40 italic">Root Location</p>
              </div>
              <div class="col-span-2">
                <p class="opacity-50 uppercase text-xs font-bold mb-1">Created</p>
                <p>{{ formatDate(location.created) }}</p>
              </div>
            </div>
          </BaseCard>

          <!-- Geo Coordinates -->
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
        <div class="lg:col-span-7 h-full">
          <BaseCard class="h-full flex flex-col" :no-padding="true">
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
            
            <div class="flex-grow min-h-[500px]">
              <FloorPlanMap 
                :location="location"
                :things="things"
                :editable="isPositioningMode"
                @thing-moved="handleThingMoved"
                @uploaded="handleFloorplanUploaded"
              />
            </div>
          </BaseCard>
        </div>
      </div>

      <!-- Lists -->
      <div class="space-y-6">
        <BaseCard title="Sub-Locations" :no-padding="true" v-if="subLocations.length">
          <ResponsiveList :items="subLocations" :columns="subLocColumns" @row-click="(i) => router.push(`/locations/${i.id}`)" />
        </BaseCard>

        <BaseCard title="Associated Things" :no-padding="true">
          <ResponsiveList :items="things" :columns="thingColumns" @row-click="(i) => router.push(`/things/${i.id}`)">
            <template #cell-code="{ item }"><code class="text-xs font-mono">{{ item.code || '-' }}</code></template>
          </ResponsiveList>
        </BaseCard>
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
