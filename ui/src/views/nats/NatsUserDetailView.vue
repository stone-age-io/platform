<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import type { NatsUser } from '@/types/pocketbase'
import DangerZone from '@/components/common/DangerZone.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import SubjectChip from '@/components/common/SubjectChip.vue'
import { findLinkedThing, type LinkedThing } from '@/utils/linkedThing'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const user = ref<NatsUser | null>(null)
const loading = ref(true)
const deleting = ref(false)
const regenerating = ref(false)
const revoking = ref(false)
const reenabling = ref(false)

function getSubjectArray(val: any): string[] {
  if (!val) return []
  if (Array.isArray(val)) return val

  if (typeof val === 'string') {
    const trimmed = val.trim()
    if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
      try {
        return JSON.parse(trimmed)
      } catch (e) {
        return []
      }
    }
    return trimmed.split(',').map(s => s.trim()).filter(s => s !== '')
  }
  return []
}

const hasPermissionOverrides = computed(() => {
  if (!user.value) return false
  return getSubjectArray(user.value.publish_permissions).length > 0
    || getSubjectArray(user.value.subscribe_permissions).length > 0
    || getSubjectArray(user.value.publish_deny_permissions).length > 0
    || getSubjectArray(user.value.subscribe_deny_permissions).length > 0
})

const userId = route.params.id as string

/**
 * Load user details
 */
async function loadUser() {
  loading.value = true
  try {
    user.value = await pb.collection('nats_users').getOne<NatsUser>(userId, {
      expand: 'account_id,role_id',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load NATS user')
    router.push('/nats/users')
    return
  } finally {
    loading.value = false
  }
  linkedThing.value = await findLinkedThing('nats_user', userId)
}

// The Thing this identity belongs to, if it is a device's. While that Thing is
// deactivated the server refuses Re-enable (hooks/active_flag.go), so the page
// points at the Thing instead of offering a button that fails.
const linkedThing = ref<LinkedThing | null>(null)
const heldByDeactivatedThing = computed(() => linkedThing.value?.active === false)

/**
 * Handle delete
 */
async function handleDelete() {
  if (!user.value) return
  const confirmed = await confirm({
    title: 'Delete NATS User',
    message: `Are you sure you want to delete "${user.value.nats_username}"?`,
    details: 'This will invalidate any credentials issued to this user.',
    confirmText: 'Delete',
    variant: 'danger',
    requireText: user.value.nats_username || undefined,
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('nats_users').delete(user.value.id)
    toast.success('NATS user deleted')
    router.push('/nats/users')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete NATS user')
  } finally {
    deleting.value = false
  }
}

/**
 * Trigger Regeneration
 */
async function confirmRegenerate() {
  if (!user.value) return

  const confirmed = await confirm({
    title: 'Regenerate Credentials',
    message: `Issue a fresh .creds file for "${user.value.nats_username}"?`,
    details: 'This does not invalidate the current file: every copy keeps working, because the key does not change. If the credentials leaked, use Revoke instead.',
    confirmText: 'Regenerate',
    variant: 'warning',
  })
  if (!confirmed) return

  regenerating.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { regenerate: true })
    toast.success('Credentials regenerated')
    await loadUser()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

/**
 * Revoke the user's currently distributed credentials and issue replacements.
 *
 * This is the "these credentials leaked" button, not a suspend. pb-nats
 * (rotateUserCredentials) generates a NEW key pair, adds the OLD public key to
 * the account's revocation list (embedded in the account JWT), and mints a
 * fresh .creds for the new key. Every copy of the old file is rejected at once
 * and for good; the user stays active, and the new file has to be delivered to
 * the legitimate holder. Suspending is `active` false, which revokes without
 * reissuing — for a device that is the Thing's own active flag.
 */
async function confirmRevoke() {
  if (!user.value) return

  const confirmed = await confirm({
    title: 'Revoke Credentials',
    message: `Reject every copy of the current .creds file for "${user.value.nats_username}"?`,
    details: 'Its key goes onto the account revocation list, so NATS rejects every copy of the existing file, permanently. A new key and a new .creds file are issued in the same step and the user stays active: deliver the new file to whoever should hold it. This does not take the identity out of service.',
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return

  revoking.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { revoke: true })
    toast.success('Credentials revoked and replaced')
    await loadUser()
  } catch (err: any) {
    toast.error(err.message || 'Failed to revoke credentials')
  } finally {
    revoking.value = false
  }
}

/**
 * Re-enable a revoked/inactive user by marking them active and issuing a fresh
 * JWT. The new credentials carry a later issue time so NATS accepts them, while
 * any previously revoked credentials stay permanently invalid.
 */
async function confirmReenable() {
  if (!user.value) return

  const confirmed = await confirm({
    title: 'Re-enable User',
    message: `Mark "${user.value.nats_username}" active and issue a fresh .creds file?`,
    details: 'Deliver the new file to wherever this identity connects from. Credentials revoked before this stay permanently invalid.',
    confirmText: 'Re-enable',
    variant: 'info',
  })
  if (!confirmed) return

  reenabling.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { active: true, regenerate: true })
    toast.success('User re-enabled with fresh credentials')
    await loadUser()
  } catch (err: any) {
    toast.error(err.message || 'Failed to re-enable user')
  } finally {
    reenabling.value = false
  }
}

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch (err) {
    toast.error('Failed to copy')
  }
}

function downloadCredsFile() {
  if (!user.value?.creds_file) {
    toast.error('No credentials file available')
    return
  }
  
  const blob = new Blob([user.value.creds_file], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${user.value.nats_username}.creds`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  
  toast.success('Credentials downloaded')
}

onMounted(() => {
  loadUser()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="user">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/users">NATS Users</router-link></li>
            <li class="truncate max-w-[200px] font-mono">{{ user.nats_username }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold font-mono break-words">{{ user.nats_username }}</h1>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/users/${user.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
          </div>
        </div>
      </div>
      
      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Identity -->
        <div class="space-y-6">
          <BaseCard title="Identity & Permissions">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ user.description || '-' }}</dd>
              </div>
              
              <div>
                <dt class="text-sm font-medium text-base-content/70">Email (Identity)</dt>
                <dd class="mt-1 text-sm">{{ user.email }}</dd>
              </div>
              
              <div>
                <dt class="text-sm font-medium text-base-content/70">Account</dt>
                <dd class="mt-1">
                  <router-link 
                    v-if="user.expand?.account_id"
                    :to="`/nats/account`"
                    class="link link-primary hover:no-underline"
                  >
                    📡 {{ user.expand.account_id.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">—</span>
                </dd>
              </div>
              
              <div>
                <dt class="text-sm font-medium text-base-content/70">Role</dt>
                <dd class="mt-1">
                  <router-link 
                    v-if="user.expand?.role_id"
                    :to="`/nats/roles/${user.role_id}`"
                    class="link link-primary hover:no-underline"
                  >
                    🎭 {{ user.expand.role_id.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">—</span>
                </dd>
              </div>

              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(user.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <!-- Permission Overrides -->
          <BaseCard v-if="hasPermissionOverrides" title="Permission Overrides">
            <p class="text-xs text-base-content/60 mb-4">User-level overrides merged with role permissions.</p>

            <!-- Publishing -->
            <div v-if="getSubjectArray(user.publish_permissions).length || getSubjectArray(user.publish_deny_permissions).length" class="mb-4">
              <h4 class="text-xs font-black uppercase opacity-50 tracking-widest mb-2 flex items-center gap-2">
                <span>📤</span> Publishing
              </h4>
              <div class="space-y-2">
                <div v-if="getSubjectArray(user.publish_permissions).length">
                  <span class="text-xs text-base-content/50">Allow:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <SubjectChip v-for="s in getSubjectArray(user.publish_permissions)" :key="s" :subject="s" />
                  </div>
                </div>
                <div v-if="getSubjectArray(user.publish_deny_permissions).length">
                  <span class="text-xs text-base-content/50">Deny:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <SubjectChip v-for="s in getSubjectArray(user.publish_deny_permissions)" :key="s" :subject="s" deny />
                  </div>
                </div>
              </div>
            </div>

            <!-- Subscribing -->
            <div v-if="getSubjectArray(user.subscribe_permissions).length || getSubjectArray(user.subscribe_deny_permissions).length">
              <h4 class="text-xs font-black uppercase opacity-50 tracking-widest mb-2 flex items-center gap-2">
                <span>📥</span> Subscribing
              </h4>
              <div class="space-y-2">
                <div v-if="getSubjectArray(user.subscribe_permissions).length">
                  <span class="text-xs text-base-content/50">Allow:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <SubjectChip v-for="s in getSubjectArray(user.subscribe_permissions)" :key="s" :subject="s" />
                  </div>
                </div>
                <div v-if="getSubjectArray(user.subscribe_deny_permissions).length">
                  <span class="text-xs text-base-content/50">Deny:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <SubjectChip v-for="s in getSubjectArray(user.subscribe_deny_permissions)" :key="s" :subject="s" deny />
                  </div>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Security -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex flex-wrap justify-between items-center gap-2 mb-4">
                <h2 class="card-title">Security & Credentials</h2>
                <div class="flex flex-wrap gap-2">
                  <!-- Active user: download / rotate / revoke -->
                  <template v-if="user.active">
                    <button
                      v-if="user.creds_file"
                      @click="downloadCredsFile"
                      class="btn btn-sm btn-outline"
                    >
                      <span class="text-lg">📥</span>
                      .creds
                    </button>
                    <button
                      @click="confirmRegenerate"
                      class="btn btn-sm btn-outline"
                      title="Issue a fresh .creds file"
                    >
                      <span class="text-lg">🔄</span>
                      Regenerate
                    </button>
                    <button
                      @click="confirmRevoke"
                      class="btn btn-sm btn-outline btn-error"
                      title="Reject the current credentials and issue replacements"
                    >
                      <span class="text-lg">⛔</span>
                      Revoke
                    </button>
                  </template>
                  <!-- Revoked / inactive user: re-enable with fresh credentials.
                       Not for a deactivated Thing's identity: the server
                       refuses it, and the notice below names the Thing. -->
                  <button
                    v-else-if="!heldByDeactivatedThing"
                    @click="confirmReenable"
                    class="btn btn-sm btn-outline btn-success"
                    title="Re-enable this user and issue new credentials"
                  >
                    <span class="text-lg">✅</span>
                    Re-enable
                  </button>
                </div>
              </div>
            </template>

            <div class="space-y-6">
              <!-- Status Indicators -->
              <div class="grid grid-cols-2 gap-4">
                <div class="bg-base-200 rounded-lg p-2 border border-base-300">
                  <span class="text-xs text-base-content/50 uppercase block mb-1">Status</span>
                  <div class="flex items-center gap-1.5" v-if="user.active">
                    <span class="w-2 h-2 rounded-full bg-success"></span>
                    <span class="font-medium text-sm">Active</span>
                  </div>
                  <div class="flex items-center gap-1.5" v-else>
                    <span class="w-2 h-2 rounded-full bg-error"></span>
                    <span class="font-medium text-sm">Inactive</span>
                  </div>
                </div>
                
                <div class="bg-base-200 rounded-lg p-2 border border-base-300">
                  <span class="text-xs text-base-content/50 uppercase block mb-1">Bearer Token</span>
                  <div class="flex items-center gap-1.5">
                    <span class="font-medium text-sm">{{ user.bearer_token ? 'Enabled' : 'Disabled' }}</span>
                  </div>
                </div>
              </div>

              <!-- Revoked / inactive notice -->
              <div v-if="!user.active" class="alert alert-warning py-2 text-sm">
                <span class="text-lg">⛔</span>
                <span v-if="heldByDeactivatedThing && linkedThing">
                  This identity belongs to the deactivated Thing
                  <router-link :to="`/things/${linkedThing.id}`" class="link font-semibold">{{ linkedThing.code || linkedThing.name }}</router-link>,
                  so NATS rejects its credentials. Reactivate the Thing to bring it back:
                  that issues a fresh <code>.creds</code> file and restores the device's
                  sign-in and Nebula host with it.
                </span>
                <span v-else>
                  This user is inactive — any credentials it currently holds are rejected by NATS.
                  Use <span class="font-semibold">Re-enable</span> to issue a fresh <code>.creds</code> file;
                  previously revoked credentials stay permanently invalid.
                </span>
              </div>

              <!-- JWT Expiry -->
              <div v-if="user.jwt_expires_at">
                <dt class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-1">JWT Expires</dt>
                <dd class="text-sm">{{ formatDate(user.jwt_expires_at) }}</dd>
              </div>

              <!-- Public Key -->
              <div v-if="user.public_key">
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Public Key</div>
                  <button @click="copyToClipboard(user.public_key!, 'Public Key')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all border border-base-300">
                  {{ user.public_key }}
                </div>
              </div>

              <!-- JWT -->
              <div v-if="user.jwt">
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">User JWT</div>
                  <button @click="copyToClipboard(user.jwt!, 'JWT')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all max-h-32 overflow-y-auto border border-base-300">
                  {{ user.jwt }}
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>

      <DangerZone>
        <button @click="handleDelete" class="btn btn-error" :disabled="deleting">
          Delete NATS User
        </button>
      </DangerZone>
    </template>
  </div>
</template>
