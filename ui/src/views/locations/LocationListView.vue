<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useNatsStore } from '@/stores/nats'
import { useOccupancySummary } from '@/composables/useOccupancySummary'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Location } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import LocationMapViz from '@/components/locations/LocationMapViz.vue'
import OccupancyList from '@/components/locations/OccupancyList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()
const natsStore = useNatsStore()

// View Mode
const viewMode = ref<'list' | 'map' | 'occupancy'>('list')

// Data State
const allLocations = ref<Location[]>([])
const loading = ref(false)
const searchQuery = ref('')

// Pagination State (Client-Side)
const currentPage = ref(1)
const itemsPerPage = 20

// --- Occupancy ---
const selectedOccupancyCode = ref<string | null>(null)

// Locations with codes for the summary composable
const knownCodes = computed(() => allLocations.value.map(l => l.code!).filter(Boolean))
const { counts: occupancyCounts, loading: countsLoading } = useOccupancySummary(knownCodes)

const totalOccupants = computed(() => {
  let total = 0
  for (const count of occupancyCounts.value.values()) total += count
  return total
})

// Occupancy: only locations with codes, sorted by count descending
const occupancySearchQuery = ref('')
const occupancyPage = ref(1)
const occupancyPerPage = 10

const filteredOccupancyLocations = computed(() => {
  let result = allLocations.value.filter(l => l.code)
  const q = occupancySearchQuery.value.toLowerCase().trim()

  if (q) {
    result = result.filter(l => {
      const nameMatch = l.name?.toLowerCase().includes(q)
      const codeMatch = l.code?.toLowerCase().includes(q)
      const typeMatch = l.expand?.type?.name?.toLowerCase().includes(q)
      const parentMatch = (l.expand?.parent as Location)?.name?.toLowerCase().includes(q)
      return nameMatch || codeMatch || typeMatch || parentMatch
    })
  }

  return [...result].sort((a, b) => {
    const countA = occupancyCounts.value.get(a.code!) || 0
    const countB = occupancyCounts.value.get(b.code!) || 0
    if (countA !== countB) return countB - countA
    return (a.name || '').localeCompare(b.name || '')
  })
})

const occupancyTotalPages = computed(() => Math.ceil(filteredOccupancyLocations.value.length / occupancyPerPage))
const occupancyRangeStart = computed(() => (occupancyPage.value - 1) * occupancyPerPage + 1)
const occupancyRangeEnd = computed(() => Math.min(occupancyPage.value * occupancyPerPage, filteredOccupancyLocations.value.length))

const paginatedOccupancyLocations = computed(() => {
  const start = (occupancyPage.value - 1) * occupancyPerPage
  return filteredOccupancyLocations.value.slice(start, start + occupancyPerPage)
})

watch(occupancySearchQuery, () => { occupancyPage.value = 1 })
watch(occupancyTotalPages, (pages) => {
  if (occupancyPage.value > pages && pages > 0) occupancyPage.value = pages
})

function getOccupancyCount(code?: string): number {
  if (!code) return 0
  return occupancyCounts.value.get(code) || 0
}

function handleOccupancyRowClick(location: Location) {
  selectedOccupancyCode.value = selectedOccupancyCode.value === location.code ? null : (location.code || null)
}

const selectedOccupancyLocation = computed(() => {
  if (!selectedOccupancyCode.value) return null
  return allLocations.value.find(l => l.code === selectedOccupancyCode.value) || null
})

const occupancyColumns: Column<Location>[] = [
  { key: 'name', label: 'Location', mobileLabel: 'Location' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'occupancy', label: 'Occupants', mobileLabel: 'Occupants' },
]

// --- List View ---

/**
 * Filter Logic (Client-Side)
 * Searches Name, Code, Description, Type Name, and Parent Name
 */
const filteredLocations = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()

  if (!q) {
    // Default: Root level only when not searching
    return allLocations.value.filter(l => !l.parent)
  }

  // Flattened search
  return allLocations.value.filter(l => {
    const nameMatch = l.name?.toLowerCase().includes(q)
    const codeMatch = l.code?.toLowerCase().includes(q)
    const descMatch = l.description?.toLowerCase().includes(q)
    const typeMatch = l.expand?.type?.name?.toLowerCase().includes(q)
    const parentMatch = (l.expand?.parent as Location)?.name?.toLowerCase().includes(q)

    return nameMatch || codeMatch || descMatch || typeMatch || parentMatch
  })
})

/**
 * Pagination Logic (Client-Side)
 */
const paginatedLocations = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredLocations.value.slice(start, end)
})

const totalPages = computed(() => Math.ceil(filteredLocations.value.length / itemsPerPage))

/**
 * Load All Locations
 */
async function loadLocations() {
  loading.value = true
  try {
    // Fetch EVERYTHING.
    // Optimization: We only need specific fields + expansions.
    allLocations.value = await pb.collection('locations').getFullList<Location>({
      sort: 'name',
      expand: 'type,parent',
    })
  } catch (err: any) {
    console.error('Failed to load locations', err)
    toast.error('Failed to load locations')
  } finally {
    loading.value = false
  }
}

// Columns
const columns: Column<Location>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'expand.parent.name', label: 'Parent', mobileLabel: 'Parent', class: 'hidden md:table-cell' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (value) => formatDate(value, 'PP') },
]

// Actions
function handleRowClick(location: Location) {
  router.push(`/locations/${location.id}`)
}

async function handleDelete(location: Location) {
  const confirmed = await confirm({
    title: 'Delete Location',
    message: `Are you sure you want to delete "${location.name}"?`,
    details: 'Sub-locations and things at this location will not be deleted but will lose their parent reference.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    await pb.collection('locations').delete(location.id)
    toast.success('Location deleted')
    // Remove locally to avoid reload
    allLocations.value = allLocations.value.filter(l => l.id !== location.id)
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete location')
  }
}

function handleOrgChange() {
  selectedOccupancyCode.value = null
  loadLocations()
}

// Reset page on search
watch(searchQuery, () => {
  currentPage.value = 1
})

onMounted(() => {
  loadLocations()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Controls -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">

      <!-- Left Side: Title & Mobile Toggle -->
      <div class="w-full sm:w-auto">
        <div class="flex justify-between items-center">
          <h1 class="text-3xl font-bold">Locations</h1>

          <!-- Mobile Toggle: Next to title -->
          <div class="join shadow-sm border border-base-300 sm:hidden">
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'list' }"
              @click="viewMode = 'list'"
            >
              📋
            </button>
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'map' }"
              @click="viewMode = 'map'"
            >
              🗺️
            </button>
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'occupancy' }"
              @click="viewMode = 'occupancy'"
            >
              👥
            </button>
          </div>
        </div>
        <p class="text-base-content/70 mt-1">
          Manage physical sites and facilities
        </p>
      </div>

      <!-- Right Side: Desktop Toggle & New Button -->
      <div class="flex gap-3 w-full sm:w-auto">

        <!-- Desktop Toggle: Hidden on mobile -->
        <div class="hidden sm:inline-flex join shadow-sm border border-base-300">
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'list' }"
            @click="viewMode = 'list'"
          >
            📋 List
          </button>
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'map' }"
            @click="viewMode = 'map'"
          >
            🗺️ Map
          </button>
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'occupancy' }"
            @click="viewMode = 'occupancy'"
          >
            👥 Occupancy
          </button>
        </div>

        <router-link to="/locations/new" class="btn btn-primary w-full sm:w-auto">
          <span class="text-lg">+</span>
          <span>New Location</span>
        </router-link>
      </div>
    </div>

    <!-- LIST VIEW -->
    <div v-if="viewMode === 'list'" class="space-y-6 fade-in">
      <!-- Search -->
      <div class="form-control">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by name, type, code, or parent..."
          class="input input-bordered w-full"
        />
        <label v-if="!searchQuery" class="label">
          <span class="label-text-alt text-base-content/60">
            Showing top-level locations. Use search to find sub-locations.
          </span>
        </label>
        <label v-else class="label">
          <span class="label-text-alt text-base-content/60">
            Searching all {{ allLocations.length }} locations.
          </span>
        </label>
      </div>

      <!-- List Card -->
      <BaseCard :no-padding="true">
        <div v-if="loading && allLocations.length === 0" class="flex justify-center p-12">
          <span class="loading loading-spinner loading-lg"></span>
        </div>

        <div v-else-if="allLocations.length === 0" class="text-center py-12">
          <span class="text-6xl">📍</span>
          <h3 class="text-xl font-bold mt-4">No locations found</h3>
          <p class="text-base-content/70 mt-2">
            Create your first root location to get started.
          </p>
        </div>

        <div v-else-if="filteredLocations.length === 0" class="text-center py-12">
          <span class="text-6xl">🔍</span>
          <h3 class="text-xl font-bold mt-4">No matching locations</h3>
          <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
        </div>

        <template v-else>
          <ResponsiveList
            :items="paginatedLocations"
            :columns="columns"
            :clickable="true"
            @row-click="handleRowClick"
          >
            <template #cell-name="{ item }">
              <div>
                <div class="font-medium">{{ item.name }}</div>
                <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
              </div>
            </template>
            <template #card-name="{ item }">
              <div>
                <div class="font-semibold text-base">{{ item.name }}</div>
                <div v-if="item.description" class="text-sm text-base-content/60 mt-1">{{ item.description }}</div>
              </div>
            </template>

            <template #cell-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-ghost">{{ item.expand.type.name }}</span>
              <span v-else class="text-base-content/40">-</span>
            </template>

            <template #cell-expand.parent.name="{ item }">
              <span v-if="item.expand?.parent" class="text-sm opacity-80">{{ item.expand.parent.name }}</span>
              <span v-else class="text-base-content/30 text-xs italic">Root</span>
            </template>

            <template #card-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">{{ item.expand.type.name }}</span>
              <span v-else>-</span>
            </template>

            <template #cell-code="{ item }">
              <code v-if="item.code" class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.code }}</code>
              <span v-else class="text-base-content/40">-</span>
            </template>

            <template #actions="{ item }">
              <router-link :to="`/locations/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial" @click.stop>Edit</router-link>
              <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1 sm:flex-initial">Delete</button>
            </template>
          </ResponsiveList>

          <!-- Pagination -->
          <div v-if="totalPages > 1" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
            <span class="text-sm text-base-content/70 text-center sm:text-left">
              Showing {{ paginatedLocations.length }} of {{ filteredLocations.length }} locations
            </span>
            <div class="join">
              <button class="join-item btn btn-sm" :disabled="currentPage === 1" @click="currentPage--">«</button>
              <button class="join-item btn btn-sm">Page {{ currentPage }}</button>
              <button class="join-item btn btn-sm" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
            </div>
          </div>
        </template>
      </BaseCard>
    </div>

    <!-- MAP VIEW -->
    <div v-else-if="viewMode === 'map'" class="fade-in">
      <LocationMapViz />
    </div>

    <!-- OCCUPANCY VIEW -->
    <div v-else-if="viewMode === 'occupancy'" class="space-y-6 fade-in">

      <!-- NATS Offline -->
      <div v-if="!natsStore.isConnected" class="alert shadow-sm border border-base-300 bg-base-100">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
        <div class="text-xs">
          <div class="font-bold">Occupancy Offline</div>
          <span class="text-base-content/70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live occupancy data.</span>
        </div>
      </div>

      <template v-else>
        <!-- Search + Total Badge -->
        <div class="flex flex-col sm:flex-row gap-4 items-start sm:items-center">
          <div class="form-control flex-1 w-full">
            <input
              v-model="occupancySearchQuery"
              type="text"
              placeholder="Search by location name, code, or type..."
              class="input input-bordered w-full"
            />
          </div>
          <div v-if="totalOccupants > 0" class="badge badge-success badge-lg gap-2 font-mono shrink-0">
            {{ totalOccupants }} Total
          </div>
        </div>

        <!-- Loading -->
        <div v-if="loading && allLocations.length === 0" class="flex justify-center p-12">
          <span class="loading loading-spinner loading-lg"></span>
        </div>

        <!-- No Trackable Locations -->
        <div v-else-if="knownCodes.length === 0" class="text-center py-12">
          <span class="text-6xl">📍</span>
          <h3 class="text-xl font-bold mt-4">No trackable locations</h3>
          <p class="text-base-content/70 mt-2">
            Locations need a <code class="bg-base-200 px-1 rounded text-xs">code</code> field to support occupancy tracking.
          </p>
        </div>

        <!-- No Search Results -->
        <div v-else-if="filteredOccupancyLocations.length === 0" class="text-center py-12">
          <span class="text-6xl">🔍</span>
          <h3 class="text-xl font-bold mt-4">No matching locations</h3>
          <button @click="occupancySearchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
        </div>

        <!-- Occupancy Table -->
        <template v-else>
          <BaseCard :no-padding="true">
            <ResponsiveList
              :items="paginatedOccupancyLocations"
              :columns="occupancyColumns"
              :clickable="true"
              @row-click="handleOccupancyRowClick"
            >
              <template #cell-name="{ item }">
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-medium">{{ item.name }}</span>
                    <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">{{ item.expand.type.name }}</span>
                  </div>
                  <div v-if="item.expand?.parent" class="text-xs text-base-content/70">
                    {{ (item.expand.parent as Location).name }}
                  </div>
                </div>
              </template>

              <template #cell-code="{ item }">
                <code class="text-xs bg-base-200 px-1 py-0.5 rounded font-mono">{{ item.code }}</code>
              </template>

              <template #cell-occupancy="{ item }">
                <span v-if="countsLoading && !occupancyCounts.size" class="loading loading-spinner loading-xs"></span>
                <span v-else-if="getOccupancyCount(item.code) > 0" class="badge badge-success font-mono">
                  {{ getOccupancyCount(item.code) }}
                </span>
                <span v-else class="badge badge-ghost text-base-content/70">0</span>
              </template>

              <!-- Mobile Card -->
              <template #card-name="{ item }">
                <div class="flex justify-between items-center w-full">
                  <div>
                    <div class="font-semibold">{{ item.name }}</div>
                    <div v-if="item.expand?.parent" class="text-xs text-base-content/70">
                      {{ (item.expand.parent as Location).name }}
                    </div>
                    <code class="text-xs text-base-content/70 font-mono">{{ item.code }}</code>
                  </div>
                  <span v-if="getOccupancyCount(item.code) > 0" class="badge badge-success font-mono">
                    {{ getOccupancyCount(item.code) }}
                  </span>
                  <span v-else class="badge badge-ghost text-base-content/70">0</span>
                </div>
              </template>
            </ResponsiveList>

            <!-- Pagination -->
            <div v-if="occupancyTotalPages > 1" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
              <span class="text-sm text-base-content/70 text-center sm:text-left">
                {{ occupancyRangeStart }}–{{ occupancyRangeEnd }} of {{ filteredOccupancyLocations.length }} locations
              </span>
              <div class="join">
                <button class="join-item btn btn-sm" :disabled="occupancyPage === 1" @click="occupancyPage--">«</button>
                <button class="join-item btn btn-sm">Page {{ occupancyPage }}</button>
                <button class="join-item btn btn-sm" :disabled="occupancyPage === occupancyTotalPages" @click="occupancyPage++">»</button>
              </div>
            </div>
          </BaseCard>

          <!-- Selected Location Detail -->
          <div v-if="selectedOccupancyLocation" class="space-y-2">
            <div class="flex items-center justify-between">
              <h2 class="text-lg font-bold flex items-center gap-2">
                <span>📍</span>
                {{ selectedOccupancyLocation.name }}
              </h2>
              <button class="btn btn-ghost btn-xs" @click="selectedOccupancyCode = null">Collapse</button>
            </div>
            <OccupancyList :location-code="selectedOccupancyLocation.code" />
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<style scoped>
.fade-in {
  animation: fadeIn 0.2s ease-in-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
