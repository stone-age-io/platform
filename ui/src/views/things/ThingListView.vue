<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: things,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<Thing>('things', 20)

// Search query (client-side filtering)
const searchQuery = ref('')

/**
 * Filtered things based on client-side search
 * Searches in: name, description, code, type name, location name
 */
const filteredThings = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  // No search query - return all things
  if (!query) return things.value
  
  // Filter client-side
  return things.value.filter(thing => {
    // Search in name
    if (thing.name?.toLowerCase().includes(query)) return true
    
    // Search in description
    if (thing.description?.toLowerCase().includes(query)) return true
    
    // Search in code
    if (thing.code?.toLowerCase().includes(query)) return true
    
    // Search in type name
    if (thing.expand?.type?.name?.toLowerCase().includes(query)) return true
    
    // Search in location name
    if (thing.expand?.location?.name?.toLowerCase().includes(query)) return true
    
    return false
  })
})

// Column configuration for responsive list
const columns: Column<Thing>[] = [
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
    key: 'expand.location.name',
    label: 'Location',
    mobileLabel: 'Location',
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

/**
 * Load things from API
 * Backend automatically filters by current organization via API rules
 */
async function loadThings() {
  // Only pass expand - backend handles org filtering via API rules
  await load({ 
    expand: 'type,location' 
  })
}

/**
 * Handle row/card click - navigate to detail view
 */
function handleRowClick(thing: Thing) {
  router.push(`/things/${thing.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(thing: Thing) {
  if (!confirm(`Delete "${thing.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('things').delete(thing.id)
    toast.success('Thing deleted')
    loadThings()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete thing')
  }
}

/**
 * Handle organization change event
 */
function handleOrgChange() {
  // Reset search when org changes
  searchQuery.value = ''
  loadThings()
}

/**
 * Initialize on mount
 */
onMounted(() => {
  loadThings()
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
        <h1 class="text-3xl font-bold">Things</h1>
        <p class="text-base-content/70 mt-1">
          Manage IoT devices and sensors
        </p>
      </div>
      <router-link to="/things/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Thing</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search things by name, code, type, or location..."
        class="input input-bordered w-full"
      />
      <label v-if="searchQuery && filteredThings.length < things.length" class="label">
        <span class="label-text-alt">
          Showing {{ filteredThings.length }} of {{ things.length }} things
          (searching current page only)
        </span>
      </label>
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && things.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="things.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📦</span>
        <h3 class="text-xl font-bold mt-4">No things found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first thing to get started
        </p>
        <router-link to="/things/new" class="btn btn-primary mt-4">
          Create Thing
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredThings.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching things</h3>
        <p class="text-base-content/70 mt-2">
          Try a different search term
        </p>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredThings" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for name (with description) - no longer a link -->
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">
              {{ item.name || 'Unnamed' }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Custom mobile card for name (make it prominent) - no longer a link -->
        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">
              {{ item.name || 'Unnamed' }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Custom cell for type (badge) -->
        <template #cell-expand.type.name="{ item }">
          <span v-if="item.expand?.type" class="badge badge-ghost">
            {{ item.expand.type.name }}
          </span>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Custom card for type (badge) -->
        <template #card-expand.type.name="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Type</span>
            <div class="mt-1">
              <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">
                {{ item.expand.type.name }}
              </span>
              <span v-else class="text-sm text-base-content/40">-</span>
            </div>
          </div>
        </template>
        
        <!-- Custom cell for code (mono font) -->
        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Actions - @click.stop is handled in ResponsiveList -->
        <template #actions="{ item }">
          <router-link 
            :to="`/things/${item.id}/edit`" 
            class="btn btn-ghost btn-sm flex-1 sm:flex-initial"
          >
            Edit
          </router-link>
          <button 
            @click="handleDelete(item)" 
            class="btn btn-ghost btn-sm text-error flex-1 sm:flex-initial"
          >
            Delete
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination (only show when not searching) -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ things.length }} of {{ totalItems }} things
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'type,location' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'type,location' })"
          >
            »
          </button>
        </div>
      </div>
      
      <!-- Search active message (when paginated) -->
      <div v-else class="p-4 border-t border-base-300 text-center">
        <p class="text-sm text-base-content/60">
          Searching within current page. 
          <button @click="searchQuery = ''" class="link link-primary text-sm">
            Clear search
          </button>
          to browse all pages.
        </p>
      </div>
    </BaseCard>
  </div>
</template>
