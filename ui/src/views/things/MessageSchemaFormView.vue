<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import type { MessageSchema, MessageSchemaFormat } from '@/types/pocketbase'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const loading = ref(false)
const isPlatform = ref(false)

const form = ref({
  namespace: '',
  name: '',
  version: '1.0.0',
  format: 'json_schema' as MessageSchemaFormat,
  description: '',
  schemaText: '{\n  "type": "object",\n  "properties": {}\n}',
})

const jsonError = ref('')

function validateJson() {
  try {
    JSON.parse(form.value.schemaText)
    jsonError.value = ''
    return true
  } catch (err: any) {
    jsonError.value = err.message
    return false
  }
}

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const rec = await pb.collection('message_schemas').getOne<MessageSchema>(id)
    isPlatform.value = !rec.organization
    form.value = {
      namespace: rec.namespace,
      name: rec.name,
      version: rec.version,
      format: rec.format,
      description: rec.description || '',
      schemaText: typeof rec.schema === 'string'
        ? rec.schema
        : JSON.stringify(rec.schema, null, 2),
    }
  } catch (err: any) {
    toast.error('Failed to load schema')
    router.push('/things/schemas')
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (!validateJson()) {
    toast.error('Schema is not valid JSON')
    return
  }
  loading.value = true
  try {
    const payload: any = {
      namespace: form.value.namespace,
      name: form.value.name,
      version: form.value.version,
      format: form.value.format,
      description: form.value.description,
      schema: JSON.parse(form.value.schemaText),
    }
    if (isEdit.value) {
      await pb.collection('message_schemas').update(id!, payload)
      toast.success('Updated')
    } else {
      payload.organization = authStore.currentOrgId
      await pb.collection('message_schemas').create(payload)
      toast.success('Created')
    }
    router.push('/things/schemas')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => { if (isEdit.value) loadData() })
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/schemas">Message Schemas</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Message Schema</h1>
    </div>

    <div v-if="isPlatform" class="alert alert-warning">
      <span>This is a platform-shipped schema. Edits may not be permitted by API rules.</span>
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
              <label class="label"><span class="label-text-alt">Semver. New versions = new records.</span></label>
            </div>

            <div class="form-control">
              <label class="label">Format *</label>
              <select v-model="form.format" class="select select-bordered" required>
                <option value="json_schema">json_schema</option>
              </select>
            </div>

            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>
          </div>
        </BaseCard>

        <BaseCard title="Schema Document">
          <div class="form-control">
            <label class="label">JSON Schema *</label>
            <textarea
              v-model="form.schemaText"
              class="textarea textarea-bordered font-mono text-xs"
              rows="18"
              required
              @blur="validateJson"
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
        <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>
  </div>
</template>
