<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { NebulaNetwork } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: networks,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NebulaNetwork>('nebula_networks', 20)

// Search query
const searchQuery = ref('')

/**
 * Filtered networks based on client-side search
 */
const filteredNetworks = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return networks.value
  
  return networks.value.filter(network => {
    if (network.name?.toLowerCase().includes(query)) return true
    if (network.description?.toLowerCase().includes(query)) return true
    if (network.cidr_range?.toLowerCase().includes(query)) return true
    if (network.expand?.ca_id?.name?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NebulaNetwork>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'cidr_range',
    label: 'CIDR Range',
    mobileLabel: 'CIDR',
  },
  {
    key: 'expand.ca_id.name',
    label: 'Certificate Authority',
    mobileLabel: 'CA',
  },
  {
    key: 'active',
    label: 'Status',
    mobileLabel: 'Status',
    format: (value) => value ? 'Active' : 'Inactive',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

/**
 * Load networks from API
 */
async function loadNetworks() {
  await load({ 
    expand: 'ca_id'
  })
}

/**
 * Handle row click
 */
function handleRowClick(network: NebulaNetwork) {
  router.push(`/nebula/networks/${network.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(network: NebulaNetwork) {
  if (!confirm(`Delete network "${network.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nebula_networks').delete(network.id)
    toast.success('Network deleted')
    loadNetworks()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete network')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadNetworks()
}

onMounted(() => {
  loadNetworks()
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
        <h1 class="text-3xl font-bold">Nebula Networks</h1>
        <p class="text-base-content/70 mt-1">
          Manage Nebula overlay networks
        </p>
      </div>
      <router-link to="/nebula/networks/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Network</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, CIDR, or CA..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && networks.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="networks.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🌐</span>
        <h3 class="text-xl font-bold mt-4">No networks found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first Nebula network to get started
        </p>
        <router-link to="/nebula/networks/new" class="btn btn-primary mt-4">
          Create Network
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredNetworks.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching networks</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredNetworks" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for name -->
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
        
        <!-- Custom mobile card for name -->
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
        
        <!-- Custom cell for CIDR (mono) -->
        <template #cell-cidr_range="{ item }">
          <code class="text-sm">{{ item.cidr_range }}</code>
        </template>
        
        <!-- Custom cell for status (badge) -->
        <template #cell-active="{ item }">
          <span 
            class="badge badge-sm"
            :class="item.active ? 'badge-success' : 'badge-error'"
          >
            {{ item.active ? 'Active' : 'Inactive' }}
          </span>
        </template>
        
        <template #card-active="{ item }">
  	  <span 
    	    class="badge badge-sm"
    	    :class="item.active ? 'badge-success' : 'badge-error'"
  	  >
    	    {{ item.active ? 'Active' : 'Inactive' }}
  	  </span>
	</template> 
        
	<!-- Actions -->
        <template #actions="{ item }">
          <router-link 
            :to="`/nebula/networks/${item.id}/edit`" 
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            Edit
          </router-link>
          <button 
            @click="handleDelete(item)" 
            class="btn btn-xs text-error flex-1 sm:flex-initial"
          >
            Delete
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ networks.length }} of {{ totalItems }} networks
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'ca_id' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'ca_id' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
