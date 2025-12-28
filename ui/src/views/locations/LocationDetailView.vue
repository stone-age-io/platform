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
      <!-- Header -->
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
            <router-link :to="`/locations/${location.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
          </div>
        </div>
      </div>
      
      <!-- Section 1: Info & Geo Map -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <div class="space-y-6">
          <!-- Basic Info -->
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ location.description || '-' }}</dd>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Code</dt>
                  <dd class="mt-1 font-mono text-sm">{{ location.code || '-' }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Parent</dt>
                  <dd class="mt-1 text-sm">
                    <router-link v-if="location.expand?.parent" :to="`/locations/${location.parent}`" class="link link-primary">
                      📍 {{ location.expand.parent.name }}
                    </router-link>
                    <span v-else class="opacity-40">Root</span>
                  </dd>
                </div>
              </div>
              <!-- Created Timestamp Restored -->
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(location.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <!-- Geo Coordinates Card with Restored Header Button -->
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Geo Coordinates</h3>
                <button 
                  v-if="location.coordinates"
                  @click="openNavigation"
                  class="btn btn-sm btn-primary btn-outline gap-2"
                >
                  <span class="text-lg">🗺️</span>
                  Navigate
                </button>
              </div>
            </template>

            <div v-if="location.coordinates" class="space-y-4">
              <div class="grid grid-cols-2 gap-3">
                <div class="bg-base-200 rounded-lg p-2 border border-base-300 text-center">
                  <span class="text-xs text-base-content/50 uppercase block">Lat</span>
                  <span class="font-mono text-sm">{{ location.coordinates.lat }}</span>
                </div>
                <div class="bg-base-200 rounded-lg p-2 border border-base-300 text-center">
                  <span class="text-xs text-base-content/50 uppercase block">Lon</span>
                  <span class="font-mono text-sm">{{ location.coordinates.lon }}</span>
                </div>
              </div>
              <div class="h-48 w-full rounded-lg overflow-hidden border border-base-300 relative z-0">
                <div :id="mapContainerId" class="absolute inset-0"></div>
              </div>
            </div>
            <div v-else class="text-center py-8 opacity-50 border border-dashed border-base-300 rounded-lg bg-base-200/30">
              No coordinates set.
            </div>
          </BaseCard>
        </div>

        <!-- Section 2: Floor Plan Map -->
        <div class="space-y-4">
          <div class="flex justify-between items-center">
            <h2 class="text-xl font-bold">Floor Plan</h2>
            <button 
              v-if="location.floorplan"
              @click="isPositioningMode = !isPositioningMode" 
              class="btn btn-sm"
              :class="isPositioningMode ? 'btn-success' : 'btn-outline'"
            >
              {{ isPositioningMode ? '💾 Save Positions' : '🛠️ Position Things' }}
            </button>
          </div>

          <FloorPlanMap 
            :location="location"
            :things="things"
            :editable="isPositioningMode"
            @thing-moved="handleThingMoved"
            @uploaded="handleFloorplanUploaded"
          />
        </div>
      </div>

      <!-- Section 3: Lists -->
      <div class="grid grid-cols-1 gap-6">
        <BaseCard title="Sub-Locations" :no-padding="true" v-if="subLocations.length">
          <ResponsiveList :items="subLocations" :columns="subLocColumns" @row-click="(i) => router.push(`/locations/${i.id}`)" />
        </BaseCard>

        <BaseCard title="Things at this Location" :no-padding="true">
          <ResponsiveList :items="things" :columns="thingColumns" @row-click="(i) => router.push(`/things/${i.id}`)">
            <template #cell-code="{ item }"><code class="text-xs">{{ item.code || '-' }}</code></template>
          </ResponsiveList>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
