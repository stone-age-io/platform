<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Location } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import LocationMapViz from '@/components/locations/LocationMapViz.vue'

const router = useRouter()
const toast = useToast()

// View Mode
const viewMode = ref<'list' | 'map'>('list')

// Data State
const allLocations = ref<Location[]>([])
const loading = ref(false)
const searchQuery = ref('')

// Pagination State (Client-Side)
const currentPage = ref(1)
const itemsPerPage = 20

/**
 * Filter Logic (Client-Side)
 * Searches Name, Code, Description, Type Name, and Parent Name
 */
const filteredLocations = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  
  if (!q) {
    // If no search, show Tree Root items (parents) only? 
    // Or show all? Grug says: Lists usually show everything or roots.
    // Let's stick to the previous behavior: Show Roots only if no search.
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
  if (!confirm(`Delete "${location.name}"? This cannot be undone.`)) return
  
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
      <div>
        <h1 class="text-3xl font-bold">Locations</h1>
        <p class="text-base-content/70 mt-1">
          Manage physical sites and facilities
        </p>
      </div>
      
      <div class="flex gap-3 w-full sm:w-auto">
        <!-- View Toggle -->
        <div class="join shadow-sm border border-base-300">
          <button 
            class="join-item btn btn-sm" 
            :class="{ 'btn-active': viewMode === 'list' }"
            @click="viewMode = 'list'"
          >
            📋 List
          </button>
          <button 
            class="join-item btn btn-sm" 
            :class="{ 'btn-active': viewMode === 'map' }"
            @click="viewMode = 'map'"
          >
            🗺️ Map
          </button>
        </div>

        <router-link to="/locations/new" class="btn btn-primary btn-sm h-full">
          <span class="text-lg">+</span>
          <span class="hidden sm:inline">New Location</span>
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
        <div v-if="loading" class="flex justify-center p-12">
          <span class="loading loading-spinner loading-lg"></span>
        </div>

        <div v-else-if="allLocations.length === 0" class="text-center py-12">
          <span class="text-6xl">📍</span>
          <h3 class="text-xl font-bold mt-4">No locations found</h3>
          <p class="text-base-content/70 mt-2">
            Create your first location to get started.
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
              <router-link :to="`/locations/${item.id}/edit`" class="btn btn-xs flex-1" @click.stop>Edit</router-link>
              <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1">Delete</button>
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
    <div v-else class="fade-in">
      <LocationMapViz />
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
