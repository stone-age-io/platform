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

const router = useRouter()
const route = useRoute()
const toast = useToast()
const uiStore = useUIStore()
const { initMap, renderMarkers, updateTheme, cleanup: cleanupMap } = useMap()

const location = ref<Location | null>(null)
const subLocations = ref<Location[]>([])
const things = ref<Thing[]>([])
const loading = ref(true)

const mapContainerId = 'mini-map-container'

// Columns for Sub-Locations List
const subLocColumns: Column<Location>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

// Columns for Things List
const thingColumns: Column<Thing>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

/**
 * Recursive Breadcrumbs Calculation
 */
const breadcrumbs = computed(() => {
  const path: { id: string; name: string }[] = []
  if (!location.value) return path

  path.unshift({ id: location.value.id, name: location.value.name })

  let current = location.value
  while (current.expand?.parent) {
    current = current.expand.parent as Location
    path.unshift({ id: current.id, name: current.name })
  }

  return path
})

/**
 * Highlight JSON Metadata
 */
const highlightedMetadata = computed(() => {
  if (!location.value?.metadata) return '{}'
  const json = JSON.stringify(location.value.metadata, null, 2)
  return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, (match) => {
    let cls = 'text-warning'
    if (/^"/.test(match)) {
      if (/:$/.test(match)) { cls = 'text-primary font-bold' } 
      else { cls = 'text-secondary' }
    } else if (/true|false/.test(match)) { cls = 'text-info' } 
    else if (/null/.test(match)) { cls = 'text-error' }
    return `<span class="${cls}">${match}</span>`
  })
})

/**
 * Load Location Data
 */
async function loadLocation(id: string) {
  // FIX: Explicitly cleanup previous map instance before loading new data
  // This ensures initMap() will create a fresh instance attached to the new DOM element
  cleanupMap()
  
  loading.value = true
  
  try {
    location.value = await pb.collection('locations').getOne<Location>(id, {
      expand: 'type,parent.parent.parent.parent.parent.parent',
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

  // Initialize Map (AFTER loading is false)
  if (location.value?.coordinates?.lat) {
    await nextTick()
    initMap(mapContainerId, uiStore.theme === 'dark')
    renderMarkers([location.value])
  }
}

function openNavigation() {
  if (!location.value?.coordinates) return
  const { lat, lon } = location.value.coordinates
  const url = `https://www.google.com/maps/search/?api=1&query=${lat},${lon}`
  window.open(url, '_blank')
}

function getFloorplanUrl() {
  if (!location.value?.floorplan) return null
  return pb.files.getUrl(location.value, location.value.floorplan)
}

async function handleDelete() {
  if (!location.value) return
  if (!confirm(`Delete "${location.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('locations').delete(location.value.id)
    toast.success('Location deleted')
    router.push('/locations')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete location')
  }
}

// Watchers
watch(() => uiStore.theme, (newTheme) => {
  updateTheme(newTheme === 'dark')
})

watch(
  () => route.params.id,
  (newId) => {
    if (newId) {
      loadLocation(newId as string)
    }
  }
)

onMounted(() => {
  loadLocation(route.params.id as string)
})

onUnmounted(() => {
  cleanupMap()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="location">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <!-- Breadcrumbs -->
        <div class="text-sm breadcrumbs">
          <ul>
            <li><router-link to="/locations">Locations</router-link></li>
            <li v-for="(crumb, index) in breadcrumbs" :key="crumb.id">
              <span v-if="index === breadcrumbs.length - 1" class="font-bold">
                {{ crumb.name }}
              </span>
              <router-link v-else :to="`/locations/${crumb.id}`">
                {{ crumb.name }}
              </router-link>
            </li>
          </ul>
        </div>

        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ location.name || 'Unnamed Location' }}</h1>
            <span v-if="location.expand?.type" class="badge badge-lg badge-ghost">
              {{ location.expand.type.name }}
            </span>
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
      
      <!-- Top Grid: Info & Map -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Basic Info & Metadata -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ location.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Code / Identifier</dt>
                <dd class="mt-1">
                  <code v-if="location.code" class="text-sm bg-base-200 px-1 py-0.5 rounded font-mono">{{ location.code }}</code>
                  <span v-else class="text-sm">-</span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Parent Location</dt>
                <dd class="mt-1">
                  <router-link 
                    v-if="location.expand?.parent" 
                    :to="`/locations/${location.parent}`"
                    class="link link-primary hover:no-underline flex items-center gap-1"
                  >
                    📍 {{ location.expand.parent.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">Top Level</span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(location.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Metadata" v-if="location.metadata && Object.keys(location.metadata).length > 0">
            <div class="bg-base-200 rounded-lg p-4 overflow-x-auto border border-base-300">
              <pre class="text-sm font-mono leading-relaxed" v-html="highlightedMetadata"></pre>
            </div>
          </BaseCard>
        </div>
        
        <!-- Right Column: Coordinates & Floorplan -->
        <div class="space-y-6">
          
          <!-- Geo Coordinates & Map -->
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
              <!-- Coordinate Text -->
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

              <!-- Mini Map -->
              <div class="h-64 w-full rounded-lg overflow-hidden border border-base-300 relative z-0">
                <div :id="mapContainerId" class="absolute inset-0"></div>
              </div>
            </div>

            <div v-else class="text-center py-8 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">📍</span>
              <p class="text-sm">No coordinates set</p>
              <router-link :to="`/locations/${location.id}/edit`" class="btn btn-link btn-xs">Add Coordinates</router-link>
            </div>
          </BaseCard>

          <!-- Floorplan -->
          <BaseCard title="Floorplan" v-if="getFloorplanUrl()">
            <div class="rounded-lg overflow-hidden border border-base-300 bg-base-200">
              <img 
                :src="getFloorplanUrl()!" 
                :alt="`Floorplan for ${location.name}`"
                class="w-full h-auto object-contain"
              />
            </div>
          </BaseCard>
        </div>
      </div>

      <!-- Bottom: Relations (Conditionally Rendered) -->
      <div class="space-y-6" v-if="subLocations.length > 0 || things.length > 0">
        
        <!-- Sub-Locations -->
        <BaseCard 
          v-if="subLocations.length > 0" 
          title="Sub-Locations" 
          :no-padding="true"
        >
          <ResponsiveList
            :items="subLocations"
            :columns="subLocColumns"
            :clickable="true"
            @row-click="(item) => router.push(`/locations/${item.id}`)"
          >
            <template #cell-name="{ item }">
              <div class="font-medium">{{ item.name }}</div>
              <div v-if="item.description" class="text-xs text-base-content/60">{{ item.description }}</div>
            </template>
            <template #cell-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-sm badge-ghost">
                {{ item.expand.type.name }}
              </span>
            </template>
          </ResponsiveList>
        </BaseCard>

        <!-- Associated Things -->
        <BaseCard 
          v-if="things.length > 0" 
          title="Associated Things" 
          :no-padding="true"
        >
          <ResponsiveList
            :items="things"
            :columns="thingColumns"
            :clickable="true"
            @row-click="(item) => router.push(`/things/${item.id}`)"
          >
            <template #cell-name="{ item }">
              <div class="font-medium">{{ item.name }}</div>
              <div v-if="item.description" class="text-xs text-base-content/60">{{ item.description }}</div>
            </template>
            <template #cell-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-sm badge-neutral">
                {{ item.expand.type.name }}
              </span>
            </template>
            <template #cell-code="{ item }">
              <code class="text-xs">{{ item.code || '-' }}</code>
            </template>
          </ResponsiveList>
        </BaseCard>

      </div>
    </template>
  </div>
</template>
