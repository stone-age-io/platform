<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import { DEFAULT_PREFIX } from '@/utils/subjectResolver'
import ThingTypeOperationFormView from '@/views/things/ThingTypeOperationFormView.vue'
import NatsRoleFormView from '@/views/nats/NatsRoleFormView.vue'
import type { ThingTypeCapability, ThingTypeOperation, NatsRole } from '@/types/pocketbase'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const loading = ref(false)

const form = ref({
  name: '',
  description: '',
  code: '',
  capabilities: [] as ThingTypeCapability[],
  subject_prefix: '',
  operations: [] as string[],
  nats_role: '',
})

const availableCapabilities: ThingTypeCapability[] = ['publish', 'subscribe', 'request', 'reply']

const availableOperations = ref<ThingTypeOperation[]>([])
const availableRoles = ref<NatsRole[]>([])

const showOperationModal = ref(false)
const showRoleModal = ref(false)

const effectivePrefix = computed(() => form.value.subject_prefix?.trim() || DEFAULT_PREFIX)

function onOperationCreated(record: ThingTypeOperation) {
  availableOperations.value.push(record)
  availableOperations.value.sort((a, b) => a.name.localeCompare(b.name))
  if (!form.value.operations.includes(record.id)) {
    form.value.operations.push(record.id)
  }
  showOperationModal.value = false
}

function onRoleCreated(record: NatsRole) {
  availableRoles.value.push(record)
  availableRoles.value.sort((a, b) => a.name.localeCompare(b.name))
  form.value.nats_role = record.id
  showRoleModal.value = false
}

async function loadOptions() {
  const orgId = authStore.currentOrgId
  if (!orgId) {
    availableOperations.value = []
    availableRoles.value = []
    return
  }
  const orgFilter = `organization = "${orgId}"`

  availableOperations.value = await pb.collection('thing_type_operations').getFullList<ThingTypeOperation>({
    filter: orgFilter,
    sort: 'name',
  })
  availableRoles.value = await pb.collection('nats_roles').getFullList<NatsRole>({
    filter: orgFilter,
    sort: 'name',
  })
}

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('thing_types').getOne(id)
    form.value = {
      name: record.name,
      description: record.description,
      code: record.code,
      capabilities: record.capabilities || [],
      subject_prefix: record.subject_prefix || '',
      operations: record.operations || [],
      nats_role: record.nats_role || '',
    }
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/things/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  loading.value = true
  try {
    const data: any = {
      ...form.value,
      nats_role: form.value.nats_role || null,
      organization: isEdit.value ? undefined : authStore.currentOrgId,
    }

    if (isEdit.value) {
      await pb.collection('thing_types').update(id!, data)
      toast.success('Updated')
    } else {
      await pb.collection('thing_types').create(data)
      toast.success('Created')
    }
    router.push('/things/types')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadOptions()
  if (isEdit.value) await loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/types">Thing Types</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Thing Type</h1>
    </div>

    <!-- Form -->
    <form @submit.prevent="submit" class="space-y-6">

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
        <!-- Left Column: Identity -->
        <BaseCard title="Identity">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Name *</label>
              <input v-model="form.name" type="text" class="input input-bordered" required />
            </div>

            <div class="form-control">
              <label class="label">Code</label>
              <input v-model="form.code" type="text" class="input input-bordered font-mono" placeholder="Optional identifier" />
            </div>

            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>
          </div>
        </BaseCard>

        <!-- Right Column: Subject + Capabilities -->
        <BaseCard title="Subject & Capabilities">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Subject Prefix</label>
              <input
                v-model="form.subject_prefix"
                type="text"
                class="input input-bordered font-mono"
                :placeholder="DEFAULT_PREFIX"
              />
              <label class="label">
                <span class="label-text-alt">
                  Template for NATS subjects. Supports
                  <code>{'{org}'}</code>, <code>{'{location}'}</code>,
                  <code>{'{thing}'}</code>, <code>{'{thing_type_code}'}</code>.
                </span>
              </label>
              <p class="text-xs text-base-content/70 mt-1">
                Resolves to: <code class="font-mono">{{ effectivePrefix }}</code>
              </p>
            </div>

            <div class="form-control">
              <label class="label">Capabilities</label>
              <select v-model="form.capabilities" multiple class="select select-bordered h-32">
                <option v-for="cap in availableCapabilities" :key="cap" :value="cap">
                  {{ cap }}
                </option>
              </select>
              <label class="label">
                <span class="label-text-alt">Hold Ctrl/Cmd to select multiple.</span>
              </label>
            </div>
          </div>
        </BaseCard>
      </div>

      <!-- Operations + Role Integration -->
      <BaseCard title="Operations & NATS Role">
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">Operations</label>
            <div class="flex gap-2">
              <select v-model="form.operations" multiple class="select select-bordered flex-1 min-w-0 h-40">
                <option v-for="op in availableOperations" :key="op.id" :value="op.id">
                  {{ op.name }} ({{ op.capability }}) &middot; {{ op.subject_suffix }}
                </option>
              </select>
              <button
                type="button"
                class="btn btn-square btn-outline self-start"
                @click="showOperationModal = true"
                title="Quick Add Operation"
              >
                +
              </button>
            </div>
            <label class="label">
              <span class="label-text-alt">Hold Ctrl/Cmd to select multiple.</span>
            </label>
          </div>

          <div class="form-control">
            <label class="label">Linked NATS Role</label>
            <div class="flex gap-2">
              <select v-model="form.nats_role" class="select select-bordered flex-1 min-w-0">
                <option value="">— None —</option>
                <option v-for="role in availableRoles" :key="role.id" :value="role.id">
                  {{ role.name }}
                </option>
              </select>
              <button
                type="button"
                class="btn btn-square btn-outline"
                @click="showRoleModal = true"
                title="Quick Add NATS Role"
              >
                +
              </button>
            </div>
            <label class="label">
              <span class="label-text-alt">
                Groups Things of this type under a shared role. Set the role's publish/subscribe permissions directly on the role.
              </span>
            </label>
          </div>
        </div>
      </BaseCard>

      <!-- Actions (Outside Card) -->
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>

    <dialog class="modal" :class="{ 'modal-open': showOperationModal }">
      <div class="modal-box w-11/12 max-w-4xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add Operation</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showOperationModal = false">&#x2715;</button>
        </div>
        <div v-if="showOperationModal">
          <ThingTypeOperationFormView :embedded="true" @success="onOperationCreated" @cancel="showOperationModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showOperationModal = false"><button>close</button></form>
    </dialog>

    <dialog class="modal" :class="{ 'modal-open': showRoleModal }">
      <div class="modal-box w-11/12 max-w-4xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add NATS Role</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showRoleModal = false">&#x2715;</button>
        </div>
        <div v-if="showRoleModal">
          <NatsRoleFormView :embedded="true" @success="onRoleCreated" @cancel="showRoleModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showRoleModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
