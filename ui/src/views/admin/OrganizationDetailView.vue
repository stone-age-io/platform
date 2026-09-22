<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useFileUrl } from '@/composables/useFileUrl'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { Organization, User, NatsAccount } from '@/types/pocketbase'
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

// Moved off the list row. Deleting an organization blanks rather than cascades
// -- every relation into it that is non-cascade and non-required is emptied,
// which orphans the entire inventory -- so the confirm carries the typed gate.
// The code is the org's own namespace root and is immutable, which makes it the
// one string that cannot have drifted from what the reader is looking at.
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

// NATS Account state
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

const isOperator = computed(() => authStore.isOperator)

const id = route.params.id as string

// Protected, so resolving the URL needs a file token: see useFileUrl. The 200x200
// thumb is one the `logo` field declares (schema.json).
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

    // NO MEMBER/THING COUNTS HERE, and the reason is worth keeping written down
    // because the code that used to do it carried a comment asserting the
    // opposite: "Operator bypasses rules, so we must manually filter by org id."
    //
    // An operator does not bypass anything. `is_operator` is a field on `users`,
    // not superuser, and of the thirteen tenant collections only `audit_logs`
    // carries an is_operator read branch -- `things` and `memberships` have
    // none, deliberately (an operator reads org metadata, users and
    // infrastructure, not tenant inventory). So the two counts were scoped by
    // the reader's OWN current_organization, not by the organization on screen:
    // Things read 0 for every org but the one you happen to be switched into,
    // and Members read 0, or 1 where the operator held a membership themselves.
    //
    // The failure had no symptom. A list rule is applied as an extra WHERE, so
    // both calls returned 200 with totalItems 0 -- the page stated "0 Things"
    // about a tenant with hundreds of devices, in the same typeface it states
    // everything else.
    //
    // Restoring these needs the number to come from somewhere the reader can
    // actually see: either an is_operator read branch on both collections (an
    // authorization change, with the authz suite bumped to match) or an
    // operator-gated route returning named aggregates with the app's own
    // privileges, the shape GET /api/me/leaf-config uses to hand over facts
    // without granting the collection. Not a filter on the client.

    // Load NATS Account if operator
    if (isOperator.value) {
      await loadNatsAccount()
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
    toast.error(`Failed to update limits: ${err.message}`)
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

      <!--
        What the red badge above actually means, spelled out where someone
        reading this page during an incident will see it.

        The badge is old; the enforcement behind it is not (hooks/org_active_flag.go),
        and for as long as the flag meant nothing the badge was a claim about a
        tenant that no part of the platform was making good on. Now that it does
        mean something, the two halves both need saying: what stopped, and what
        did not. A reader who assumes "Inactive" locks the tenant out of the
        console will misread every other screen from here on.
      -->
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
        <!-- Info -->
        <div class="md:col-span-2">
          <BaseCard title="Details">
            <dl class="space-y-4">
              <!--
                Code comes first, and it is on this page at all because it is the
                one identifier here that anything outside the database addresses:
                the root of the organization namespace (ADR 0002), the token the
                hub rewrites managed helpdesk subjects through, and a string that
                ends up printed on labels. The id below it is storage.

                It was also the one field the page did not show while the delete
                confirm asked the reader to TYPE it -- a typed gate whose answer
                was nowhere on screen.
              -->
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
            <!--
              "It may still be provisioning" was not true and cost a reader the
              only move available to them: provisioning runs synchronously in the
              organization's after-create/after-update hook and RETURNS its error
              rather than logging it (hooks/org_provisioning.go), so there is no
              pending state to wait out. A missing account means the save that
              should have made it failed, and somebody was told at the time.

              Saying so also names the remedy, which is on this screen: the hook
              is create-if-missing and bound to update as well as create, so
              re-saving the organization retries it.
            -->
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
