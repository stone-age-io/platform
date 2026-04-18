<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import { DEFAULT_PREFIX } from '@/utils/subjectResolver'
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

const effectivePrefix = computed(() => form.value.subject_prefix?.trim() || DEFAULT_PREFIX)

async function loadOptions() {
  const orgId = authStore.currentOrgId
  // Operations: current org OR platform-shipped (organization = "")
  const opsFilter = orgId
    ? `organization = "${orgId}" || organization = ""`
    : 'organization = ""'
  const ops = await pb.collection('thing_type_operations').getFullList<ThingTypeOperation>({
    filter: opsFilter,
    sort: 'name',
  })
  availableOperations.value = ops

  if (orgId) {
    const roles = await pb.collection('nats_roles').getFullList<NatsRole>({
      filter: `organization = "${orgId}"`,
      sort: 'name',
    })
    availableRoles.value = roles
  }
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
                  Template for NATS subjects. Reserved:
                  <code>{'{org}'}</code>, <code>{'{location}'}</code>,
                  <code>{'{thing}'}</code>, <code>{'{thing_type_code}'}</code>.
                  Defaults to <code>{{ DEFAULT_PREFIX }}</code> when empty.
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
                <span class="label-text-alt">Coarse-grained summary filter. Hold Ctrl/Cmd to select multiple.</span>
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
            <select v-model="form.operations" multiple class="select select-bordered h-40">
              <option v-for="op in availableOperations" :key="op.id" :value="op.id">
                {{ op.name }} ({{ op.capability }}) &middot; {{ op.subject_suffix }}{{ op.organization ? '' : ' · platform' }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                Operations this Thing Type declares. Platform-shipped operations are shared across organizations.
              </span>
            </label>
          </div>

          <div class="form-control">
            <label class="label">Linked NATS Role</label>
            <select v-model="form.nats_role" class="select select-bordered">
              <option value="">— None —</option>
              <option v-for="role in availableRoles" :key="role.id" :value="role.id">
                {{ role.name }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                When linked, the role's publish/subscribe permissions are derived from this Thing Type's operations
                ({'{thing}'} and unset {'{location}'} become NATS wildcards).
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
  </div>
</template>
