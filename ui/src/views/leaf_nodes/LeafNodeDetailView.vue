<!-- ui/src/views/leaf_nodes/LeafNodeDetailView.vue -->
<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useNow } from '@vueuse/core'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { useNatsStore } from '@/stores/nats'
import { useNatsKv } from '@/composables/useNatsKv'
import { formatDate, formatRelativeTime } from '@/utils/format'
import { generateRandomPassword } from '@/utils/password'
import { leafStatus, type LeafHeartbeat, type LeafStatusState } from '@/utils/leafStatus'
import type { LeafNode, NatsUser, NatsRole, NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import LeafStatusBadge from '@/components/leaf_nodes/LeafStatusBadge.vue'

// The broad role the provisioning hook mints per org. When a leaf node still
// uses it, editing it would affect every leaf node in the org — so we warn.
const LEAF_NODE_ROLE_NAME = 'leaf-node'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()
const authStore = useAuthStore()

const nodeId = route.params.id as string
const node = ref<LeafNode | null>(null)
const natsUser = ref<NatsUser | null>(null)
const roles = ref<NatsRole[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const selectedRoleId = ref('')
const savingRole = ref(false)
const regenerating = ref(false)
const showRegenerateModal = ref(false)

// PocketBase credential reset (the login leaf-sync authenticates with).
const showResetModal = ref(false)
const resettingPassword = ref(false)
const showResetResultModal = ref(false)
const newCredentials = ref({ email: '', password: '' })

const canManage = computed(() => authStore.canManageUsers)

const nebulaHost = computed(() => node.value?.expand?.nebula_host as NebulaHost | undefined)

// Live status from the leaf_status NATS KV (written by leaf-sync). Keyed by code.
const natsStore = useNatsStore()
const { entries: statusEntries, init: initStatus } = useNatsKv('leaf_status')
const now = useNow({ interval: 15000 })

const heartbeat = computed<LeafHeartbeat | undefined>(() =>
  node.value?.code
    ? (statusEntries.value.get(node.value.code)?.value as LeafHeartbeat | undefined)
    : undefined,
)
const liveStatus = computed<LeafStatusState>(() =>
  leafStatus(heartbeat.value, natsStore.isConnected, now.value.getTime()),
)

watch(
  () => natsStore.isConnected,
  (connected) => { if (connected) initStatus() },
  { immediate: true },
)

const usingSharedRole = computed(
  () => natsUser.value?.expand?.role_id?.name === LEAF_NODE_ROLE_NAME,
)
const roleDirty = computed(
  () => !!natsUser.value && selectedRoleId.value !== natsUser.value.role_id,
)

// pb-nats stores per-user permission overrides as JSON; normalize to subjects.
function getSubjectArray(val: any): string[] {
  if (!val) return []
  if (Array.isArray(val)) return val
  if (typeof val === 'string') {
    const trimmed = val.trim()
    if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
      try {
        return JSON.parse(trimmed)
      } catch {
        return []
      }
    }
    return trimmed.split(',').map((s) => s.trim()).filter((s) => s !== '')
  }
  return []
}

const hasPermissionOverrides = computed(() => {
  if (!natsUser.value) return false
  return (
    getSubjectArray(natsUser.value.publish_permissions).length > 0 ||
    getSubjectArray(natsUser.value.subscribe_permissions).length > 0 ||
    getSubjectArray(natsUser.value.publish_deny_permissions).length > 0 ||
    getSubjectArray(natsUser.value.subscribe_deny_permissions).length > 0
  )
})

async function loadNode() {
  loading.value = true
  error.value = null
  try {
    node.value = await pb.collection('leaf_nodes').getOne<LeafNode>(nodeId, {
      expand: 'location,nats_user,nebula_host',
    })
    if (node.value.nats_user) {
      await loadNatsIdentity(node.value.nats_user)
    }
  } catch (err: any) {
    error.value = err.message || 'Failed to load leaf node'
  } finally {
    loading.value = false
  }
}

// The leaf node's NATS identity (its user + role) is loaded separately so the
// role and permission overrides come back fully typed and expanded.
async function loadNatsIdentity(userId: string) {
  try {
    natsUser.value = await pb.collection('nats_users').getOne<NatsUser>(userId, {
      expand: 'role_id',
    })
    selectedRoleId.value = natsUser.value.role_id
    if (canManage.value && roles.value.length === 0) {
      roles.value = await pb.collection('nats_roles').getFullList<NatsRole>({ sort: 'name' })
    }
  } catch {
    // Non-fatal: the identity panel just shows what it can.
  }
}

async function applyRole() {
  if (!natsUser.value || !roleDirty.value) return
  savingRole.value = true
  try {
    await pb.collection('nats_users').update(natsUser.value.id, { role_id: selectedRoleId.value })
    toast.success('NATS role updated')
    await loadNatsIdentity(natsUser.value.id)
  } catch (err: any) {
    toast.error(err.message || 'Failed to update role')
  } finally {
    savingRole.value = false
  }
}

async function confirmRegenerate() {
  if (!natsUser.value) return
  regenerating.value = true
  try {
    await pb.collection('nats_users').update(natsUser.value.id, { regenerate: true })
    toast.success('Credentials regenerated — re-run `leaf-sync config` on the edge')
    showRegenerateModal.value = false
    await loadNatsIdentity(natsUser.value.id)
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

// Reset the leaf node's PocketBase password (manageRule lets org Admins/Owners
// do this). The new password is shown once — leaf-sync must be updated with it.
async function confirmResetPassword() {
  if (!node.value) return
  resettingPassword.value = true
  try {
    const password = generateRandomPassword(24)
    await pb.collection('leaf_nodes').update(nodeId, { password, passwordConfirm: password })
    newCredentials.value = { email: node.value.email, password }
    showResetModal.value = false
    showResetResultModal.value = true
  } catch (err: any) {
    toast.error(err.message || 'Failed to reset credentials')
  } finally {
    resettingPassword.value = false
  }
}

function copyNewCredentials() {
  const text = `PocketBase URL: ${window.location.origin}\nEmail: ${newCredentials.value.email}\nPassword: ${newCredentials.value.password}`
  navigator.clipboard.writeText(text)
  toast.success('Credentials copied to clipboard')
}

function downloadFile(filename: string, content: string, contentType: string) {
  if (!content) {
    toast.error('No content to download')
    return
  }
  const blob = new Blob([content], { type: contentType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function downloadNatsCreds() {
  if (!natsUser.value?.creds_file) {
    toast.error('No credentials file available')
    return
  }
  downloadFile(`${natsUser.value.nats_username}.creds`, natsUser.value.creds_file, 'text/plain')
  toast.success('Credentials downloaded')
}

function downloadNebulaConfig() {
  if (!nebulaHost.value?.config_yaml) {
    toast.error('No configuration available')
    return
  }
  downloadFile(`${nebulaHost.value.hostname}.yaml`, nebulaHost.value.config_yaml, 'text/yaml')
  toast.success('Config downloaded')
}

async function copyMetadata() {
  if (!node.value?.metadata) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(node.value.metadata, null, 2))
    toast.success('Metadata copied to clipboard')
  } catch {
    toast.error('Failed to copy')
  }
}

async function handleDelete() {
  if (!node.value) return
  const confirmed = await confirm({
    title: 'Delete Leaf Node',
    message: `Are you sure you want to delete "${node.value.name || node.value.code}"?`,
    details: 'This removes the edge node record. Its NATS user is not deleted automatically.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await pb.collection('leaf_nodes').delete(nodeId)
    toast.success('Leaf node deleted')
    router.push('/leaf-nodes')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete leaf node')
  }
}

onMounted(loadNode)
</script>

<template>
  <div class="space-y-6">
    <div class="breadcrumbs text-sm">
      <ul>
        <li><router-link to="/leaf-nodes">Leaf Nodes</router-link></li>
        <li>{{ node?.name || node?.code || 'Detail' }}</li>
      </ul>
    </div>

    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load leaf node</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadNode" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <template v-else-if="node">
      <!-- Header -->
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 class="text-3xl font-bold">{{ node.name || 'Unnamed' }}</h1>
          <p v-if="node.description" class="text-base-content/70 mt-1">{{ node.description }}</p>
        </div>
        <div class="flex gap-2 w-full sm:w-auto">
          <router-link :to="`/leaf-nodes/${node.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
          <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left column: identity, synced collections, metadata -->
        <div class="space-y-6">
          <BaseCard title="Identity">
            <dl class="space-y-3">
              <div>
                <dt class="text-xs uppercase text-base-content/50">Code</dt>
                <dd><code class="text-sm">{{ node.code || '-' }}</code></dd>
              </div>
              <div>
                <dt class="text-xs uppercase text-base-content/50">JetStream Domain</dt>
                <dd><code class="text-sm">{{ node.domain || '-' }}</code></dd>
              </div>
              <div>
                <dt class="text-xs uppercase text-base-content/50">leaf-sync Login</dt>
                <dd class="flex items-center justify-between gap-2">
                  <code class="text-sm break-all">{{ node.email }}</code>
                  <button
                    v-if="canManage"
                    @click="showResetModal = true"
                    class="btn btn-xs btn-outline btn-warning shrink-0"
                    title="Reset the PocketBase password leaf-sync authenticates with"
                  >
                    Reset
                  </button>
                </dd>
              </div>
              <div>
                <dt class="text-xs uppercase text-base-content/50">Location</dt>
                <dd>
                  <router-link
                    v-if="node.expand?.location"
                    :to="`/locations/${node.location}`"
                    class="link link-primary hover:no-underline inline-flex items-center gap-1"
                  >
                    📍 {{ node.expand.location.name }}
                  </router-link>
                  <span v-else class="text-base-content/40">No location assigned</span>
                </dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Synced Collections">
            <div v-if="node.synced_collections?.length" class="flex flex-wrap gap-2">
              <span v-for="col in node.synced_collections" :key="col" class="badge badge-ghost">
                <code>{{ col }}</code>
              </span>
            </div>
            <p v-else class="text-base-content/60 text-sm">No collections selected.</p>

            <div class="divider"></div>
            <div class="text-sm text-base-content/70 space-y-1">
              <p class="font-semibold">Bootstrap on the edge:</p>
              <pre class="bg-base-200 rounded p-2 text-xs overflow-x-auto">leaf-sync config   # writes nats-leaf.conf + creds
leaf-sync run      # mirror config → local KV</pre>
            </div>
          </BaseCard>

          <!-- Metadata -->
          <BaseCard v-if="node.metadata && Object.keys(node.metadata).length">
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Metadata</h3>
                <button @click="copyMetadata" class="btn btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100" title="Copy raw JSON">
                  📋 Copy
                </button>
              </div>
            </template>

            <div class="bg-base-200 rounded-lg p-4 border border-base-300 overflow-hidden">
              <div class="max-h-[500px] overflow-y-auto overflow-x-auto custom-scrollbar">
                <JsonViewer :data="node.metadata" class="text-sm leading-relaxed" />
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right column: connectivity (NATS + Nebula — a leaf can have both) -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Connectivity</h3>
                <div class="flex gap-2">
                  <template v-if="natsUser">
                    <button @click="downloadNatsCreds" class="btn btn-sm btn-outline h-8 min-h-0" title="Download .creds file">
                      <span class="text-lg">📥</span>
                      <span class="hidden sm:inline">.creds</span>
                    </button>
                    <button
                      v-if="canManage"
                      @click="showRegenerateModal = true"
                      class="btn btn-sm btn-outline btn-error h-8 min-h-0"
                      title="Regenerate credentials"
                    >
                      🔄
                    </button>
                  </template>
                  <button
                    v-if="nebulaHost"
                    @click="downloadNebulaConfig"
                    class="btn btn-sm btn-outline h-8 min-h-0"
                    title="Download Nebula config"
                  >
                    📥 Config
                  </button>
                </div>
              </div>
            </template>

            <!-- NATS Section -->
            <div class="mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">NATS</span>
            </div>
            <div v-if="natsUser" class="flex flex-col gap-3">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <div class="flex justify-between items-start mb-1">
                  <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Username</span>
                  <div class="flex items-center gap-1.5" v-if="natsUser.active">
                    <span class="w-2 h-2 rounded-full bg-success"></span>
                    <span class="text-xs font-medium text-base-content/70">Active</span>
                  </div>
                  <div class="flex items-center gap-1.5" v-else>
                    <span class="w-2 h-2 rounded-full bg-error"></span>
                    <span class="text-xs font-medium text-base-content/70">Inactive</span>
                  </div>
                </div>
                <router-link :to="`/nats/users/${natsUser.id}`" class="link link-primary font-mono text-base break-all">
                  {{ natsUser.nats_username }}
                </router-link>
              </div>

              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider block mb-1">Role</span>
                <router-link :to="`/nats/roles/${natsUser.role_id}`" class="link link-primary text-sm font-mono">
                  🎭 {{ natsUser.expand?.role_id?.name || natsUser.role_id }}
                </router-link>

                <div v-if="canManage" class="mt-3 space-y-2">
                  <label class="text-xs uppercase text-base-content/50">Reassign role</label>
                  <div class="flex flex-col sm:flex-row gap-2">
                    <select v-model="selectedRoleId" class="select select-bordered select-sm flex-1">
                      <option v-for="role in roles" :key="role.id" :value="role.id">
                        {{ role.name }}<span v-if="role.is_default"> (default)</span>
                      </option>
                    </select>
                    <button class="btn btn-sm btn-primary" :disabled="!roleDirty || savingRole" @click="applyRole">
                      <span v-if="savingRole" class="loading loading-spinner loading-xs"></span>
                      <span v-else>Apply</span>
                    </button>
                  </div>
                  <p v-if="usingSharedRole" class="text-xs text-warning">
                    The <code>leaf-node</code> role is shared by every leaf node in this org.
                  </p>
                </div>
              </div>

              <div v-if="natsUser.jwt_expires_at" class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider block mb-1">Credentials expire</span>
                <div class="text-sm">{{ formatDate(natsUser.jwt_expires_at) }}</div>
              </div>

              <!-- Per-user permission overrides (merged with the role, union) -->
              <div v-if="hasPermissionOverrides" class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider block mb-1">Permission Overrides</span>
                <p class="text-xs text-base-content/60 mb-3">User-level overrides merged with the role (union).</p>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div v-if="getSubjectArray(natsUser.publish_permissions).length || getSubjectArray(natsUser.publish_deny_permissions).length">
                    <span class="text-xs text-base-content/50">📤 Publish</span>
                    <div class="flex flex-wrap gap-1 mt-1">
                      <code v-for="s in getSubjectArray(natsUser.publish_permissions)" :key="`pa-${s}`" class="badge badge-outline font-mono text-xs">{{ s }}</code>
                      <code v-for="s in getSubjectArray(natsUser.publish_deny_permissions)" :key="`pd-${s}`" class="badge badge-error badge-outline font-mono text-xs">!{{ s }}</code>
                    </div>
                  </div>
                  <div v-if="getSubjectArray(natsUser.subscribe_permissions).length || getSubjectArray(natsUser.subscribe_deny_permissions).length">
                    <span class="text-xs text-base-content/50">📥 Subscribe</span>
                    <div class="flex flex-wrap gap-1 mt-1">
                      <code v-for="s in getSubjectArray(natsUser.subscribe_permissions)" :key="`sa-${s}`" class="badge badge-outline font-mono text-xs">{{ s }}</code>
                      <code v-for="s in getSubjectArray(natsUser.subscribe_deny_permissions)" :key="`sd-${s}`" class="badge badge-error badge-outline font-mono text-xs">!{{ s }}</code>
                    </div>
                  </div>
                </div>
                <router-link v-if="canManage" :to="`/nats/users/${natsUser.id}/edit`" class="link text-xs mt-3 inline-block">
                  Edit permission overrides
                </router-link>
              </div>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">📡</span>
              <p class="text-sm">Not provisioned yet</p>
            </div>

            <!-- Divider -->
            <div class="border-t border-base-300 my-4"></div>

            <!-- Nebula Section -->
            <div class="mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Nebula</span>
            </div>
            <div v-if="nebulaHost" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Hostname</span>
                <router-link :to="`/nebula/hosts/${node.nebula_host}`" class="link link-primary font-mono text-sm break-all">
                  {{ nebulaHost.hostname }}
                </router-link>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Overlay IP</span>
                <div class="font-mono text-sm">{{ nebulaHost.overlay_ip }}</div>
              </div>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">🌐</span>
              <p class="text-sm">No Nebula host linked</p>
            </div>
          </BaseCard>

          <BaseCard title="Liveness">
            <div v-if="!natsStore.isConnected" class="space-y-2">
              <LeafStatusBadge :status="liveStatus" />
              <p class="text-sm text-base-content/60">Connect to NATS to see this leaf node's live heartbeat.</p>
            </div>
            <div v-else class="space-y-3">
              <div class="flex items-center justify-between">
                <LeafStatusBadge :status="liveStatus" :hb="heartbeat" />
                <span v-if="heartbeat" class="text-xs text-base-content/50">agent {{ heartbeat.version }}</span>
              </div>
              <template v-if="heartbeat">
                <div>
                  <dt class="text-xs uppercase text-base-content/50">Last heartbeat</dt>
                  <dd class="text-sm">{{ formatRelativeTime(heartbeat.ts) }}</dd>
                </div>
                <div v-if="Object.keys(heartbeat.synced || {}).length">
                  <dt class="text-xs uppercase text-base-content/50">Synced records</dt>
                  <dd class="flex flex-wrap gap-2 mt-1">
                    <span v-for="(count, col) in heartbeat.synced" :key="col" class="badge badge-ghost badge-sm gap-1">
                      <code>{{ col }}</code> {{ count }}
                    </span>
                  </dd>
                </div>
                <div v-if="heartbeat.errors?.length">
                  <dt class="text-xs uppercase text-error">Sync errors</dt>
                  <dd class="mt-1 space-y-1">
                    <p v-for="(e, i) in heartbeat.errors" :key="i" class="text-xs text-error font-mono break-all">{{ e }}</p>
                  </dd>
                </div>
              </template>
              <p v-else class="text-sm text-base-content/60">
                No heartbeat received yet. Ensure <code>leaf-sync run</code> is active and
                <code>nats.hub_domain</code> is set on the edge.
              </p>
            </div>
          </BaseCard>
        </div>
      </div>

      <p class="text-xs text-base-content/50">
        Created {{ formatDate(node.created, 'PPpp') }} · Updated {{ formatDate(node.updated, 'PPpp') }}
      </p>
    </template>

    <!-- Regenerate Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRegenerateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Regenerate Credentials?</h3>
        <p class="py-4">
          This invalidates the leaf node's existing credentials immediately. The edge will lose its
          hub connection until you re-run <code>leaf-sync config</code> to fetch the new
          <code>.creds</code>.
        </p>
        <div class="modal-action">
          <button class="btn" @click="showRegenerateModal = false" :disabled="regenerating">Cancel</button>
          <button class="btn btn-error" @click="confirmRegenerate" :disabled="regenerating">
            <span v-if="regenerating" class="loading loading-spinner"></span>
            Regenerate
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showRegenerateModal = false">close</button>
      </form>
    </dialog>

    <!-- Reset PocketBase credentials: confirm -->
    <dialog class="modal" :class="{ 'modal-open': showResetModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Reset PocketBase Credentials?</h3>
        <p class="py-4">
          This sets a new password for the leaf node's PocketBase login. The current password stops
          working immediately — <code>leaf-sync</code> cannot authenticate until you update
          <code>leaf-sync.yaml</code> with the new password and restart the agent.
        </p>
        <div class="modal-action">
          <button class="btn" @click="showResetModal = false" :disabled="resettingPassword">Cancel</button>
          <button class="btn btn-warning" @click="confirmResetPassword" :disabled="resettingPassword">
            <span v-if="resettingPassword" class="loading loading-spinner"></span>
            Reset Credentials
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showResetModal = false">close</button>
      </form>
    </dialog>

    <!-- Reset PocketBase credentials: new password shown once -->
    <dialog class="modal" :class="{ 'modal-open': showResetResultModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-success">New Credentials</h3>
        <p class="py-4 text-sm text-base-content/70">
          Update <code>leaf-sync.yaml</code> on the edge with these, then restart
          <code>leaf-sync</code>. <strong>Save this password now — it cannot be recovered later.</strong>
        </p>
        <div class="bg-base-200 p-4 rounded-lg space-y-3 font-mono text-sm">
          <div>
            <span class="text-base-content/50 text-xs uppercase block">Email</span>
            <span class="select-all break-all">{{ newCredentials.email }}</span>
          </div>
          <div>
            <span class="text-base-content/50 text-xs uppercase block">Password</span>
            <span class="select-all text-primary font-bold break-all">{{ newCredentials.password }}</span>
          </div>
        </div>
        <div class="modal-action">
          <button class="btn btn-ghost" @click="showResetResultModal = false">Close</button>
          <button class="btn btn-primary" @click="copyNewCredentials">Copy</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showResetResultModal = false">close</button>
      </form>
    </dialog>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
  height: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: oklch(var(--bc) / 0.2);
  border-radius: 10px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
</style>
