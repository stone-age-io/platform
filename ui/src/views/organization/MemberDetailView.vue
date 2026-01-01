<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { Membership, NatsUser, User, Organization } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

/**
 * Local interface that explicitly defines the fields we are expanding 
 * in this specific view.
 */
interface FullMemberMembership extends Membership {
  expand: {
    user: User
    organization: Organization
    nats_user?: NatsUser
    invited_by?: User
  }
}

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

// --- State ---
// Use the locally defined interface here
const membership = ref<FullMemberMembership | null>(null)
const natsUsers = ref<NatsUser[]>([])
const loading = ref(true)

const roleSaving = ref(false)
const natsSaving = ref(false)

const memberId = route.params.id as string

// --- Computeds ---
const isSelf = computed(() => membership.value?.user === authStore.user?.id)
const isOwner = computed(() => membership.value?.role === 'owner')
const canManage = computed(() => authStore.canManageUsers && !isSelf.value && !isOwner.value)

// --- Actions ---

async function loadData() {
  loading.value = true
  try {
    // Cast the result to our local comprehensive interface
    membership.value = await pb.collection('memberships').getOne(memberId, {
      expand: 'user,organization,nats_user,invited_by'
    }) as FullMemberMembership

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
  if (!membership.value || membership.value.role === newRole) return
  
  roleSaving.value = true
  try {
    const updated = await pb.collection('memberships').update(membership.value.id, {
      role: newRole
    }, {
      expand: 'user,organization,nats_user,invited_by'
    }) as FullMemberMembership
    
    Object.assign(membership.value, updated)
    toast.success(`Role updated to ${newRole}`)
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    roleSaving.value = false
  }
}

async function updateNatsIdentity(event: Event) {
  const select = event.target as HTMLSelectElement
  const natsUserId = select.value || null
  
  if (!membership.value) return
  
  natsSaving.value = true
  try {
    const updated = await pb.collection('memberships').update(membership.value.id, {
      nats_user: natsUserId
    }, {
      expand: 'user,organization,nats_user,invited_by'
    }) as FullMemberMembership
    
    Object.assign(membership.value, updated)
    toast.success('NATS Identity updated')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    natsSaving.value = false
  }
}

async function removeMember() {
  if (!membership.value) return
  const userName = membership.value.expand.user.name || membership.value.expand.user.email
  
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
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="membership">
      <!-- Page Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/organization/members">Members</router-link></li>
            <li>{{ membership.expand.user.name || 'Member Details' }}</li>
          </ul>
        </div>
        
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-12 border-2 border-base-300">
                <span class="text-xl">{{ membership.expand.user.name?.[0]?.toUpperCase() || 'U' }}</span>
              </div>
            </div>
            <div>
              <h1 class="text-3xl font-bold break-words">{{ membership.expand.user.name || 'Unknown User' }}</h1>
              <div class="flex items-center gap-2 mt-1">
                 <span class="badge badge-sm badge-outline opacity-70">{{ membership.role.toUpperCase() }}</span>
                 <span v-if="isSelf" class="badge badge-sm badge-info">YOU</span>
              </div>
            </div>
          </div>
          
          <div class="flex gap-2 w-full sm:w-auto" v-if="canManage">
            <button @click="removeMember" class="btn btn-error flex-1 sm:flex-initial">
              Remove Member
            </button>
          </div>
        </div>
      </div>

      <!-- Main Dashboard Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- LEFT COLUMN -->
        <div class="space-y-6">
          <BaseCard title="Profile Information">
            <div class="space-y-5">
              <div>
                <dt class="text-xs font-bold text-base-content/50 uppercase tracking-widest mb-1">Email Address</dt>
                <dd class="font-mono text-sm select-all bg-base-200 inline-block px-3 py-1.5 rounded-lg border border-base-300">
                  {{ membership.expand.user.email }}
                </dd>
              </div>
              
              <div>
                <dt class="text-xs font-bold text-base-content/50 uppercase tracking-widest mb-2">Onboarding</dt>
                <div v-if="membership.expand.invited_by" class="flex items-center gap-3 p-3 bg-base-200/50 rounded-xl border border-base-300">
                  <div class="avatar placeholder">
                    <div class="bg-primary text-primary-content rounded-full w-8">
                      <span class="text-xs">{{ membership.expand.invited_by.name?.[0] || '?' }}</span>
                    </div>
                  </div>
                  <div class="flex flex-col">
                    <span class="text-xs opacity-60">Invited By</span>
                    <span class="text-sm font-bold">{{ membership.expand.invited_by.name || membership.expand.invited_by.email }}</span>
                  </div>
                </div>
                <div v-else class="text-sm opacity-50 italic px-1">
                  System provisioned or Direct entry
                </div>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Role & Permissions">
            <div class="space-y-4">
              <div v-if="isOwner" class="alert bg-primary/10 border-primary/20 py-3 text-sm">
                <span class="text-xl">👑</span>
                <span>This user is the <strong>Organization Owner</strong>. Roles are immutable for owners.</span>
              </div>
              
              <div v-else-if="isSelf" class="alert bg-info/10 border-info/20 py-3 text-sm">
                <span class="text-xl">👤</span>
                <span>You are managing your own membership. Use the <strong>Settings</strong> page for personal changes.</span>
              </div>

              <div v-else>
                <label class="label pt-0"><span class="label-text font-bold text-base-content/50 uppercase text-xs">Assign Organization Role</span></label>
                <div class="join w-full bg-base-200 p-1 rounded-xl">
                  <button 
                    class="btn join-item flex-1 border-none shadow-none" 
                    :class="membership.role === 'admin' ? 'btn-primary' : 'btn-ghost'"
                    @click="updateRole('admin')"
                    :disabled="roleSaving" 
                  >
                    <span v-if="roleSaving && membership.role === 'member'" class="loading loading-spinner loading-xs"></span>
                    Administrator
                  </button>
                  <button 
                    class="btn join-item flex-1 border-none shadow-none" 
                    :class="membership.role === 'member' ? 'btn-primary' : 'btn-ghost'"
                    @click="updateRole('member')"
                    :disabled="roleSaving"
                  >
                    <span v-if="roleSaving && membership.role === 'admin'" class="loading loading-spinner loading-xs"></span>
                    Standard Member
                  </button>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- RIGHT COLUMN -->
        <div class="space-y-6">
          <BaseCard title="Infrastructure Identity">
            <div class="space-y-5">
              <div class="form-control">
                <label class="label pt-0">
                  <span class="label-text font-bold text-base-content/50 uppercase text-xs">Linked NATS Identity</span>
                </label>
                <div class="relative">
                  <select 
                    class="select select-bordered font-mono text-sm w-full h-12"
                    :value="membership.nats_user || ''"
                    @change="updateNatsIdentity"
                    :disabled="natsSaving || isSelf" 
                  >
                    <option value="">-- No Identity Linked --</option>
                    <option v-for="u in natsUsers" :key="u.id" :value="u.id">
                      {{ u.nats_username }}
                    </option>
                  </select>
                  <div v-if="natsSaving" class="absolute right-10 top-3.5">
                     <span class="loading loading-spinner loading-xs text-primary"></span>
                  </div>
                </div>
              </div>

              <div v-if="membership.expand.nats_user" class="bg-base-300/50 rounded-xl p-4 border border-base-300">
                <div class="flex justify-between items-start mb-4">
                  <span class="text-[10px] font-black text-base-content/40 uppercase tracking-widest">Active Credentials</span>
                  <div class="badge badge-success badge-xs gap-1.5 font-bold">ACTIVE</div>
                </div>
                
                <div class="grid grid-cols-1 gap-4">
                  <div>
                    <span class="block text-[10px] font-bold text-base-content/60 uppercase mb-1">NATS Username</span>
                    <span class="font-mono text-sm text-primary select-all">{{ membership.expand.nats_user.nats_username }}</span>
                  </div>
                  <div>
                    <span class="block text-[10px] font-bold text-base-content/60 uppercase mb-1">Public Key (NKey)</span>
                    <div class="font-mono text-[10px] break-all bg-base-100 p-2 rounded border border-base-300 text-base-content/70">
                      {{ membership.expand.nats_user.public_key }}
                    </div>
                  </div>
                </div>
              </div>

              <div v-else class="flex flex-col items-center justify-center py-10 border-2 border-dashed border-base-300 rounded-xl bg-base-200/30">
                <span class="text-4xl mb-2 opacity-20">📡</span>
                <span class="text-xs font-bold opacity-40 uppercase tracking-widest">Offline Mode</span>
                <p class="text-[10px] opacity-40 mt-1">No operational identity assigned</p>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
