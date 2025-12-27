<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { Membership } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const authStore = useAuthStore()
const toast = useToast()

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

/**
 * Filtered members based on client-side search
 */
const filteredMembers = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return members.value
  
  return members.value.filter(member => {
    if (member.expand?.user?.name?.toLowerCase().includes(query)) return true
    if (member.expand?.user?.email?.toLowerCase().includes(query)) return true
    if (member.role?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration for responsive list
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

/**
 * Load members from API
 * Backend automatically filters by current organization
 */
async function loadMembers() {
  await load({ 
    expand: 'user,invited_by',
    sort: 'role',
  })
}

/**
 * Check if current user can remove a member
 */
function canRemoveMember(member: Membership): boolean {
  // Can't remove yourself
  if (member.user === authStore.user?.id) return false
  
  // Can't remove the owner
  if (member.role === 'owner') return false
  
  // Only owners and admins can remove members
  if (!authStore.canManageUsers) return false
  
  return true
}

/**
 * Handle removing a member
 */
async function handleRemoveMember(member: Membership) {
  const memberName = member.expand?.user?.name || member.expand?.user?.email || 'this member'
  
  if (!confirm(`Remove ${memberName} from ${authStore.currentOrg?.name}?`)) return
  
  try {
    await pb.collection('memberships').delete(member.id)
    toast.success('Member removed')
    await loadMembers()
  } catch (err: any) {
    toast.error(err.message || 'Failed to remove member')
  }
}

/**
 * Handle updating member role
 */
async function handleUpdateRole(member: Membership, newRole: 'admin' | 'member') {
  // Don't allow changing owner role
  if (member.role === 'owner') {
    toast.error('Cannot change owner role')
    return
  }
  
  // Don't allow changing your own role
  if (member.user === authStore.user?.id) {
    toast.error('Cannot change your own role')
    return
  }
  
  try {
    await pb.collection('memberships').update(member.id, {
      role: newRole,
    })
    toast.success('Role updated')
    await loadMembers()
  } catch (err: any) {
    toast.error(err.message || 'Failed to update role')
  }
}

/**
 * Get role badge class
 */
function getRoleBadgeClass(role: string): string {
  switch (role) {
    case 'owner': return 'badge-primary'
    case 'admin': return 'badge-secondary'
    case 'member': return 'badge-ghost'
    default: return 'badge-ghost'
  }
}

/**
 * Handle organization change
 */
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
    
    <!-- Info Alert -->
    <div class="alert alert-info">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      <span>Members are added when they accept an invitation. Use the "Send Invitation" button to invite new members.</span>
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
        <p class="text-base-content/70 mt-2">
          This shouldn't happen - at least you should be a member!
        </p>
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
        :clickable="false"
      >
        <!-- Custom cell for name (with avatar placeholder) -->
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
          <a 
            :href="`mailto:${item.expand?.user?.email}`"
            class="link link-hover text-sm"
          >
            {{ item.expand?.user?.email }}
          </a>
        </template>
        
        <!-- Custom cell for role (badge with dropdown for admins/owners) -->
        <template #cell-role="{ item }">
          <div class="flex items-center gap-2">
            <span 
              class="badge badge-sm"
              :class="getRoleBadgeClass(item.role)"
            >
              {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
            </span>
            
            <!-- Role change dropdown (only for admins/owners, not for self or owner) -->
            <div 
              v-if="authStore.canManageUsers && item.role !== 'owner' && item.user !== authStore.user?.id"
              class="dropdown dropdown-end"
            >
              <label tabindex="0" class="btn btn-ghost btn-xs">⋮</label>
              <ul tabindex="0" class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-52">
                <li><a @click="handleUpdateRole(item, 'admin')">Make Admin</a></li>
                <li><a @click="handleUpdateRole(item, 'member')">Make Member</a></li>
              </ul>
            </div>
          </div>
        </template>
        
        <!-- Custom card for role -->
        <template #card-role="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Role</span>
            <div class="mt-1 flex items-center gap-2">
              <span 
                class="badge badge-sm"
                :class="getRoleBadgeClass(item.role)"
              >
                {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
              </span>
              
              <!-- Role change dropdown for mobile -->
              <div 
                v-if="authStore.canManageUsers && item.role !== 'owner' && item.user !== authStore.user?.id"
                class="dropdown dropdown-end"
              >
                <label tabindex="0" class="btn btn-ghost btn-xs">⋮</label>
                <ul tabindex="0" class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-52">
                  <li><a @click="handleUpdateRole(item, 'admin')">Make Admin</a></li>
                  <li><a @click="handleUpdateRole(item, 'member')">Make Member</a></li>
                </ul>
              </div>
            </div>
          </div>
        </template>
        
        <!-- Actions (remove member) -->
        <template #actions="{ item }">
          <button 
            v-if="canRemoveMember(item)"
            @click="handleRemoveMember(item)" 
            class="btn btn-ghost btn-sm text-error flex-1 sm:flex-initial"
          >
            Remove
          </button>
          <span v-else class="text-sm text-base-content/40 flex-1 sm:flex-initial text-center">
            {{ item.user === authStore.user?.id ? 'You' : 'Owner' }}
          </span>
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
            @click="prevPage({ expand: 'user,invited_by', sort: 'role,-created' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'user,invited_by', sort: 'role,-created' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
