<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import SchemaBuilder from '@/components/things/SchemaBuilder.vue'
import type { MessageSchema } from '@/types/pocketbase'

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: MessageSchema): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id && !props.embedded)
const loading = ref(false)

const form = ref({
  namespace: '',
  name: '',
  version: '1.0.0',
  description: '',
})

// Source of truth for the schema document itself. Both the builder and JSON
// textarea stay in sync with it.
const schemaDoc = ref<Record<string, any>>({ type: 'object', properties: {} })
const schemaText = ref('')
const jsonError = ref('')
const activeTab = ref<'form' | 'json'>('form')

// Infer-from-sample modal state
const showInferModal = ref(false)
const sampleText = ref('')
const sampleError = ref('')

function inferSchema(value: any): Record<string, any> {
  if (value === null) return { type: 'null' }
  if (Array.isArray(value)) {
    if (value.length === 0) return { type: 'array', items: {} }
    const itemSchemas = value.map(inferSchema)
    const firstKey = JSON.stringify(itemSchemas[0])
    const allSame = itemSchemas.every(s => JSON.stringify(s) === firstKey)
    return { type: 'array', items: allSame ? itemSchemas[0] : { anyOf: itemSchemas } }
  }
  if (typeof value === 'object') {
    const properties: Record<string, any> = {}
    const required: string[] = []
    for (const [k, v] of Object.entries(value)) {
      properties[k] = inferSchema(v)
      required.push(k)
    }
    return required.length
      ? { type: 'object', properties, required }
      : { type: 'object', properties }
  }
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value)) return { type: 'string', format: 'date-time' }
    if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return { type: 'string', format: 'date' }
    if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) return { type: 'string', format: 'email' }
    if (/^https?:\/\//.test(value)) return { type: 'string', format: 'uri' }
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) return { type: 'string', format: 'uuid' }
    return { type: 'string' }
  }
  if (typeof value === 'number') {
    return Number.isInteger(value) ? { type: 'integer' } : { type: 'number' }
  }
  if (typeof value === 'boolean') return { type: 'boolean' }
  return {}
}

function openInferModal() {
  sampleText.value = ''
  sampleError.value = ''
  showInferModal.value = true
}

function applyInferred() {
  let parsed: any
  try {
    parsed = JSON.parse(sampleText.value)
  } catch (err: any) {
    sampleError.value = err.message
    return
  }
  schemaDoc.value = inferSchema(parsed)
  refreshJsonFromDoc()
  showInferModal.value = false
  toast.success('Schema inferred from sample — review before saving')
}

function refreshJsonFromDoc() {
  schemaText.value = JSON.stringify(schemaDoc.value, null, 2)
}

// Whenever the builder (via schemaDoc) changes, keep the JSON textarea fresh —
// but only when the user isn't actively editing it.
watch(schemaDoc, () => {
  if (activeTab.value !== 'json') refreshJsonFromDoc()
}, { deep: true })

function onJsonBlur() {
  try {
    const parsed = JSON.parse(schemaText.value)
    jsonError.value = ''
    schemaDoc.value = parsed
  } catch (err: any) {
    jsonError.value = err.message
  }
}

function switchTab(tab: 'form' | 'json') {
  if (tab === activeTab.value) return
  if (activeTab.value === 'json') {
    // Leaving JSON — try to parse so the form sees the latest.
    try {
      schemaDoc.value = JSON.parse(schemaText.value)
      jsonError.value = ''
    } catch (err: any) {
      jsonError.value = err.message
      toast.error('JSON has errors — fix them before switching views')
      return
    }
  } else {
    refreshJsonFromDoc()
  }
  activeTab.value = tab
}

async function loadData() {
  if (!id || props.embedded) return
  loading.value = true
  try {
    const rec = await pb.collection('message_schemas').getOne<MessageSchema>(id)
    form.value = {
      namespace: rec.namespace,
      name: rec.name,
      version: rec.version,
      description: rec.description || '',
    }
    const raw = rec.schema
    schemaDoc.value = typeof raw === 'string' ? JSON.parse(raw) : raw
    refreshJsonFromDoc()
  } catch (err: any) {
    toast.error('Failed to load schema')
    router.push('/things/schemas')
  } finally {
    loading.value = false
  }
}

async function submit() {
  // If the user is on the JSON tab, use that as the source of truth.
  if (activeTab.value === 'json') {
    try {
      schemaDoc.value = JSON.parse(schemaText.value)
      jsonError.value = ''
    } catch (err: any) {
      jsonError.value = err.message
      toast.error('Schema is not valid JSON')
      return
    }
  }

  loading.value = true
  try {
    const payload: any = {
      namespace: form.value.namespace,
      name: form.value.name,
      version: form.value.version,
      format: 'json_schema',
      description: form.value.description,
      schema: schemaDoc.value,
    }
    let record: MessageSchema
    if (isEdit.value) {
      record = await pb.collection('message_schemas').update<MessageSchema>(id!, payload)
      toast.success('Updated')
    } else {
      payload.organization = authStore.currentOrgId
      record = await pb.collection('message_schemas').create<MessageSchema>(payload)
      toast.success('Created')
    }

    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/things/schemas')
    }
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('cancel')
  } else {
    router.back()
  }
}

onMounted(async () => {
  if (isEdit.value) {
    await loadData()
  } else {
    refreshJsonFromDoc()
  }
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/schemas">Message Schemas</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Message Schema</h1>
    </div>

    <form @submit.prevent="submit" class="space-y-6">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
        <BaseCard title="Identity">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Namespace *</label>
              <input
                v-model="form.namespace"
                type="text"
                class="input input-bordered font-mono"
                required
                pattern="[a-z0-9_]+"
                placeholder="e.g. ip_camera or common"
              />
            </div>

            <div class="form-control">
              <label class="label">Name *</label>
              <input
                v-model="form.name"
                type="text"
                class="input input-bordered font-mono"
                required
                pattern="[a-z0-9_]+"
                placeholder="e.g. motion or heartbeat"
              />
            </div>

            <div class="form-control">
              <label class="label">Version *</label>
              <input
                v-model="form.version"
                type="text"
                class="input input-bordered font-mono"
                required
                pattern="[0-9]+\.[0-9]+\.[0-9]+"
                placeholder="1.0.0"
              />
              <label class="label"><span class="label-text-alt">Semantic version (e.g. 1.0.0). Each version is a separate record.</span></label>
            </div>

            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>
          </div>
        </BaseCard>

        <BaseCard title="Schema Document">
          <div class="flex items-center justify-between mb-4">
            <div role="tablist" class="tabs tabs-bordered">
              <a
                role="tab"
                class="tab"
                :class="{ 'tab-active': activeTab === 'form' }"
                @click="switchTab('form')"
              >Form</a>
              <a
                role="tab"
                class="tab"
                :class="{ 'tab-active': activeTab === 'json' }"
                @click="switchTab('json')"
              >JSON</a>
            </div>
            <button
              type="button"
              class="btn btn-sm btn-outline"
              @click="openInferModal"
            >
              Infer from sample
            </button>
          </div>

          <SchemaBuilder v-if="activeTab === 'form'" v-model="schemaDoc" />

          <div v-else class="form-control">
            <textarea
              v-model="schemaText"
              class="textarea textarea-bordered font-mono text-xs"
              rows="18"
              required
              @blur="onJsonBlur"
            ></textarea>
            <label class="label">
              <span class="label-text-alt" :class="{ 'text-error': jsonError }">
                {{ jsonError || 'Draft-07 or later JSON Schema.' }}
              </span>
            </label>
          </div>
        </BaseCard>
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost" @click="handleCancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>

    <dialog class="modal" :class="{ 'modal-open': showInferModal }">
      <div class="modal-box w-11/12 max-w-2xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Infer Schema from Sample</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showInferModal = false">&#x2715;</button>
        </div>
        <p class="text-sm text-base-content/70 mb-3">
          Paste an example JSON message. The inferred schema is a starting point —
          fields are marked required and no constraints (enums, min/max, patterns) are guessed.
          Review and refine in the Form or JSON view before saving.
        </p>
        <div class="form-control">
          <textarea
            v-model="sampleText"
            class="textarea textarea-bordered font-mono text-xs"
            rows="14"
            placeholder='{"example": "paste your JSON message here"}'
          ></textarea>
          <label v-if="sampleError" class="label">
            <span class="label-text-alt text-error">{{ sampleError }}</span>
          </label>
        </div>
        <div class="modal-action">
          <button type="button" class="btn btn-ghost" @click="showInferModal = false">Cancel</button>
          <button type="button" class="btn btn-primary" :disabled="!sampleText.trim()" @click="applyInferred">
            Infer Schema
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showInferModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
