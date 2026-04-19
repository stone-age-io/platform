<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import type { ThingTypeCapability, ThingTypeOperation, MessageSchema } from '@/types/pocketbase'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const loading = ref(false)

const form = ref({
  name: '',
  capability: 'publish' as ThingTypeCapability,
  subject_suffix: '',
  description: '',
  schema: '',
})

const availableCapabilities: ThingTypeCapability[] = ['publish', 'subscribe', 'request', 'reply']
const availableSchemas = ref<MessageSchema[]>([])

function schemaLabel(s: MessageSchema) {
  return `${s.namespace}/${s.name}@${s.version}`
}

async function loadOptions() {
  const orgId = authStore.currentOrgId
  if (!orgId) { availableSchemas.value = []; return }
  availableSchemas.value = await pb.collection('message_schemas').getFullList<MessageSchema>({
    filter: `organization = "${orgId}"`,
    sort: 'namespace,name,-version',
  })
}

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const rec = await pb.collection('thing_type_operations').getOne<ThingTypeOperation>(id)
    form.value = {
      name: rec.name,
      capability: rec.capability,
      subject_suffix: rec.subject_suffix,
      description: rec.description || '',
      schema: rec.schema || '',
    }
  } catch (err: any) {
    toast.error('Failed to load operation')
    router.push('/things/operations')
  } finally {
    loading.value = false
  }
}

async function submit() {
  loading.value = true
  try {
    const payload = { ...form.value, schema: form.value.schema || null }
    if (isEdit.value) {
      await pb.collection('thing_type_operations').update(id!, payload)
      toast.success('Updated')
    } else {
      await pb.collection('thing_type_operations').create({
        ...payload,
        organization: authStore.currentOrgId,
      })
      toast.success('Created')
    }
    router.push('/things/operations')
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
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/operations">Operations</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Operation</h1>
    </div>

    <form @submit.prevent="submit" class="space-y-6">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
        <BaseCard title="Identity">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Name *</label>
              <input
                v-model="form.name"
                type="text"
                class="input input-bordered font-mono"
                required
                pattern="[a-z0-9_]+"
                placeholder="e.g. motion"
              />
              <label class="label"><span class="label-text-alt">Lowercase snake_case.</span></label>
            </div>

            <div class="form-control">
              <label class="label">Capability *</label>
              <select v-model="form.capability" class="select select-bordered" required>
                <option v-for="cap in availableCapabilities" :key="cap" :value="cap">{{ cap }}</option>
              </select>
            </div>

            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>
          </div>
        </BaseCard>

        <BaseCard title="Subject & Schema">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Subject Suffix *</label>
              <input
                v-model="form.subject_suffix"
                type="text"
                class="input input-bordered font-mono"
                required
                placeholder="e.g. motion or cmd.ptz"
              />
              <label class="label">
                <span class="label-text-alt">
                  Appended to the Thing Type's subject prefix with a dot separator.
                  Stored literally — the platform does not resolve templates.
                </span>
              </label>
            </div>

            <div class="form-control">
              <label class="label">Message Schema</label>
              <select v-model="form.schema" class="select select-bordered">
                <option value="">— None —</option>
                <option v-for="s in availableSchemas" :key="s.id" :value="s.id">
                  {{ schemaLabel(s) }}
                </option>
              </select>
              <label class="label">
                <span class="label-text-alt">
                  JSON Schema describing this operation's payload. Optional — consumers fall back to raw JSON when unset.
                </span>
              </label>
            </div>
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
