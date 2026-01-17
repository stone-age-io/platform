<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { NatsRole } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: roles,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsRole>('nats_roles', 20)

// Search query
const searchQuery = ref('')

/**
 * Filtered roles based on client-side search
 */
const filteredRoles = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return roles.value
  
  return roles.value.filter(role => {
    if (role.name?.toLowerCase().includes(query)) return true
    if (role.description?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NatsRole>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'is_default',
    label: 'Default',
    mobileLabel: 'Default',
    format: (value) => value ? 'Yes' : 'No',
  },
  {
    key: 'max_subscriptions',
    label: 'Max Subscriptions',
    mobileLabel: 'Max Subs',
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
 * Load roles from API
 */
async function loadRoles() {
  await load()
}

/**
 * Handle row click
 */
function handleRowClick(role: NatsRole) {
  router.push(`/nats/roles/${role.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(role: NatsRole) {
  if (!confirm(`Delete role "${role.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nats_roles').delete(role.id)
    toast.success('Role deleted')
    loadRoles()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete role')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadRoles()
}

onMounted(() => {
  loadRoles()
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
        <h1 class="text-3xl font-bold">NATS Roles</h1>
        <p class="text-base-content/70 mt-1">
          Define permissions for NATS users
        </p>
      </div>
      <router-link to="/nats/roles/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Role</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search roles by name or description..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && roles.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="roles.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🎭</span>
        <h3 class="text-xl font-bold mt-4">No roles found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first NATS role to get started
        </p>
        <router-link to="/nats/roles/new" class="btn btn-primary mt-4">
          Create Role
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredRoles.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching roles</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredRoles" 
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
        
        <!-- Custom cell for default (badge) -->
        <template #cell-is_default="{ item }">
          <span v-if="item.is_default" class="badge badge-primary badge-sm">
            Default
          </span>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <router-link 
            :to="`/nats/roles/${item.id}/edit`" 
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
          Showing {{ roles.length }} of {{ totalItems }} roles
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
