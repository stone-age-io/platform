<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { Membership, NatsUser } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const membership = ref<Membership | null>(null)
const natsUsers = ref<NatsUser[]>([])
const loading = ref(true)
const saving = ref(false)

const memberId = route.params.id as string

const isSelf = computed(() => membership.value?.user === authStore.user?.id)
const isOwner = computed(() => membership.value?.role === 'owner')
const canManage = computed(() => authStore.canManageUsers && !isSelf.value && !isOwner.value)

async function loadData() {
  loading.value = true
  try {
    // 1. Get Membership details (added invited_by expansion)
    membership.value = await pb.collection('memberships').getOne(memberId, {
      expand: 'user,organization,nats_user,invited_by'
    })

    // 2. Get available NATS users for this org
    natsUsers.value = await pb.collection('nats_users').getFullList<NatsUser>({
      sort: 'nats_username',
      filter: 'active = true'
    })
  } catch (err: any) {
    toast.error('Failed to load member details')
    router.push('/organization/members')
  } finally {
    loading.value = false
  }
}

async function updateRole(newRole: 'admin' | 'member') {
  if (!membership.value) return
  
  saving.value = true
  try {
    const updated = await pb.collection('memberships').update(membership.value.id, {
      role: newRole
    })
    membership.value.role = updated.role
    toast.success(`Role updated to ${newRole}`)
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    saving.value = false
  }
}

async function updateNatsIdentity(event: Event) {
  const select = event.target as HTMLSelectElement
  const natsUserId = select.value || null
  
  if (!membership.value) return
  
  saving.value = true
  try {
    await pb.collection('memberships').update(membership.value.id, {
      nats_user: natsUserId
    })
    toast.success('NATS Identity assigned')
    await loadData() 
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    saving.value = false
  }
}

async function removeMember() {
  if (!membership.value) return
  const userName = membership.value.expand?.user?.name || membership.value.expand?.user?.email
  
  if (!confirm(`Are you sure you want to remove ${userName} from the organization?`)) return
  
  try {
    await pb.collection('memberships').delete(membership.value.id)
    toast.success('Member removed')
    router.push('/organization/members')
  } catch (err: any) {
    toast.error(err.message)
  }
}

onMounted(loadData)
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="membership">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/organization/members">Members</router-link></li>
            <li>{{ membership.expand?.user?.name || 'Member Details' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-10">
                <span class="text-xl">{{ membership.expand?.user?.name?.[0]?.toUpperCase() || 'U' }}</span>
              </div>
            </div>
            <div>
              <h1 class="text-3xl font-bold break-words">{{ membership.expand?.user?.name || 'Unknown User' }}</h1>
            </div>
          </div>
          
          <div class="flex gap-2 w-full sm:w-auto" v-if="canManage">
            <button @click="removeMember" class="btn btn-error flex-1 sm:flex-initial">
              Remove Member
            </button>
          </div>
        </div>
      </div>

      <!-- Content Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: User Profile & Permissions -->
        <div class="space-y-6">
          <BaseCard title="Profile Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Email</dt>
                <dd class="mt-1 text-sm font-mono select-all bg-base-200 inline-block px-2 py-1 rounded">
                  {{ membership.expand?.user?.email }}
                </dd>
              </div>
              
              <div v-if="membership.expand?.invited_by">
                <dt class="text-sm font-medium text-base-content/70 mb-2">Invited By</dt>
                <div class="flex items-center gap-2 p-2 bg-base-200/50 rounded-lg border border-base-200">
                  <div class="avatar placeholder">
                    <div class="bg-neutral-focus text-neutral-content rounded-full w-6">
                      <span class="text-xs">{{ membership.expand.invited_by.name?.[0] || '?' }}</span>
                    </div>
                  </div>
                  <div class="text-sm">
                    <span class="font-bold">{{ membership.expand.invited_by.name || 'Unknown' }}</span>
                    <span class="text-xs opacity-60 ml-2">({{ membership.expand.invited_by.email }})</span>
                  </div>
                </div>
              </div>
              <div v-else>
                <dt class="text-sm font-medium text-base-content/70">Invited By</dt>
                <dd class="text-sm opacity-50 italic">System / Direct Add</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Role & Permissions">
            <div class="space-y-4">
              <div v-if="isOwner" class="alert alert-info py-2 text-sm">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                <span>Organization Owner (Immutable)</span>
              </div>
              
              <div v-else-if="isSelf" class="alert alert-warning py-2 text-sm">
                <span>You cannot change your own role.</span>
              </div>

              <div v-else>
                <dt class="text-sm font-medium text-base-content/70 mb-2">Assigned Role</dt>
                <div class="join w-full">
                  <button 
                    class="btn join-item flex-1" 
                    :class="membership.role === 'admin' ? 'btn-primary' : 'btn-active btn-ghost'"
                    @click="updateRole('admin')"
                    :disabled="saving"
                  >
                    Admin
                  </button>
                  <button 
                    class="btn join-item flex-1" 
                    :class="membership.role === 'member' ? 'btn-primary' : 'btn-active btn-ghost'"
                    @click="updateRole('member')"
                    :disabled="saving"
                  >
                    Member
                  </button>
                </div>
                <p class="text-xs text-base-content/60 mt-2">
                  Admins can invite new users and manage existing members.
                </p>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Infrastructure Identity -->
        <div class="space-y-6">
          <BaseCard title="Infrastructure Identity">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text font-medium text-base-content/70">Assigned NATS User</span>
                </label>
                <select 
                  class="select select-bordered font-mono text-sm w-full"
                  :value="membership.nats_user || ''"
                  @change="updateNatsIdentity"
                  :disabled="saving || isSelf" 
                >
                  <option value="">-- None Assigned --</option>
                  <option v-for="u in natsUsers" :key="u.id" :value="u.id">
                    {{ u.nats_username }}
                  </option>
                </select>
                <label class="label">
                  <span class="label-text-alt">
                    Links this user to a specific NATS Credential for this organization context.
                    {{ isSelf ? 'Go to Settings to change your own identity.' : '' }}
                  </span>
                </label>
              </div>

              <div v-if="membership.expand?.nats_user" class="bg-base-200 rounded-lg p-3 border border-base-300">
                <div class="flex justify-between items-start mb-1">
                  <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Username</span>
                  <div class="flex items-center gap-1.5">
                    <span class="w-2 h-2 rounded-full" :class="membership.expand.nats_user.active ? 'bg-success' : 'bg-error'"></span>
                    <span class="text-xs font-medium text-base-content/70">{{ membership.expand.nats_user.active ? 'Active' : 'Inactive' }}</span>
                  </div>
                </div>
                <div class="font-mono text-sm break-all select-all">{{ membership.expand.nats_user.nats_username }}</div>
                
                <div class="divider my-2"></div>
                
                <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider block mb-1">Public Key</span>
                <div class="font-mono text-xs text-base-content/70 break-all">
                  {{ membership.expand.nats_user.public_key }}
                </div>
              </div>

              <div v-else class="text-center py-6 border-2 border-dashed border-base-300 rounded-lg bg-base-100">
                <span class="text-2xl block mb-2 opacity-50">🚫</span>
                <span class="text-sm opacity-60">No Identity Linked</span>
              </div>
            </div>
          </BaseCard>
        </div>

      </div>
    </template>
  </div>
</template>
