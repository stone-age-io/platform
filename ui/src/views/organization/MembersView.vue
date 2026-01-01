<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router' // Added
import { usePagination } from '@/composables/usePagination'
import { useAuthStore } from '@/stores/auth'
import type { Membership } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter() // Added
const authStore = useAuthStore()

// Pagination for members list
const {
  items: members,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<Membership>('memberships', 20)

// Search
const searchQuery = ref('')

const filteredMembers = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return members.value
  
  return members.value.filter(member => {
    if (member.expand?.user?.name?.toLowerCase().includes(query)) return true
    if (member.expand?.user?.email?.toLowerCase().includes(query)) return true
    if (member.expand?.organization?.name?.toLowerCase().includes(query)) return true
    if (member.role?.toLowerCase().includes(query)) return true
    return false
  })
})

const columns: Column<Membership>[] = [
  {
    key: 'expand.user.name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.user.email',
    label: 'Email',
    mobileLabel: 'Email',
  },
  {
    key: 'role',
    label: 'Role',
    mobileLabel: 'Role',
    format: (value) => value.charAt(0).toUpperCase() + value.slice(1),
  },
]

async function loadMembers() {
  if (!authStore.currentOrgId) return

  await load({ 
    filter: `organization = "${authStore.currentOrgId}"`, 
    expand: 'user,invited_by,organization', 
    sort: 'role',
  })
}

// Navigate to Detail View
function handleRowClick(member: Membership) {
  router.push(`/organization/members/${member.id}`)
}

function getRoleBadgeClass(role: string): string {
  switch (role) {
    case 'owner': return 'badge-primary'
    case 'admin': return 'badge-secondary'
    case 'member': return 'badge-ghost'
    default: return 'badge-ghost'
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  loadMembers()
}

onMounted(() => {
  loadMembers()
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
        <h1 class="text-3xl font-bold">Team Members</h1>
        <p class="text-base-content/70 mt-1">
          Manage members of {{ authStore.currentOrg?.name }}
        </p>
      </div>
      <router-link 
        to="/organization/invitations" 
        class="btn btn-primary w-full sm:w-auto"
      >
        <span class="text-lg">✉️</span>
        <span>Send Invitation</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search members by name, email, or role..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && members.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="members.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">👥</span>
        <h3 class="text-xl font-bold mt-4">No members found</h3>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredMembers.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching members</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredMembers" 
        :columns="columns" 
        :loading="loading"
        :clickable="true"
        @row-click="handleRowClick"
      >
        <!-- Custom cell for name -->
        <template #cell-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-10">
                <span class="text-xs">
                  {{ item.expand?.user?.name?.[0]?.toUpperCase() || '?' }}
                </span>
              </div>
            </div>
            <div>
              <div class="font-medium">
                {{ item.expand?.user?.name || 'Unknown User' }}
              </div>
              <div v-if="item.user === authStore.user?.id" class="text-xs text-base-content/60">
                (You)
              </div>
            </div>
          </div>
        </template>
        
        <!-- Custom mobile card for name -->
        <template #card-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-12">
                <span class="text-sm">
                  {{ item.expand?.user?.name?.[0]?.toUpperCase() || '?' }}
                </span>
              </div>
            </div>
            <div>
              <div class="font-semibold text-base">
                {{ item.expand?.user?.name || 'Unknown User' }}
              </div>
              <div v-if="item.user === authStore.user?.id" class="text-xs text-base-content/60">
                (You)
              </div>
            </div>
          </div>
        </template>
        
        <!-- Custom cell for email -->
        <template #cell-expand.user.email="{ item }">
          <span class="opacity-70 text-sm">{{ item.expand?.user?.email }}</span>
        </template>
        
        <!-- Custom cell for role -->
        <template #cell-role="{ item }">
          <span 
            class="badge badge-sm"
            :class="getRoleBadgeClass(item.role)"
          >
            {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
          </span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <button 
            @click.stop="handleRowClick(item)"
            class="btn btn-ghost btn-sm"
          >
            {{ authStore.canManageUsers ? 'Manage' : 'View' }}
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ members.length }} of {{ totalItems }} members
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'user,invited_by,organization', sort: 'role,-created' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'user,invited_by,organization', sort: 'role,-created' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
