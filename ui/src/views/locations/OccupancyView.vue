<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useNatsStore } from '@/stores/nats'
import { useOccupancySummary } from '@/composables/useOccupancySummary'
import type { Location } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import OccupancyList from '@/components/locations/OccupancyList.vue'

const toast = useToast()
const natsStore = useNatsStore()

// Data
const locations = ref<Location[]>([])
const loading = ref(false)
const searchQuery = ref('')
const selectedCode = ref<string | null>(null)

// Extract codes for the summary composable
const knownCodes = computed(() => locations.value.map(l => l.code!).filter(Boolean))
const { counts: occupancyCounts, loading: countsLoading } = useOccupancySummary(knownCodes)

// Columns
const columns: Column<Location>[] = [
  { key: 'name', label: 'Location', mobileLabel: 'Location' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'occupancy', label: 'Occupants', mobileLabel: 'Occupants' },
]

// Total occupants across all locations
const totalOccupants = computed(() => {
  let total = 0
  for (const count of occupancyCounts.value.values()) {
    total += count
  }
  return total
})

// Filtering
const filteredLocations = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  let result = locations.value

  if (q) {
    result = result.filter(l => {
      const nameMatch = l.name?.toLowerCase().includes(q)
      const codeMatch = l.code?.toLowerCase().includes(q)
      const typeMatch = l.expand?.type?.name?.toLowerCase().includes(q)
      return nameMatch || codeMatch || typeMatch
    })
  }

  // Sort: locations with occupants first, then alphabetically
  return [...result].sort((a, b) => {
    const countA = occupancyCounts.value.get(a.code!) || 0
    const countB = occupancyCounts.value.get(b.code!) || 0
    if (countA !== countB) return countB - countA
    return (a.name || '').localeCompare(b.name || '')
  })
})

// Pagination
const currentPage = ref(1)
const itemsPerPage = 10
const totalPages = computed(() => Math.ceil(filteredLocations.value.length / itemsPerPage))
const rangeStart = computed(() => (currentPage.value - 1) * itemsPerPage + 1)
const rangeEnd = computed(() => Math.min(currentPage.value * itemsPerPage, filteredLocations.value.length))

const paginatedLocations = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return filteredLocations.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })
watch(totalPages, (pages) => {
  if (currentPage.value > pages && pages > 0) currentPage.value = pages
})

// Helpers
function getCount(code?: string): number {
  if (!code) return 0
  return occupancyCounts.value.get(code) || 0
}

function handleRowClick(location: Location) {
  // Toggle: click same location to collapse, click different to switch
  selectedCode.value = selectedCode.value === location.code ? null : (location.code || null)
}

// Selected location for the detail panel
const selectedLocation = computed(() => {
  if (!selectedCode.value) return null
  return locations.value.find(l => l.code === selectedCode.value) || null
})

// Data loading
async function loadLocations() {
  loading.value = true
  try {
    locations.value = await pb.collection('locations').getFullList<Location>({
      filter: 'code != ""',
      expand: 'type',
      sort: 'name',
    })
  } catch (err: any) {
    console.error('Failed to load locations', err)
    toast.error('Failed to load locations')
  } finally {
    loading.value = false
  }
}

function handleOrgChange() {
  selectedCode.value = null
  loadLocations()
}

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
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Occupancy</h1>
        <p class="text-base-content/70 mt-1">
          Live presence tracking across locations
        </p>
      </div>
      <div v-if="natsStore.isConnected && totalOccupants > 0" class="badge badge-success badge-lg gap-2 font-mono">
        {{ totalOccupants }} Total
      </div>
    </div>

    <!-- NATS Offline -->
    <div v-if="!natsStore.isConnected" class="alert shadow-sm border border-base-300 bg-base-100">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      <div class="text-xs">
        <div class="font-bold">Occupancy Offline</div>
        <span class="text-base-content/70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live occupancy data.</span>
      </div>
    </div>

    <template v-else>
      <!-- Search -->
      <div class="form-control">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by location name, code, or type..."
          class="input input-bordered w-full"
        />
      </div>

      <!-- Loading -->
      <div v-if="loading && locations.length === 0" class="flex justify-center p-12">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <!-- Empty: No Locations with Codes -->
      <div v-else-if="locations.length === 0" class="text-center py-12">
        <span class="text-6xl">📍</span>
        <h3 class="text-xl font-bold mt-4">No trackable locations</h3>
        <p class="text-base-content/70 mt-2">
          Locations need a <code class="bg-base-200 px-1 rounded text-xs">code</code> field to support occupancy tracking.
        </p>
        <router-link to="/locations" class="btn btn-primary mt-4">Manage Locations</router-link>
      </div>

      <!-- No Search Results -->
      <div v-else-if="filteredLocations.length === 0" class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching locations</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
      </div>

      <!-- Locations Table -->
      <template v-else>
        <BaseCard :no-padding="true">
          <ResponsiveList
            :items="paginatedLocations"
            :columns="columns"
            :clickable="true"
            @row-click="handleRowClick"
          >
            <template #cell-name="{ item }">
              <div class="flex items-center gap-2">
                <span class="font-medium">{{ item.name }}</span>
                <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">{{ item.expand.type.name }}</span>
              </div>
            </template>

            <template #cell-code="{ item }">
              <code class="text-xs bg-base-200 px-1 py-0.5 rounded font-mono">{{ item.code }}</code>
            </template>

            <template #cell-occupancy="{ item }">
              <span v-if="countsLoading && !occupancyCounts.size" class="loading loading-spinner loading-xs"></span>
              <span v-else-if="getCount(item.code) > 0" class="badge badge-success font-mono">
                {{ getCount(item.code) }}
              </span>
              <span v-else class="badge badge-ghost text-base-content/70">0</span>
            </template>

            <!-- Mobile Card -->
            <template #card-name="{ item }">
              <div class="flex justify-between items-center w-full">
                <div>
                  <div class="font-semibold">{{ item.name }}</div>
                  <code class="text-xs text-base-content/70 font-mono">{{ item.code }}</code>
                </div>
                <span v-if="getCount(item.code) > 0" class="badge badge-success font-mono">
                  {{ getCount(item.code) }}
                </span>
                <span v-else class="badge badge-ghost text-base-content/70">0</span>
              </div>
            </template>
          </ResponsiveList>

          <!-- Pagination -->
          <div v-if="totalPages > 1" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
            <span class="text-sm text-base-content/70 text-center sm:text-left">
              {{ rangeStart }}–{{ rangeEnd }} of {{ filteredLocations.length }} locations
            </span>
            <div class="join">
              <button class="join-item btn btn-sm" :disabled="currentPage === 1" @click="currentPage--">«</button>
              <button class="join-item btn btn-sm">Page {{ currentPage }}</button>
              <button class="join-item btn btn-sm" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
            </div>
          </div>
        </BaseCard>

        <!-- Selected Location Detail -->
        <div v-if="selectedLocation" class="space-y-2">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-bold flex items-center gap-2">
              <span>📍</span>
              {{ selectedLocation.name }}
            </h2>
            <button class="btn btn-ghost btn-xs" @click="selectedCode = null">Collapse</button>
          </div>
          <OccupancyList :location-code="selectedLocation.code" />
        </div>
      </template>
    </template>
  </div>
</template>
