<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useFileUrl } from '@/composables/useFileUrl'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import { apiErrorMessage } from '@/utils/apiError'
import { expiryState, expiryLabel, CA_EXPIRY_WARNING_DAYS } from '@/utils/expiry'
import { caRotationState } from '@/utils/nebula'
import type { Organization, User, NatsAccount, NebulaCA, OrgStats } from '@/types/pocketbase'
import { useConfirm } from '@/composables/useConfirm'
import BaseCard from '@/components/ui/BaseCard.vue'
import DangerZone from '@/components/common/DangerZone.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

interface OrganizationWithExpand extends Organization {
  expand?: {
    owner?: User
  }
}

const org = ref<OrganizationWithExpand | null>(null)
const { confirm } = useConfirm()
const deleting = ref(false)

// Deleting an organization blanks its relations rather than cascading, which
// orphans the whole inventory, hence the typed gate. The code is immutable.
async function handleDelete() {
  if (!org.value) return
  const confirmed = await confirm({
    title: 'Delete Organization',
    message: `Are you sure you want to delete "${org.value.name}"?`,
    details: 'This will delete ALL data associated with this organization including users, things, locations, and configurations. This action cannot be undone.',
    confirmText: 'Delete Organization',
    variant: 'danger',
    requireText: org.value.code || org.value.name || undefined,
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('organizations').delete(org.value.id)
    toast.success('Organization deleted')
    router.push('/organizations')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
  }
}
const loading = ref(true)

const natsAccount = ref<NatsAccount | null>(null)
const editingLimits = ref(false)
const savingLimits = ref(false)
const limitsForm = ref({
  max_connections: -1,
  max_subscriptions: -1,
  max_data: -1,
  max_payload: -1,
  max_jetstream_disk_storage: -1,
  max_jetstream_memory_storage: -1,
})

// Readable via nebula_ca's is_operator branch; private_key is hidden. Summarised,
// not linked: /nebula/ca shows the reader's own org's CA.
const nebulaCa = ref<NebulaCA | null>(null)
const caExpiry = computed(() => expiryState(nebulaCa.value?.expires_at, CA_EXPIRY_WARNING_DAYS))
const caExpiryText = computed(() => expiryLabel(nebulaCa.value?.expires_at, CA_EXPIRY_WARNING_DAYS))
const caRotation = computed(() => caRotationState(nebulaCa.value))

// Left null on failure, so the card hides rather than showing a false 0.
const stats = ref<OrgStats | null>(null)

const isOperator = computed(() => authStore.isOperator)

const id = route.params.id as string

// 200x200 is a thumb the `logo` field declares (schema.json).
const logoUrl = useFileUrl(() => {
  const record = org.value
  if (!record?.logo) return null
  return { record, filename: record.logo, thumb: '200x200' }
})

async function loadData() {
  loading.value = true
  try {
    org.value = await pb.collection('organizations').getOne<OrganizationWithExpand>(id, {
      expand: 'owner',
    })

    // Counts come from org_stats, never the tenant collections: `things`,
    // `locations` and `memberships` have no operator read branch, so a getList
    // there silently returns 0. org_stats is an operator-only view of counts
    // (migrations/schema_update_org_stats.go, test-authz.sh section 27).
    if (isOperator.value) {
      await Promise.all([loadNatsAccount(), loadNebulaCa(), loadStats()])
    }
  } catch (err: any) {
    toast.error('Failed to load organization')
    router.push('/organizations')
  } finally {
    loading.value = false
  }
}

async function loadNatsAccount() {
  try {
    const result = await pb.collection('nats_accounts').getList<NatsAccount>(1, 1, {
      filter: `organization = "${id}"`,
    })
    if (result.items.length > 0) {
      natsAccount.value = result.items[0]
    }
  } catch (err) {
    console.error('Failed to load NATS account:', err)
    toast.error('Failed to load NATS account')
  }
}

async function loadNebulaCa() {
  try {
    const result = await pb.collection('nebula_ca').getList<NebulaCA>(1, 1, {
      filter: pb.filter('organization = {:id}', { id }),
    })
    nebulaCa.value = result.items[0] ?? null
  } catch (err) {
    console.error('Failed to load Nebula CA:', err)
    toast.error('Failed to load Nebula CA')
  }
}

async function loadStats() {
  try {
    stats.value = await pb.collection('org_stats').getOne<OrgStats>(id)
  } catch (err) {
    console.error('Failed to load organization counts:', err)
  }
}

function startEditingLimits() {
  if (!natsAccount.value) return
  limitsForm.value = {
    max_connections: natsAccount.value.max_connections ?? -1,
    max_subscriptions: natsAccount.value.max_subscriptions ?? -1,
    max_data: natsAccount.value.max_data ?? -1,
    max_payload: natsAccount.value.max_payload ?? -1,
    max_jetstream_disk_storage: natsAccount.value.max_jetstream_disk_storage ?? -1,
    max_jetstream_memory_storage: natsAccount.value.max_jetstream_memory_storage ?? -1,
  }
  editingLimits.value = true
}

function cancelEditingLimits() {
  editingLimits.value = false
}

async function saveLimits() {
  if (!natsAccount.value) return
  savingLimits.value = true
  try {
    await pb.collection('nats_accounts').update(natsAccount.value.id, limitsForm.value)
    toast.success('NATS account limits updated')
    await loadNatsAccount()
    editingLimits.value = false
  } catch (err: any) {
    // A limit of 0 is refused, and the reason is in the field errors.
    toast.error(`Failed to update limits: ${apiErrorMessage(err, 'unknown error')}`)
  } finally {
    savingLimits.value = false
  }
}

function formatLimit(value: number | undefined): string {
  if (value === undefined || value === -1) return 'Unlimited'
  return value.toLocaleString()
}

function formatBytes(bytes: number | undefined): string {
  if (bytes === undefined || bytes === -1) return 'Unlimited'
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

onMounted(() => loadData())
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="org">
      <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
        <div>
          <div class="breadcrumbs text-sm">
            <ul>
              <li><router-link to="/organizations">Organizations</router-link></li>
              <li>{{ org.name }}</li>
            </ul>
          </div>
          <div class="flex items-center gap-3">
            <div
              v-if="logoUrl"
              class="w-12 h-12 rounded-lg border border-base-300 bg-base-200 overflow-hidden flex-shrink-0"
            >
              <img :src="logoUrl" :alt="`${org.name} logo`" class="w-full h-full object-contain" />
            </div>
            <h1 class="text-3xl font-bold">{{ org.name }}</h1>
            <StatusBadge :active="org.active" size="md" />
            <span v-if="org.is_system_org" class="badge badge-neutral">System</span>
            <span v-if="org.is_operator_org" class="badge badge-secondary">Operator</span>
            <span v-if="org.managed" class="badge badge-primary">Managed</span>
          </div>
        </div>
        <div class="flex gap-2">
          <router-link :to="`/organizations/${org.id}/edit`" class="btn btn-primary">
            Edit
          </router-link>
        </div>
      </div>

      <!-- Says both what suspension stops and what it does not (hooks/org_active_flag.go). -->
      <div v-if="!org.active" class="alert alert-warning">
        <span>
          <strong>This organization is deactivated.</strong>
          Its NATS account is withdrawn, so every device, edge agent and browser
          session in the tenant is disconnected. Credentials remain valid and
          reconnect when it is reactivated. Console sign-in, inventory and the
          Nebula overlay are unaffected.
        </span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div class="md:col-span-2">
          <BaseCard title="Details">
            <dl class="space-y-4">
              <!-- Code is for addressing (ADR 0002), the id is storage. Shown because
                   the delete confirm asks for it. -->
              <div>
                <dt class="text-sm font-medium opacity-70">Code</dt>
                <dd class="font-mono text-sm">{{ org.code || '—' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">ID</dt>
                <dd class="font-mono text-sm">{{ org.id }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Owner</dt>
                <dd>
                  {{ org.expand?.owner?.email || 'Unknown' }}
                  <span v-if="org.expand?.owner?.name" class="opacity-70">
                    ({{ org.expand.owner.name }})
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Description</dt>
                <dd>{{ org.description || '—' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Created</dt>
                <dd>{{ formatDate(org.created) }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>

        <!-- "Locations", never "sites": a location can be one room, and a site
             is a pricing unit with no record. -->
        <div v-if="isOperator && stats">
          <BaseCard title="Inventory">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium opacity-70">Things</dt>
                <dd class="text-2xl font-semibold">
                  {{ stats.things.toLocaleString() }}
                  <span
                    v-if="stats.things_active !== stats.things"
                    class="text-sm font-normal opacity-70"
                  >
                    {{ stats.things_active.toLocaleString() }} active
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Locations</dt>
                <dd class="text-2xl font-semibold">{{ stats.locations.toLocaleString() }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Members</dt>
                <dd class="text-2xl font-semibold">{{ stats.members.toLocaleString() }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>

        <!-- The two per-tenant boundaries: NATS account and Nebula CA. -->
        <div v-if="isOperator" class="md:col-span-3">
          <BaseCard title="Identity">
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <div>
                <h3 class="text-sm font-semibold uppercase tracking-wide opacity-70 mb-3">NATS account</h3>
                <dl v-if="natsAccount" class="space-y-3">
                  <div>
                    <dt class="text-sm font-medium opacity-70">Name</dt>
                    <dd class="flex items-center gap-2">
                      {{ natsAccount.name }}
                      <StatusBadge :active="natsAccount.active !== false" />
                    </dd>
                  </div>
                  <div v-if="natsAccount.public_key">
                    <dt class="text-sm font-medium opacity-70">Public key</dt>
                    <dd class="font-mono text-xs break-all">{{ natsAccount.public_key }}</dd>
                  </div>
                </dl>
                <p v-else class="text-sm opacity-70">No NATS account. See the limits card below.</p>
              </div>

              <div>
                <h3 class="text-sm font-semibold uppercase tracking-wide opacity-70 mb-3">Nebula certificate authority</h3>
                <dl v-if="nebulaCa" class="space-y-3">
                  <div>
                    <dt class="text-sm font-medium opacity-70">Name</dt>
                    <dd class="flex items-center gap-2 flex-wrap">
                      {{ nebulaCa.name }}
                      <span
                        v-if="caExpiryText"
                        class="badge"
                        :class="caExpiry === 'expired' ? 'badge-error' : 'badge-warning'"
                      >
                        {{ caExpiryText }}
                      </span>
                      <span v-if="caRotation === 'prepared'" class="badge badge-info">Rotation prepared</span>
                      <span v-else-if="caRotation === 'rotated'" class="badge badge-info">Rotation committed</span>
                    </dd>
                  </div>
                  <div class="flex gap-8">
                    <div>
                      <dt class="text-sm font-medium opacity-70">Curve</dt>
                      <dd class="font-mono text-sm">{{ nebulaCa.curve || 'P256' }}</dd>
                    </div>
                    <div v-if="nebulaCa.expires_at">
                      <dt class="text-sm font-medium opacity-70">Expires</dt>
                      <dd class="text-sm">{{ formatDate(nebulaCa.expires_at) }}</dd>
                    </div>
                  </div>
                </dl>
                <!-- Provisioning runs synchronously in the save hook, so a missing CA
                     is a failed save and re-saving retries. The System org is skipped
                     there (hooks/org_provisioning.go); bootstrap provisions it. -->
                <p v-else-if="org.is_system_org" class="text-sm opacity-70">
                  No Nebula CA found. The System organization's CA is provisioned by
                  bootstrap, so re-running bootstrap retries it.
                </p>
                <p v-else class="text-sm opacity-70">
                  No Nebula CA found. Provisioning runs when the organization is saved,
                  so saving it again retries.
                </p>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- NATS Account Limits (Operators only) -->
        <div v-if="isOperator && natsAccount" class="md:col-span-3">
          <BaseCard title="NATS Account Limits">
            <template #actions>
              <template v-if="!editingLimits">
                <button class="btn btn-sm btn-ghost" @click="startEditingLimits">
                  Edit
                </button>
              </template>
              <template v-else>
                <button class="btn btn-sm btn-ghost" @click="cancelEditingLimits" :disabled="savingLimits">
                  Cancel
                </button>
                <button class="btn btn-sm btn-primary" @click="saveLimits" :disabled="savingLimits">
                  <span v-if="savingLimits" class="loading loading-spinner loading-xs"></span>
                  Save
                </button>
              </template>
            </template>

            <!-- View Mode -->
            <div v-if="!editingLimits" class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
              <div>
                <dt class="text-sm font-medium opacity-70">Max Connections</dt>
                <dd class="text-lg font-semibold">{{ formatLimit(natsAccount.max_connections) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Max Subscriptions</dt>
                <dd class="text-lg font-semibold">{{ formatLimit(natsAccount.max_subscriptions) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Max Data</dt>
                <dd class="text-lg font-semibold">{{ formatBytes(natsAccount.max_data) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">Max Payload</dt>
                <dd class="text-lg font-semibold">{{ formatBytes(natsAccount.max_payload) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">JetStream Disk</dt>
                <dd class="text-lg font-semibold">{{ formatBytes(natsAccount.max_jetstream_disk_storage) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium opacity-70">JetStream Memory</dt>
                <dd class="text-lg font-semibold">{{ formatBytes(natsAccount.max_jetstream_memory_storage) }}</dd>
              </div>
            </div>

            <!-- Edit Mode -->
            <div v-else class="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div class="form-control">
                <label class="label py-1"><span class="label-text">Max Connections</span></label>
                <input
                  v-model.number="limitsForm.max_connections"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
              <div class="form-control">
                <label class="label py-1"><span class="label-text">Max Subscriptions</span></label>
                <input
                  v-model.number="limitsForm.max_subscriptions"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
              <div class="form-control">
                <label class="label py-1"><span class="label-text">Max Data (bytes)</span></label>
                <input
                  v-model.number="limitsForm.max_data"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
              <div class="form-control">
                <label class="label py-1"><span class="label-text">Max Payload (bytes)</span></label>
                <input
                  v-model.number="limitsForm.max_payload"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
              <div class="form-control">
                <label class="label py-1"><span class="label-text">JetStream Disk (bytes)</span></label>
                <input
                  v-model.number="limitsForm.max_jetstream_disk_storage"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
              <div class="form-control">
                <label class="label py-1"><span class="label-text">JetStream Memory (bytes)</span></label>
                <input
                  v-model.number="limitsForm.max_jetstream_memory_storage"
                  type="number"
                  class="input input-bordered input-sm"
                  min="-1"
                  :disabled="savingLimits"
                />
                <label class="label py-0"><span class="label-text-alt">-1 = unlimited</span></label>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- No NATS Account Message (Operators only) -->
        <div v-else-if="isOperator && !natsAccount" class="md:col-span-3">
          <BaseCard title="NATS Account Limits">
            <!-- No pending state: the save hook provisions synchronously and returns
                 its error, and is create-if-missing on update too (hooks/org_provisioning.go). -->
            <div class="text-center py-4 opacity-70">
              <p>No NATS account found for this organization.</p>
              <p class="text-sm">
                Provisioning runs when the organization is saved, so this means it
                failed rather than that it is still in progress. Saving the
                organization again retries it.
              </p>
            </div>
          </BaseCard>
        </div>
      </div>

      <DangerZone>
        <button @click="handleDelete" class="btn btn-error" :disabled="deleting">
          Delete Organization
        </button>
      </DangerZone>
    </template>
  </div>
</template>
