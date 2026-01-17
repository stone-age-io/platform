<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Location } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: locations,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<Location>('locations', 20)

// Search query
const searchQuery = ref('')

/**
 * Filter Logic
 * If Searching: Global flat search (find any location)
 * Default: Only show Root locations (parent = empty)
 */
async function loadLocations() {
  let filter = ''
  
  if (searchQuery.value.trim()) {
    // SEARCH MODE: Flatten everything
    const q = searchQuery.value.toLowerCase().trim()
    filter = `name ~ "${q}" || description ~ "${q}" || code ~ "${q}"`
  } else {
    // DEFAULT: Root level only
    filter = 'parent = ""'
  }

  await load({ 
    filter,
    expand: 'type',
    sort: 'name' 
  })
}

// Column configuration
const columns: Column<Location>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.type.name',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'code',
    label: 'Code',
    mobileLabel: 'Code',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

function handleRowClick(location: Location) {
  router.push(`/locations/${location.id}`)
}

async function handleDelete(location: Location) {
  if (!confirm(`Delete "${location.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('locations').delete(location.id)
    toast.success('Location deleted')
    loadLocations()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete location')
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  loadLocations()
}

// Watch search to reload automatically
let searchTimeout: any
watch(searchQuery, () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    page.value = 1
    loadLocations()
  }, 300)
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
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Locations</h1>
        <p class="text-base-content/70 mt-1">
          Manage physical sites and facilities
        </p>
      </div>
      <router-link to="/locations/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Location</span>
      </router-link>
    </div>
    
    <!-- Controls -->
    <div class="flex flex-col gap-4">
      <div class="form-control">
        <input 
          v-model="searchQuery"
          type="text"
          placeholder="Search all locations..."
          class="input input-bordered w-full"
        />
        <label v-if="!searchQuery" class="label">
          <span class="label-text-alt text-base-content/60">
            Showing top-level locations. Use search to find sub-locations.
          </span>
        </label>
        <label v-else class="label">
          <span class="label-text-alt text-base-content/60">
            Searching all locations (flat view).
          </span>
        </label>
      </div>
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && locations.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="locations.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📍</span>
        <h3 class="text-xl font-bold mt-4">
          {{ searchQuery ? 'No matching locations' : 'No top-level locations' }}
        </h3>
        <p class="text-base-content/70 mt-2">
          {{ searchQuery ? 'Try a different search term' : 'Create your first root location to get started' }}
        </p>
        <div class="mt-4 flex gap-2 justify-center">
          <button v-if="searchQuery" @click="searchQuery = ''" class="btn btn-ghost">
            Clear Search
          </button>
          <router-link v-else to="/locations/new" class="btn btn-primary">
            Create Location
          </router-link>
        </div>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="locations" 
        :columns="columns" 
        :loading="loading"
        :clickable="true"
        @row-click="handleRowClick"
      >
        <!-- Name -->
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">
              {{ item.name }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Mobile Name -->
        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">
              {{ item.name }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Type -->
        <template #cell-expand.type.name="{ item }">
          <span v-if="item.expand?.type" class="badge badge-ghost">
            {{ item.expand.type.name }}
          </span>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Code -->
        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <router-link 
            :to="`/locations/${item.id}/edit`" 
            class="btn btn-xs flex-1"
            @click.stop
          >
            Edit
          </router-link>
          <button 
            @click.stop="handleDelete(item)" 
            class="btn btn-xs text-error flex-1"
          >
            Delete
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ locations.length }} of {{ totalItems }} locations
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage()"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage()"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
