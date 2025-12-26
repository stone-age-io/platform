<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { formatDate } from '@/utils/format'
import type { NatsAccount } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

// Pagination
const {
  items: accounts,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsAccount>('nats_accounts', 20)

// Search query (client-side filtering)
const searchQuery = ref('')

/**
 * Filtered accounts based on client-side search
 */
const filteredAccounts = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return accounts.value
  
  return accounts.value.filter(account => {
    if (account.name?.toLowerCase().includes(query)) return true
    if (account.description?.toLowerCase().includes(query)) return true
    if (account.public_key?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NatsAccount>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'active',
    label: 'Status',
    mobileLabel: 'Status',
    format: (value) => value ? 'Active' : 'Inactive',
  },
  {
    key: 'max_connections',
    label: 'Max Connections',
    mobileLabel: 'Max Connections',
    format: (value) => value === -1 ? 'Unlimited' : value?.toString() || '0',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

/**
 * Load accounts from API
 */
async function loadAccounts() {
  await load()
}

/**
 * Handle organization change event
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadAccounts()
}

onMounted(() => {
  loadAccounts()
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
        <h1 class="text-3xl font-bold">NATS Accounts</h1>
        <p class="text-base-content/70 mt-1">
          Automatically provisioned NATS accounts for your organization
        </p>
      </div>
    </div>
    
    <!-- Info Alert -->
    <div class="alert alert-info">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      <span>NATS Accounts are automatically created when your organization is provisioned. Create NATS Users to access these accounts.</span>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search accounts by name or description..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && accounts.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="accounts.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📡</span>
        <h3 class="text-xl font-bold mt-4">No NATS accounts found</h3>
        <p class="text-base-content/70 mt-2">
          NATS accounts are automatically created for your organization
        </p>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredAccounts.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching accounts</h3>
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
        :items="filteredAccounts" 
        :columns="columns" 
        :loading="loading"
        :clickable="false"
      >
        <!-- Custom cell for name (with description) -->
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
        
        <!-- Custom cell for status (badge) -->
        <template #cell-active="{ item }">
          <span 
            class="badge badge-sm"
            :class="item.active ? 'badge-success' : 'badge-error'"
          >
            {{ item.active ? 'Active' : 'Inactive' }}
          </span>
        </template>
        
        <!-- Custom card for status -->
        <template #card-active="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Status</span>
            <div class="mt-1">
              <span 
                class="badge badge-sm"
                :class="item.active ? 'badge-success' : 'badge-error'"
              >
                {{ item.active ? 'Active' : 'Inactive' }}
              </span>
            </div>
          </div>
        </template>
        
        <!-- Actions (View Details) -->
        <template #actions="{ item }">
          <button class="btn btn-ghost btn-sm flex-1 sm:flex-initial">
            View Details
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ accounts.length }} of {{ totalItems }} accounts
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
