<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { NatsUser } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

// Pagination
const {
  items: users,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsUser>('nats_users', 20)

// Search query
const searchQuery = ref('')

/**
 * Filtered users based on client-side search
 */
const filteredUsers = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return users.value
  
  return users.value.filter(user => {
    if (user.nats_username?.toLowerCase().includes(query)) return true
    if (user.description?.toLowerCase().includes(query)) return true
    if (user.expand?.account_id?.name?.toLowerCase().includes(query)) return true
    if (user.expand?.role_id?.name?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration
const columns: Column<NatsUser>[] = [
  {
    key: 'nats_username',
    label: 'Username',
    mobileLabel: 'Username',
  },
  {
    key: 'expand.account_id.name',
    label: 'Account',
    mobileLabel: 'Account',
  },
  {
    key: 'expand.role_id.name',
    label: 'Role',
    mobileLabel: 'Role',
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
 * Load users from API
 */
async function loadUsers() {
  await load({ 
    expand: 'account_id,role_id'
  })
}

/**
 * Handle row click
 */
function handleRowClick(user: NatsUser) {
  router.push(`/nats/users/${user.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(user: NatsUser) {
  const confirmed = await confirm({
    title: 'Delete NATS User',
    message: `Are you sure you want to delete "${user.nats_username}"?`,
    details: 'This will invalidate any credentials issued to this user.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    await pb.collection('nats_users').delete(user.id)
    toast.success('NATS user deleted')
    loadUsers()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete NATS user')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadUsers()
}

onMounted(() => {
  loadUsers()
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
        <h1 class="text-3xl font-bold">NATS Users</h1>
        <p class="text-base-content/70 mt-1">
          Manage NATS user credentials and access
        </p>
      </div>
      <router-link to="/nats/users/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New NATS User</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by username, account, or role..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && users.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="users.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">👤</span>
        <h3 class="text-xl font-bold mt-4">No NATS users found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first NATS user to get started
        </p>
        <router-link to="/nats/users/new" class="btn btn-primary mt-4">
          Create NATS User
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredUsers.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching users</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredUsers" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for username -->
        <template #cell-nats_username="{ item }">
          <div>
            <div class="font-medium font-mono">
              {{ item.nats_username }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Custom mobile card for username -->
        <template #card-nats_username="{ item }">
          <div>
            <div class="font-semibold font-mono text-base">
              {{ item.nats_username }}
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
            :to="`/nats/users/${item.id}/edit`" 
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
          Showing {{ users.length }} of {{ totalItems }} users
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'account_id,role_id' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'account_id,role_id' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
