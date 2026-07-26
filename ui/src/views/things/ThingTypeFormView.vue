<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import { DEFAULT_PREFIX } from '@/utils/subjectResolver'
import ThingTypeOperationFormView from '@/views/things/ThingTypeOperationFormView.vue'
import SchemaBuilder from '@/components/things/SchemaBuilder.vue'
import type { ThingTypeCapability, ThingTypeOperation } from '@/types/pocketbase'

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
})

const availableCapabilities: ThingTypeCapability[] = ['publish', 'subscribe', 'request', 'reply']

// Inventory metadata schema. Authored here with the same Form/JSON pair the
// message-schema editor uses, then rendered as typed inputs on the Thing form.
// An empty document means "no schema" — the Thing form falls back to free-form
// key/value rows, which is what every type does today.
const metadataSchemaDoc = ref<Record<string, any>>({})
const metadataSchemaTab = ref<'form' | 'json'>('form')
const metadataSchemaText = ref('')
const metadataSchemaError = ref('')

const metadataFieldCount = computed(() =>
  Object.keys(metadataSchemaDoc.value?.properties || {}).length,
)

function refreshMetadataJson() {
  metadataSchemaText.value = metadataFieldCount.value
    ? JSON.stringify(metadataSchemaDoc.value, null, 2)
    : ''
}

watch(metadataSchemaDoc, () => {
  if (metadataSchemaTab.value !== 'json') refreshMetadataJson()
}, { deep: true })

function onMetadataJsonBlur() {
  const text = metadataSchemaText.value.trim()
  if (!text) {
    metadataSchemaError.value = ''
    metadataSchemaDoc.value = {}
    return
  }
  try {
    metadataSchemaDoc.value = JSON.parse(text)
    metadataSchemaError.value = ''
  } catch (err: any) {
    metadataSchemaError.value = err.message
  }
}

function switchMetadataTab(tab: 'form' | 'json') {
  if (tab === metadataSchemaTab.value) return
  if (metadataSchemaTab.value === 'json') {
    onMetadataJsonBlur()
    if (metadataSchemaError.value) {
      toast.error('JSON has errors — fix them before switching views')
      return
    }
  } else {
    refreshMetadataJson()
  }
  metadataSchemaTab.value = tab
}

// Normalise to the shape MetadataEditor/JsonSchemaForm expect. A builder with
// no properties yet produces `{type:'object', properties:{}}`, which is not a
// schema — store null so the Thing form takes the free-form path.
function metadataSchemaPayload(): Record<string, any> | null {
  if (!metadataFieldCount.value) return null
  return { type: 'object', ...metadataSchemaDoc.value }
}

const availableOperations = ref<ThingTypeOperation[]>([])

const showOperationModal = ref(false)

const effectivePrefix = computed(() => form.value.subject_prefix?.trim() || DEFAULT_PREFIX)

function onOperationCreated(record: ThingTypeOperation) {
  availableOperations.value.push(record)
  availableOperations.value.sort((a, b) => a.name.localeCompare(b.name))
  if (!form.value.operations.includes(record.id)) {
    form.value.operations.push(record.id)
  }
  showOperationModal.value = false
}

async function loadOptions() {
  const orgId = authStore.currentOrgId
  if (!orgId) {
    availableOperations.value = []
    return
  }
  const orgFilter = `organization = "${orgId}"`

  availableOperations.value = await pb.collection('thing_type_operations').getFullList<ThingTypeOperation>({
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
    }
    const raw = record.metadata_schema
    metadataSchemaDoc.value = (typeof raw === 'string' ? JSON.parse(raw) : raw) || {}
    refreshMetadataJson()
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/things/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (metadataSchemaTab.value === 'json') {
    onMetadataJsonBlur()
    if (metadataSchemaError.value) {
      toast.error('Metadata schema is not valid JSON')
      return
    }
  }

  loading.value = true
  try {
    const data: any = {
      ...form.value,
      metadata_schema: metadataSchemaPayload(),
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

      <BaseCard title="Operations">
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
      </BaseCard>

      <BaseCard title="Inventory Fields">
        <p class="text-sm text-base-content/70 mb-4">
          Describes what is tracked about each device of this type — a service date,
          an asset tag, a warranty reference. Members filling in a Thing get these as
          typed inputs instead of a JSON blob. Leave empty to let them add free-form
          fields instead.
          <span class="block mt-1 text-base-content/50">
            Rendering only: existing Things are never invalidated by a change here,
            and fields not listed are kept.
          </span>
        </p>

        <div role="tablist" class="tabs tabs-bordered mb-4">
          <a
            role="tab"
            class="tab"
            :class="{ 'tab-active': metadataSchemaTab === 'form' }"
            @click="switchMetadataTab('form')"
          >Form</a>
          <a
            role="tab"
            class="tab"
            :class="{ 'tab-active': metadataSchemaTab === 'json' }"
            @click="switchMetadataTab('json')"
          >JSON</a>
        </div>

        <SchemaBuilder v-if="metadataSchemaTab === 'form'" v-model="metadataSchemaDoc" />

        <div v-else class="form-control">
          <textarea
            v-model="metadataSchemaText"
            class="textarea textarea-bordered font-mono text-xs"
            rows="12"
            placeholder='{"type":"object","properties":{"last_service":{"type":"string","format":"date"}}}'
            @blur="onMetadataJsonBlur"
          ></textarea>
          <label v-if="metadataSchemaError" class="label">
            <span class="label-text-alt text-error">{{ metadataSchemaError }}</span>
          </label>
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
  </div>
</template>
