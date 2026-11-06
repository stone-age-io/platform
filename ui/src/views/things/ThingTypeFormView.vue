<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import DangerZone from '@/components/common/DangerZone.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import { DEFAULT_PREFIX, resolveThing } from '@/utils/subjectResolver'
import OperationSubjects from '@/components/things/OperationSubjects.vue'
import StickyFormActions from '@/components/common/StickyFormActions.vue'
import ThingTypeOperationFormView from '@/views/things/ThingTypeOperationFormView.vue'
import MetadataSchemaCard from '@/components/common/MetadataSchemaCard.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import RecordTimestamps from '@/components/common/RecordTimestamps.vue'
import TypePrefixField from '@/components/common/TypePrefixField.vue'
import type { PickerOption } from '@/types/picker'
import type { ThingTypeOperation } from '@/types/pocketbase'
import { useEscapeKey } from '@/composables/useEscapeKey'

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
    title: 'Delete Thing Type',
    message: `Are you sure you want to delete "${form.value.name}"?`,
    details: 'Things using this type will not be deleted but will lose their type reference.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  deleting.value = true
  try {
    await pb.collection('thing_types').delete(id)
    toast.success('Deleted')
    router.push('/things/types')
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
  subject_prefix: '',
  operations: [] as string[],
})

// Null means no declared fields; the Thing form falls back to free-form rows.
const metadataSchema = ref<Record<string, any> | null>(null)
const metadataSchemaCard = ref<InstanceType<typeof MetadataSchemaCard> | null>(null)

const availableOperations = ref<ThingTypeOperation[]>([])

const createdAt = ref<string | undefined>()
const updatedAt = ref<string | undefined>()

const operationOptions = computed<PickerOption[]>(() =>
  availableOperations.value.map(op => ({
    id: op.id,
    label: op.name,
    sublabel: `${op.capability} · ${op.subject_suffix}`,
  }))
)

const showOperationModal = ref(false)

// Only org and type code resolve here; `{thing}` and `{location}` stay as tokens
// because no Thing exists.
const subjectContext = computed(() => ({
  org: authStore.currentOrg?.code || '',
  thingTypeCode: form.value.code?.trim() || '',
}))

const effectivePrefix = computed(() =>
  resolveThing(form.value.subject_prefix?.trim() || DEFAULT_PREFIX, subjectContext.value),
)

const selectedOperations = computed(() =>
  availableOperations.value.filter(op => form.value.operations.includes(op.id)),
)

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
      prefix: record.prefix || '',
      subject_prefix: record.subject_prefix || '',
      operations: record.operations || [],
    }
    const raw = record.metadata_schema
    metadataSchema.value = (typeof raw === 'string' ? JSON.parse(raw) : raw) || null
    createdAt.value = record.created
    updatedAt.value = record.updated
  } catch (err: any) {
    toast.error('Failed to load type')
    router.push('/things/types')
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (metadataSchemaCard.value && !metadataSchemaCard.value.commit()) return

  loading.value = true
  try {
    const data: any = {
      ...form.value,
      metadata_schema: metadataSchema.value,
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

// See useEscapeKey for why dialogs need this.
useEscapeKey(showOperationModal, () => { showOperationModal.value = false })
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/types">Thing Types</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Thing Type</h1>
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

            <TypePrefixField v-model="form.prefix" kind="Thing" example="CA" />

            <div class="form-control">
              <label class="label">Description</label>
              <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
            </div>
          </div>
        </BaseCard>

        <!-- Prefix, operations and what they resolve to share one card: each
             only means something with the others. -->
        <BaseCard title="Subjects">
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
                  <!-- Single braces render as written in Vue; {'{org}'} is JSX
                       and prints its quotes. -->
                  Template for NATS subjects. Supports
                  <code>{org}</code>, <code>{location}</code>,
                  <code>{thing}</code>, <code>{thing_type_code}</code>.
                  Leave blank for the default. Add <code>{location}</code> only for
                  things that never move: the subject follows the Thing's current
                  location, so moving it moves its subjects.
                </span>
              </label>
              <p class="text-xs text-base-content/70 mt-1">
                Resolves to: <code class="font-mono">{{ effectivePrefix }}</code>
              </p>
            </div>

            <div class="form-control">
              <label class="label">Operations</label>
              <RecordPicker
                v-model="form.operations"
                :options="operationOptions"
                title="Operations"
                placeholder="Select operations..."
                multiple
                empty-text="No operations defined for this organization yet."
              >
                <template #footer="{ close }">
                  <button
                    type="button"
                    class="btn btn-sm btn-ghost w-full justify-start"
                    @click="close(); showOperationModal = true"
                  >
                    + New Operation
                  </button>
                </template>
              </RecordPicker>
            </div>

            <OperationSubjects
              v-if="selectedOperations.length"
              :prefix="form.subject_prefix"
              :operations="selectedOperations"
              :context="subjectContext"
            />
          </div>
        </BaseCard>
      </div>

      <MetadataSchemaCard
        ref="metadataSchemaCard"
        v-model="metadataSchema"
        noun="device"
      />

      <StickyFormActions>
        <button type="button" class="btn btn-ghost flex-1 sm:flex-none" @click="router.back()">Cancel</button>
        <button type="submit" class="btn btn-primary flex-1 sm:flex-none" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </StickyFormActions>
    </form>

    <DangerZone v-if="isEdit">
      <button type="button" @click="handleDelete" class="btn btn-error" :disabled="deleting">
        Delete Thing Type
      </button>
    </DangerZone>

    <dialog class="modal" :class="{ 'modal-open': showOperationModal }">
      <div class="modal-box w-11/12 max-w-4xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add Operation</h3>
          <button class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="showOperationModal = false">&#x2715;</button>
        </div>
        <div v-if="showOperationModal">
          <ThingTypeOperationFormView :embedded="true" @success="onOperationCreated" @cancel="showOperationModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showOperationModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
