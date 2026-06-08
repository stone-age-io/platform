<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'
import type { LeafNode, NatsUser, NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

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

const canManage = computed(() => authStore.canManageUsers)

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
      expand: 'location,nats_user',
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
        <div class="flex gap-2">
          <router-link :to="`/leaf-nodes/${node.id}/edit`" class="btn btn-primary">Edit</router-link>
          <button @click="handleDelete" class="btn btn-outline btn-error">Delete</button>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
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
              <dt class="text-xs uppercase text-base-content/50">Location</dt>
              <dd>{{ node.expand?.location?.name || '-' }}</dd>
            </div>
            <div>
              <dt class="text-xs uppercase text-base-content/50">NATS User</dt>
              <dd>
                <router-link
                  v-if="node.nats_user"
                  :to="`/nats/users/${node.nats_user}`"
                  class="link link-primary text-sm font-mono"
                >
                  {{ node.expand?.nats_user?.nats_username || node.nats_user }}
                </router-link>
                <span v-else class="text-warning text-sm">Not provisioned yet</span>
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
      </div>

      <!-- NATS role & credentials -->
      <BaseCard v-if="natsUser" title="NATS Role & Credentials">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
          <!-- Role -->
          <div class="space-y-3">
            <div>
              <dt class="text-xs uppercase text-base-content/50">Role</dt>
              <dd class="mt-1">
                <router-link :to="`/nats/roles/${natsUser.role_id}`" class="link link-primary">
                  🎭 {{ natsUser.expand?.role_id?.name || natsUser.role_id }}
                </router-link>
              </dd>
            </div>

            <p class="text-xs text-base-content/60">
              The role defines this edge's publish/subscribe permissions within its NATS account.
              The default <code>leaf-node</code> role is allow-all inside the account.
            </p>

            <div v-if="canManage" class="space-y-2">
              <label class="text-xs uppercase text-base-content/50">Reassign role</label>
              <div class="flex flex-col sm:flex-row gap-2">
                <select v-model="selectedRoleId" class="select select-bordered select-sm flex-1">
                  <option v-for="role in roles" :key="role.id" :value="role.id">
                    {{ role.name }}<span v-if="role.is_default"> (default)</span>
                  </option>
                </select>
                <button
                  class="btn btn-sm btn-primary"
                  :disabled="!roleDirty || savingRole"
                  @click="applyRole"
                >
                  <span v-if="savingRole" class="loading loading-spinner loading-xs"></span>
                  <span v-else>Apply</span>
                </button>
              </div>
              <p class="text-xs text-base-content/50">
                Assign a narrower role to shrink this edge's blast radius.
                <router-link :to="`/nats/roles/${natsUser.role_id}/edit`" class="link">
                  Edit this role's permissions
                </router-link>
                <span v-if="usingSharedRole" class="text-warning">
                  — heads up: the <code>leaf-node</code> role is shared by every leaf node in this org.
                </span>
              </p>
            </div>
          </div>

          <!-- Credentials -->
          <div class="space-y-3">
            <div>
              <dt class="text-xs uppercase text-base-content/50">Credentials</dt>
              <dd class="mt-1 text-sm text-base-content/70">
                The edge fetches its <code>.creds</code> via <code>leaf-sync config</code>.
                Regenerating rotates them — the edge must re-run <code>leaf-sync config</code>.
              </dd>
            </div>
            <div v-if="natsUser.jwt_expires_at" class="text-sm">
              <span class="text-xs uppercase text-base-content/50">JWT expires</span>
              <div>{{ formatDate(natsUser.jwt_expires_at) }}</div>
            </div>
            <button
              v-if="canManage"
              class="btn btn-sm btn-outline btn-error"
              @click="showRegenerateModal = true"
            >
              🔄 Regenerate credentials
            </button>
          </div>
        </div>

        <!-- Per-user permission overrides (merged with the role, union) -->
        <template v-if="hasPermissionOverrides">
          <div class="divider"></div>
          <h4 class="text-xs font-black uppercase opacity-50 tracking-widest mb-2">Permission Overrides</h4>
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
        </template>
      </BaseCard>

      <BaseCard title="Metadata" v-if="node.metadata && Object.keys(node.metadata).length">
        <pre class="bg-base-200 rounded p-3 text-xs overflow-x-auto"><code>{{ JSON.stringify(node.metadata, null, 2) }}</code></pre>
      </BaseCard>

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
  </div>
</template>
