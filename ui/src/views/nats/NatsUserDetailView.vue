<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import type { NatsUser } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const user = ref<NatsUser | null>(null)
const loading = ref(true)
const deleting = ref(false)
const regenerating = ref(false)
const showRegenerateModal = ref(false)
const revoking = ref(false)
const showRevokeModal = ref(false)
const reenabling = ref(false)
const showReenableModal = ref(false)

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
  } finally {
    loading.value = false
  }
}

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
    variant: 'danger'
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

  regenerating.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { regenerate: true })
    toast.success('Credentials regenerated')
    showRegenerateModal.value = false
    await loadUser()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

/**
 * Revoke the user's currently distributed credentials.
 *
 * pb-nats adds the user's public key to the owning account's revocation list
 * (embedded in the account JWT) and marks the user inactive. NATS then rejects
 * the user's existing .creds immediately. The cutoff is permanent — only a
 * freshly issued JWT (see re-enable) is accepted afterward.
 */
async function confirmRevoke() {
  if (!user.value) return

  revoking.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { revoke: true })
    toast.success('Credentials revoked')
    showRevokeModal.value = false
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

  reenabling.value = true
  try {
    await pb.collection('nats_users').update(user.value.id, { active: true, regenerate: true })
    toast.success('User re-enabled with fresh credentials')
    showReenableModal.value = false
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
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial" :disabled="deleting">
              Delete
            </button>
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
                  <span v-else class="text-sm text-base-content/40">-</span>
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
                  <span v-else class="text-sm text-base-content/40">-</span>
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
                    <code v-for="s in getSubjectArray(user.publish_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1 px-2">{{ s }}</code>
                  </div>
                </div>
                <div v-if="getSubjectArray(user.publish_deny_permissions).length">
                  <span class="text-xs text-base-content/50">Deny:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <code v-for="s in getSubjectArray(user.publish_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1 px-2">{{ s }}</code>
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
                    <code v-for="s in getSubjectArray(user.subscribe_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1 px-2">{{ s }}</code>
                  </div>
                </div>
                <div v-if="getSubjectArray(user.subscribe_deny_permissions).length">
                  <span class="text-xs text-base-content/50">Deny:</span>
                  <div class="flex flex-wrap gap-1 mt-1">
                    <code v-for="s in getSubjectArray(user.subscribe_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1 px-2">{{ s }}</code>
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
                <h3 class="card-title text-base">Security & Credentials</h3>
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
                      @click="showRegenerateModal = true"
                      class="btn btn-sm btn-outline"
                      title="Issue a fresh .creds file"
                    >
                      <span class="text-lg">🔄</span>
                      Regenerate
                    </button>
                    <button
                      @click="showRevokeModal = true"
                      class="btn btn-sm btn-outline btn-error"
                      title="Reject this user's current credentials"
                    >
                      <span class="text-lg">⛔</span>
                      Revoke
                    </button>
                  </template>
                  <!-- Revoked / inactive user: re-enable with fresh credentials -->
                  <button
                    v-else
                    @click="showReenableModal = true"
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
                <span>
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
    </template>

    <!-- Regenerate Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRegenerateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Regenerate Credentials?</h3>
        <p class="py-4">
          A fresh <code>.creds</code> file will be issued for this user. Note that regeneration
          does <strong>not</strong> invalidate previously distributed credentials — they keep
          working until the user is revoked. To immediately reject the current credentials, use
          <span class="font-semibold">Revoke</span> instead.
        </p>
        <div class="modal-action">
          <button
            class="btn"
            @click="showRegenerateModal = false"
            :disabled="regenerating"
          >
            Cancel
          </button>
          <button
            class="btn btn-warning"
            @click="confirmRegenerate"
            :disabled="regenerating"
          >
            <span v-if="regenerating" class="loading loading-spinner"></span>
            Regenerate
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showRegenerateModal = false">close</button>
      </form>
    </dialog>

    <!-- Revoke Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRevokeModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-error">Revoke Credentials?</h3>
        <p class="py-4">
          This immediately and permanently invalidates every credential currently issued to
          <span class="font-mono font-semibold">{{ user?.nats_username }}</span>. NATS will reject
          their existing <code>.creds</code> file, and the user will be marked inactive.
        </p>
        <p class="pb-4 text-sm text-base-content/70">
          The revocation is permanent and cannot be lifted for the current credentials. You can
          later <span class="font-semibold">Re-enable</span> the user, which issues brand-new
          credentials while the revoked ones stay invalid.
        </p>
        <div class="modal-action">
          <button
            class="btn"
            @click="showRevokeModal = false"
            :disabled="revoking"
          >
            Cancel
          </button>
          <button
            class="btn btn-error"
            @click="confirmRevoke"
            :disabled="revoking"
          >
            <span v-if="revoking" class="loading loading-spinner"></span>
            Revoke
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showRevokeModal = false">close</button>
      </form>
    </dialog>

    <!-- Re-enable Modal -->
    <dialog class="modal" :class="{ 'modal-open': showReenableModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-success">Re-enable User?</h3>
        <p class="py-4">
          This marks <span class="font-mono font-semibold">{{ user?.nats_username }}</span> active
          and issues a fresh <code>.creds</code> file. You'll need to distribute the new credentials
          to the user's devices — any previously revoked credentials remain permanently invalid.
        </p>
        <div class="modal-action">
          <button
            class="btn"
            @click="showReenableModal = false"
            :disabled="reenabling"
          >
            Cancel
          </button>
          <button
            class="btn btn-success"
            @click="confirmReenable"
            :disabled="reenabling"
          >
            <span v-if="reenabling" class="loading loading-spinner"></span>
            Re-enable
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showReenableModal = false">close</button>
      </form>
    </dialog>
  </div>
</template>
