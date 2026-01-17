<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { NebulaHost } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: hosts,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NebulaHost>('nebula_hosts', 20)

// Search query
const searchQuery = ref('')

/**
 * Filtered hosts based on client-side search
 */
const filteredHosts = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return hosts.value
  
  return hosts.value.filter(host => {
    if (host.hostname?.toLowerCase().includes(query)) return true
    if (host.overlay_ip?.toLowerCase().includes(query)) return true
    if (host.expand?.network_id?.name?.toLowerCase().includes(query)) return true
    if (host.public_host_port?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NebulaHost>[] = [
  {
    key: 'hostname',
    label: 'Hostname',
    mobileLabel: 'Hostname',
  },
  {
    key: 'overlay_ip',
    label: 'Overlay IP',
    mobileLabel: 'IP',
  },
  {
    key: 'expand.network_id.name',
    label: 'Network',
    mobileLabel: 'Network',
  },
  {
    key: 'is_lighthouse',
    label: 'Type',
    mobileLabel: 'Type',
    format: (value) => value ? 'Lighthouse' : 'Node',
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
 * Load hosts from API
 */
async function loadHosts() {
  await load({ 
    expand: 'network_id'
  })
}

/**
 * Handle row click
 */
function handleRowClick(host: NebulaHost) {
  router.push(`/nebula/hosts/${host.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(host: NebulaHost) {
  if (!confirm(`Delete Nebula host "${host.hostname}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nebula_hosts').delete(host.id)
    toast.success('Nebula host deleted')
    loadHosts()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete Nebula host')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadHosts()
}

onMounted(() => {
  loadHosts()
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
        <h1 class="text-3xl font-bold">Nebula Hosts</h1>
        <p class="text-base-content/70 mt-1">
          Manage Nebula VPN hosts and lighthouses
        </p>
      </div>
      <router-link to="/nebula/hosts/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Host</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by hostname, IP, or network..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && hosts.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="hosts.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🖥️</span>
        <h3 class="text-xl font-bold mt-4">No Nebula hosts found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first Nebula host to get started
        </p>
        <router-link to="/nebula/hosts/new" class="btn btn-primary mt-4">
          Create Host
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredHosts.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching hosts</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredHosts" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for hostname -->
        <template #cell-hostname="{ item }">
          <div>
            <div class="font-medium font-mono">
              {{ item.hostname }}
            </div>
            <div v-if="item.public_host_port" class="text-sm text-base-content/60">
              {{ item.public_host_port }}
            </div>
          </div>
        </template>
        
        <!-- Custom mobile card for hostname -->
        <template #card-hostname="{ item }">
          <div>
            <div class="font-semibold font-mono text-base">
              {{ item.hostname }}
            </div>
            <div v-if="item.public_host_port" class="text-sm text-base-content/60 mt-1">
              {{ item.public_host_port }}
            </div>
          </div>
        </template>
        
        <!-- Custom cell for IP -->
        <template #cell-overlay_ip="{ item }">
          <code class="text-sm">{{ item.overlay_ip }}</code>
        </template>
        
        <!-- Custom cell for type (badge) -->
        <template #cell-is_lighthouse="{ item }">
          <span 
            class="badge badge-sm"
            :class="item.is_lighthouse ? 'badge-primary' : 'badge-ghost'"
          >
            {{ item.is_lighthouse ? 'Lighthouse' : 'Node' }}
          </span>
        </template>
        
        <template #card-is_lighthouse="{ item }">
  	  <span 
    	    class="badge badge-sm"
    	    :class="item.is_lighthouse ? 'badge-primary' : 'badge-ghost'"
  	  >
    	    {{ item.is_lighthouse ? 'Lighthouse' : 'Node' }}
  	  </span>
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
            :to="`/nebula/hosts/${item.id}/edit`" 
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
          Showing {{ hosts.length }} of {{ totalItems }} hosts
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'network_id' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'network_id' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
