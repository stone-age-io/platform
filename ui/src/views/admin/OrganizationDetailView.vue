<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { Organization, User, NatsAccount } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

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
const stats = ref({ members: 0, things: 0 })
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

async function loadData() {
  loading.value = true
  try {
    org.value = await pb.collection('organizations').getOne<OrganizationWithExpand>(id, {
      expand: 'owner',
    })

    // Load Stats (Operator bypasses rules, so we must manually filter by org id)
    const [m, t] = await Promise.all([
      pb.collection('memberships').getList(1, 1, { filter: `organization = "${id}"` }),
      pb.collection('things').getList(1, 1, { filter: `organization = "${id}"` }),
    ])

    stats.value = { members: m.totalItems, things: t.totalItems }

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
            <h1 class="text-3xl font-bold">{{ org.name }}</h1>
            <span class="badge" :class="org.active ? 'badge-success' : 'badge-error'">
              {{ org.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
        </div>
        <div class="flex gap-2">
          <router-link :to="`/organizations/${org.id}/edit`" class="btn btn-primary">
            Edit
          </router-link>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <!-- Stats -->
        <div class="stats shadow bg-base-100 w-full md:col-span-3">
          <div class="stat">
            <div class="stat-title">Members</div>
            <div class="stat-value">{{ stats.members }}</div>
          </div>
          <div class="stat">
            <div class="stat-title">Things</div>
            <div class="stat-value text-primary">{{ stats.things }}</div>
          </div>
        </div>

        <!-- Info -->
        <div class="md:col-span-2">
          <BaseCard title="Details">
            <dl class="space-y-4">
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
            <div class="text-center py-4 opacity-70">
              <p>No NATS account found for this organization.</p>
              <p class="text-sm">It may still be provisioning or was not created.</p>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
