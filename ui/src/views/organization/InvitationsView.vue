<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate, formatRelativeTime } from '@/utils/format'
import type { Invitation } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const authStore = useAuthStore()
const toast = useToast()

// Pagination for invitations list
const {
  items: invitations,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<Invitation>('invites', 20)

// Form state
const showInviteForm = ref(false)
const inviteForm = ref({
  email: '',
  role: 'member' as 'admin' | 'member',
})
const submitting = ref(false)

// Search
const searchQuery = ref('')

/**
 * Filtered invitations based on client-side search
 */
const filteredInvitations = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return invitations.value
  
  return invitations.value.filter(invite => {
    if (invite.email?.toLowerCase().includes(query)) return true
    if (invite.role?.toLowerCase().includes(query)) return true
    if (invite.expand?.invited_by?.name?.toLowerCase().includes(query)) return true
    return false
  })
})

// Column configuration for responsive list
const columns: Column<Invitation>[] = [
  {
    key: 'email',
    label: 'Email',
    mobileLabel: 'Email',
  },
  {
    key: 'role',
    label: 'Role',
    mobileLabel: 'Role',
    format: (value) => value.charAt(0).toUpperCase() + value.slice(1),
  },
  {
    key: 'expand.invited_by.name',
    label: 'Invited By',
    mobileLabel: 'Invited By',
  },
  {
    key: 'expires_at',
    label: 'Expires',
    mobileLabel: 'Expires',
    format: (value) => value ? formatRelativeTime(value) : 'Never',
  },
]

/**
 * Check if invitation is expired
 */
function isExpired(invite: Invitation): boolean {
  if (!invite.expires_at) return false
  return new Date(invite.expires_at) < new Date()
}

/**
 * Load invitations from API
 * Backend automatically filters by current organization
 */
async function loadInvitations() {
  await load({ 
    expand: 'invited_by',
  })
}

/**
 * Handle sending new invitation
 */
async function handleSendInvite() {
  if (!inviteForm.value.email.trim()) {
    toast.error('Email is required')
    return
  }
  
  submitting.value = true
  
  try {
    // Simply create the invitation record
    // The pb-tenancy library handles sending the email automatically
    await pb.collection('invites').create({
      email: inviteForm.value.email,
      role: inviteForm.value.role,
      organization: authStore.currentOrgId,
      invited_by: authStore.user?.id,
    })
    
    toast.success('Invitation sent!')
    
    // Reset form and reload list
    inviteForm.value.email = ''
    inviteForm.value.role = 'member'
    showInviteForm.value = false
    
    await loadInvitations()
  } catch (err: any) {
    toast.error(err.message || 'Failed to send invitation')
  } finally {
    submitting.value = false
  }
}

/**
 * Handle resending invitation
 */
async function handleResend(invite: Invitation) {
  try {
    // Set resend_invite flag to true
    await pb.collection('invites').update(invite.id, {
      resend_invite: true,
    })
    
    toast.success('Invitation resent!')
    await loadInvitations()
  } catch (err: any) {
    toast.error(err.message || 'Failed to resend invitation')
  }
}

/**
 * Handle deleting invitation
 */
async function handleDelete(invite: Invitation) {
  if (!confirm(`Revoke invitation for ${invite.email}?`)) return
  
  try {
    await pb.collection('invites').delete(invite.id)
    toast.success('Invitation revoked')
    await loadInvitations()
  } catch (err: any) {
    toast.error(err.message || 'Failed to revoke invitation')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  searchQuery.value = ''
  showInviteForm.value = false
  loadInvitations()
}

onMounted(() => {
  loadInvitations()
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
        <h1 class="text-3xl font-bold">Invitations</h1>
        <p class="text-base-content/70 mt-1">
          Invite new members to {{ authStore.currentOrg?.name }}
        </p>
      </div>
      <button 
        @click="showInviteForm = !showInviteForm"
        class="btn btn-primary w-full sm:w-auto"
      >
        <span class="text-lg">✉️</span>
        <span>{{ showInviteForm ? 'Cancel' : 'Send Invitation' }}</span>
      </button>
    </div>
    
    <!-- Invite Form -->
    <BaseCard v-if="showInviteForm" title="Send New Invitation">
      <form @submit.prevent="handleSendInvite" class="space-y-4">
        <!-- Email -->
        <div class="form-control">
          <label class="label">
            <span class="label-text">Email Address *</span>
          </label>
          <input 
            v-model="inviteForm.email"
            type="email" 
            placeholder="colleague@example.com"
            class="input input-bordered"
            required
            :disabled="submitting"
          />
        </div>
        
        <!-- Role -->
        <div class="form-control">
          <label class="label">
            <span class="label-text">Role *</span>
          </label>
          <select 
            v-model="inviteForm.role" 
            class="select select-bordered"
            :disabled="submitting"
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </select>
          <label class="label">
            <span class="label-text-alt">
              Admins can invite and manage other members
            </span>
          </label>
        </div>
        
        <!-- Submit -->
        <div class="flex flex-col-reverse sm:flex-row gap-2">
          <button 
            type="button"
            @click="showInviteForm = false"
            class="btn btn-ghost"
            :disabled="submitting"
          >
            Cancel
          </button>
          <button 
            type="submit"
            class="btn btn-primary"
            :disabled="submitting"
          >
            <span v-if="submitting" class="loading loading-spinner"></span>
            <span v-else>Send Invitation</span>
          </button>
        </div>
      </form>
    </BaseCard>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search invitations by email or role..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && invitations.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="invitations.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">✉️</span>
        <h3 class="text-xl font-bold mt-4">No pending invitations</h3>
        <p class="text-base-content/70 mt-2">
          Send an invitation to add new members to your organization
        </p>
        <button 
          @click="showInviteForm = true"
          class="btn btn-primary mt-4"
        >
          Send Invitation
        </button>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredInvitations.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching invitations</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredInvitations" 
        :columns="columns" 
        :loading="loading"
        :clickable="false"
      >
        <!-- Custom cell for email (with expired badge) -->
        <template #cell-email="{ item }">
          <div>
            <div class="font-medium">
              {{ item.email }}
            </div>
            <div v-if="isExpired(item)" class="mt-1">
              <span class="badge badge-error badge-sm">Expired</span>
            </div>
          </div>
        </template>
        
        <!-- Custom mobile card for email -->
        <template #card-email="{ item }">
          <div>
            <div class="font-semibold text-base">
              {{ item.email }}
            </div>
            <div v-if="isExpired(item)" class="mt-1">
              <span class="badge badge-error badge-sm">Expired</span>
            </div>
          </div>
        </template>
        
        <!-- Custom cell for role (badge) -->
        <template #cell-role="{ item }">
          <span 
            class="badge badge-sm"
            :class="item.role === 'admin' ? 'badge-primary' : 'badge-ghost'"
          >
            {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
          </span>
        </template>
        
        <!-- Custom card for role -->
        <template #card-role="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Role</span>
            <div class="mt-1">
              <span 
                class="badge badge-sm"
                :class="item.role === 'admin' ? 'badge-primary' : 'badge-ghost'"
              >
                {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
              </span>
            </div>
          </div>
        </template>
        
        <!-- Custom cell for expires_at (with expired warning) -->
        <template #cell-expires_at="{ item }">
          <span 
            :class="{ 'text-error': isExpired(item) }"
            class="text-sm"
          >
            {{ item.expires_at ? formatRelativeTime(item.expires_at) : 'Never' }}
          </span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <button 
            @click="handleResend(item)"
            class="btn btn-ghost btn-sm flex-1 sm:flex-initial"
            :disabled="!isExpired(item)"
            :title="isExpired(item) ? 'Resend invitation' : 'Cannot resend active invitation'"
          >
            Resend
          </button>
          <button 
            @click="handleDelete(item)" 
            class="btn btn-ghost btn-sm text-error flex-1 sm:flex-initial"
          >
            Revoke
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ invitations.length }} of {{ totalItems }} invitations
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'invited_by', sort: '-created' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'invited_by', sort: '-created' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
