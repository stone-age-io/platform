<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { formatDate } from '@/utils/format'
import type { NebulaCA } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()

// Pagination
const {
  items: cas,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NebulaCA>('nebula_ca', 20)

// Search query
const searchQuery = ref('')

/**
 * Filtered CAs based on client-side search
 */
const filteredCAs = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return cas.value
  
  return cas.value.filter(ca => {
    if (ca.name?.toLowerCase().includes(query)) return true
    if (ca.curve?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NebulaCA>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'curve',
    label: 'Curve',
    mobileLabel: 'Curve',
  },
  {
    key: 'validity_years',
    label: 'Validity',
    mobileLabel: 'Validity',
    format: (value) => value ? `${value} years` : '-',
  },
  {
    key: 'expires_at',
    label: 'Expires',
    mobileLabel: 'Expires',
    format: (value) => value ? formatDate(value, 'PP') : 'Never',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

/**
 * Load CAs from API
 */
async function loadCAs() {
  await load()
}

/**
 * Handle row click
 */
function handleRowClick(ca: NebulaCA) {
  router.push(`/nebula/cas/${ca.id}`)
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadCAs()
}

onMounted(() => {
  loadCAs()
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
        <h1 class="text-3xl font-bold">Nebula Certificate Authorities</h1>
        <p class="text-base-content/70 mt-1">
          Automatically provisioned Nebula CAs for your organization
        </p>
      </div>
    </div>
    
    <!-- Info Alert -->
    <div class="alert alert-info">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      <span>Nebula CAs are automatically created when your organization is provisioned. Create Networks and Hosts to use the CA.</span>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search CAs by name..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && cas.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="cas.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔐</span>
        <h3 class="text-xl font-bold mt-4">No Nebula CAs found</h3>
        <p class="text-base-content/70 mt-2">
          Nebula CAs are automatically created for your organization
        </p>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredCAs.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching CAs</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredCAs" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for name -->
        <template #cell-name="{ item }">
          <div class="font-medium">
            {{ item.name }}
          </div>
        </template>
        
        <!-- Custom mobile card for name -->
        <template #card-name="{ item }">
          <div class="font-semibold text-base">
            {{ item.name }}
          </div>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <router-link 
            :to="`/nebula/cas/${item.id}`" 
            class="btn btn-ghost btn-sm flex-1 sm:flex-initial"
          >
            View Details
          </router-link>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ cas.length }} of {{ totalItems }} CAs
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
