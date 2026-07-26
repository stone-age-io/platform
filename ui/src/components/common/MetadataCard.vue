<!-- ui/src/components/common/MetadataCard.vue -->
<script setup lang="ts">
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataEditor from './MetadataEditor.vue'

// MetadataCard — the metadata block on a detail page, with quick edit.
//
// Locked, it is a MetadataEditor with `disabled` set, so the read view and the
// edit view are the same component and cannot drift apart in layout or in which
// fields they show. Unlocked, the same editor becomes writable in place.
//
// The save sends ONLY `metadata`. That is not just economy: things.updateRule
// admits a member through a branch requiring `nats_user:changed = false` and
// `nebula_host:changed = false`, and a field absent from the body counts as
// unchanged — so a partial update is exactly what keeps a member's edit legal.
// Sending the whole record back is what would turn an inventory edit into a 404.

interface Props {
  collection: 'things' | 'locations'
  recordId: string
  modelValue: Record<string, any> | null
  /** metadata_schema from the record's type, if it has one. */
  schema?: any
  /** Whether this caller may edit inventory (authStore.can.manageInventory). */
  canEdit?: boolean
}

const props = withDefaults(defineProps<Props>(), { schema: null, canEdit: false })
const emit = defineEmits<{
  (e: 'saved', metadata: Record<string, any> | null): void
}>()

const toast = useToast()

const editing = ref(false)
const saving = ref(false)
const draft = ref<Record<string, any> | null>(null)
const editor = ref<InstanceType<typeof MetadataEditor> | null>(null)

const isEmpty = computed(() => !props.modelValue || Object.keys(props.modelValue).length === 0)

function startEdit() {
  // Deep clone so Cancel genuinely discards — the editor mutates nested values
  // in place for `json` rows.
  draft.value = props.modelValue ? JSON.parse(JSON.stringify(props.modelValue)) : null
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  draft.value = null
}

async function save() {
  // Commit a JSON tab or nested-object row left mid-edit; refuses on a parse
  // error rather than saving a stale value.
  if (editor.value && !editor.value.commit()) return

  saving.value = true
  try {
    await pb.collection(props.collection).update(props.recordId, {
      metadata: draft.value,
    })
    toast.success('Metadata saved')
    emit('saved', draft.value)
    editing.value = false
    draft.value = null
  } catch (err: any) {
    toast.error(err.response?.message || err.message || 'Failed to save metadata')
  } finally {
    saving.value = false
  }
}

async function copyMetadata() {
  if (isEmpty.value) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(props.modelValue, null, 2))
    toast.success('Metadata copied to clipboard')
  } catch {
    toast.error('Failed to copy')
  }
}
</script>

<template>
  <BaseCard>
    <template #header>
      <div class="flex justify-between items-center mb-2 gap-2">
        <h3 class="card-title text-base">Metadata</h3>

        <div class="flex items-center gap-1">
          <button
            v-if="!editing && !isEmpty"
            class="btn btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100"
            title="Copy raw JSON"
            @click="copyMetadata"
          >
            📋 Copy
          </button>

          <button
            v-if="!editing && canEdit"
            class="btn btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100"
            :title="isEmpty ? 'Add metadata' : 'Edit metadata'"
            @click="startEdit"
          >
            {{ isEmpty ? '＋ Add' : '✎ Edit' }}
          </button>

          <template v-if="editing">
            <button class="btn btn-xs btn-ghost" :disabled="saving" @click="cancelEdit">
              Cancel
            </button>
            <button class="btn btn-xs btn-primary" :disabled="saving" @click="save">
              <span v-if="saving" class="loading loading-spinner loading-xs"></span>
              <span v-else>Save</span>
            </button>
          </template>
        </div>
      </div>
    </template>

    <MetadataEditor
      v-if="editing"
      ref="editor"
      v-model="draft"
      :schema="schema"
    />
    <MetadataEditor
      v-else
      :model-value="modelValue"
      :schema="schema"
      disabled
    />
  </BaseCard>
</template>
