<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import DangerZone from '@/components/common/DangerZone.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataSchemaCard from '@/components/common/MetadataSchemaCard.vue'
import RecordTimestamps from '@/components/common/RecordTimestamps.vue'
import TypePrefixField from '@/components/common/TypePrefixField.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const { confirm } = useConfirm()

const deleting = ref(false)

// Delete lives here, not on the list row: a row button is aimed by position,
// which moves under sort, search and pagination.
async function handleDelete() {
  if (!id) return
  const confirmed = await confirm({
    title: 'Delete Location Type',
    message: `Are you sure you want to delete "${form.value.name}"?`,
    details: 'Locations using this type will not be deleted but will lose their type reference.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  deleting.value = true
  try {
    await pb.collection('location_types').delete(id)
    toast.success('Deleted')
    router.push('/locations/types')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
  }
}

const loading = ref(false)

const form = ref({
  name: '',
  description: '',
  code: '',
  prefix: '',
})

// Null means no declared fields; the Location form falls back to free-form rows.
const metadataSchema = ref<Record<string, any> | null>(null)
const metadataSchemaCard = ref<InstanceType<typeof MetadataSchemaCard> | null>(null)

const createdAt = ref<string | undefined>()
const updatedAt = ref<string | undefined>()

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('location_types').getOne(id)
    form.value = {
      name: record.name,
      description: record.description,
      code: record.code,
      prefix: record.prefix || '',
    }
    const raw = record.metadata_schema
    metadataSchema.value = (typeof raw === 'string' ? JSON.parse(raw) : raw) || null
    createdAt.value = record.created
    updatedAt.value = record.updated
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/locations/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (metadataSchemaCard.value && !metadataSchemaCard.value.commit()) return

  loading.value = true
  try {
    const data = {
      ...form.value,
      metadata_schema: metadataSchema.value,
      organization: isEdit.value ? undefined : authStore.currentOrgId
    }
    
    if (isEdit.value) {
      await pb.collection('location_types').update(id!, data)
      toast.success('Updated')
    } else {
      await pb.collection('location_types').create(data)
      toast.success('Created')
    }
    router.push('/locations/types')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (isEdit.value) loadData()
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/locations/types">Location Types</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Location Type</h1>
      <!-- No detail view exists, so this form is where an activity-feed entry
           (hooks/activity.go) is correlated against the record. -->
      <dl v-if="isEdit" class="grid grid-cols-2 gap-4 max-w-sm mt-3">
        <RecordTimestamps :created="createdAt" :updated="updatedAt" />
      </dl>
    </div>

    <form @submit.prevent="submit" class="space-y-6">
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
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

            <TypePrefixField v-model="form.prefix" kind="Location" example="BLD" />
          </div>
        </BaseCard>

        <BaseCard title="Details">
          <div class="space-y-4">
            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="5"></textarea>
            </div>
          </div>
        </BaseCard>
      </div>

      <MetadataSchemaCard
        ref="metadataSchemaCard"
        v-model="metadataSchema"
        noun="place"
      />

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>

    <DangerZone v-if="isEdit">
      <button type="button" @click="handleDelete" class="btn btn-error" :disabled="deleting">
        Delete Location Type
      </button>
    </DangerZone>
  </div>
</template>
