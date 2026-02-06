<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatRelativeTime } from '@/utils/format'
import type { Invitation, Organization } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const authStore = useAuthStore()
const toast = useToast()
const { confirm } = useConfirm()

// Pagination for invitations list
const {
  items: invitations,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<Invitation>('invites', 20)

// All organizations (for operators to select from)
const allOrganizations = ref<Organization[]>([])
const loadingOrgs = ref(false)

// Form state
const showInviteForm = ref(false)
const inviteForm = ref({
  email: '',
  role: 'member' as 'admin' | 'member',
  organization: '' as string, // For operators to select target org
})
const submitting = ref(false)

/**
 * Load all organizations for operator org selector
 */
async function loadOrganizations() {
  if (!authStore.isOperator) return
  loadingOrgs.value = true
  try {
    allOrganizations.value = await pb.collection('organizations').getFullList<Organization>({
      sort: 'name',
    })
    // Default to current org if available
    if (authStore.currentOrgId && !inviteForm.value.organization) {
      inviteForm.value.organization = authStore.currentOrgId
    }
  } catch (err) {
    console.error('Failed to load organizations:', err)
  } finally {
    loadingOrgs.value = false
  }
}

// Search
const searchQuery = ref('')

// Token Visibility State
const visibleTokens = ref<Record<string, boolean>>({})

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
    key: 'token',
    label: 'Token',
    mobileLabel: 'Token',
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
 * Toggle token visibility
 */
function toggleToken(id: string) {
  visibleTokens.value[id] = !visibleTokens.value[id]
}

/**
 * Copy token to clipboard
 */
async function copyToken(token?: string) {
  if (!token) return
  try {
    await navigator.clipboard.writeText(token)
    toast.success('Token copied to clipboard')
  } catch (err) {
    toast.error('Failed to copy token')
  }
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

  // Determine target organization
  const targetOrgId = authStore.isOperator
    ? inviteForm.value.organization
    : authStore.currentOrgId

  if (!targetOrgId) {
    toast.error('Please select an organization')
    return
  }

  submitting.value = true

  try {
    // Simply create the invitation record
    // The pb-tenancy library handles sending the email automatically
    await pb.collection('invites').create({
      email: inviteForm.value.email,
      role: inviteForm.value.role,
      organization: targetOrgId,
      invited_by: authStore.user?.id,
    })

    toast.success('Invitation sent!')

    // Reset form and reload list
    inviteForm.value.email = ''
    inviteForm.value.role = 'member'
    if (authStore.isOperator && authStore.currentOrgId) {
      inviteForm.value.organization = authStore.currentOrgId
    }
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
  const confirmed = await confirm({
    title: 'Revoke Invitation',
    message: `Are you sure you want to revoke the invitation for ${invite.email}?`,
    details: 'The invitation link will no longer be valid.',
    confirmText: 'Revoke',
    variant: 'warning'
  })
  if (!confirmed) return

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
  loadOrganizations()
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
        <!-- Organization (Operators only) -->
        <div v-if="authStore.isOperator" class="form-control">
          <label class="label">
            <span class="label-text">Organization *</span>
          </label>
          <select
            v-model="inviteForm.organization"
            class="select select-bordered"
            :disabled="submitting || loadingOrgs"
            required
          >
            <option value="" disabled>Select organization...</option>
            <option
              v-for="org in allOrganizations"
              :key="org.id"
              :value="org.id"
            >
              {{ org.name }}
            </option>
          </select>
          <label class="label">
            <span class="label-text-alt">
              As an operator, you can invite users to any organization
            </span>
          </label>
        </div>

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

    <!-- Error State -->
    <BaseCard v-else-if="error && invitations.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load invitations</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadInvitations" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

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

        <!-- Custom cell for Token -->
        <template #cell-token="{ item }">
          <div class="flex items-center gap-2">
            <div class="font-mono text-xs bg-base-200 px-2 py-1 rounded min-w-[8rem] text-center select-all">
              {{ visibleTokens[item.id] ? item.token : '••••••••••••••••' }}
            </div>
            <div class="join">
              <button 
                @click="toggleToken(item.id)" 
                class="btn btn-ghost btn-xs join-item"
                :title="visibleTokens[item.id] ? 'Hide' : 'Show'"
              >
                {{ visibleTokens[item.id] ? '🙈' : '👁️' }}
              </button>
              <button 
                @click="copyToken(item.token)" 
                class="btn btn-ghost btn-xs join-item"
                title="Copy"
              >
                📋
              </button>
            </div>
          </div>
        </template>

        <!-- Custom mobile card for Token -->
        <template #card-token="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Token</span>
            <div class="mt-1 flex items-center gap-2">
              <div class="font-mono text-xs bg-base-200 px-2 py-1 rounded break-all select-all">
                {{ visibleTokens[item.id] ? item.token : '••••••••••••••••' }}
              </div>
              <button @click="toggleToken(item.id)" class="btn btn-ghost btn-xs">
                {{ visibleTokens[item.id] ? '🙈' : '👁️' }}
              </button>
              <button @click="copyToken(item.token)" class="btn btn-ghost btn-xs">
                📋
              </button>
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
