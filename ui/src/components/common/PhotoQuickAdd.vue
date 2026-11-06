<script setup lang="ts">
/**
 * The empty photo slot on a thing or location detail view: a dashed plate (not
 * a skeleton, which reads as loading) that uploads one photo and saves at once,
 * like MetadataCard's quick edit. The caller shows it only under
 * `can.manageInventory`. Add only; replacing or removing is the edit form's job.
 *
 * The body is `photo` and nothing else. Multipart cannot omit a field, and the
 * member branch of things.updateRule freezes several, so any other field could
 * clear one. Section 23 of scripts/test-authz.sh asserts this upload is allowed.
 */
import { ref } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useEscapeKey } from '@/composables/useEscapeKey'
import ImageUploadField from '@/components/common/ImageUploadField.vue'

const props = defineProps<{
  collection: 'things' | 'locations'
  recordId: string
  /** Says what the photo should show, under the picker. */
  hint?: string
}>()

const emit = defineEmits<{
  /** The stored filename, for the caller to put on its record. */
  saved: [filename: string]
}>()

const toast = useToast()

const open = ref(false)
const file = ref<File | null>(null)
// Always false (no stored photo), but ImageUploadField still emits it.
const removed = ref(false)
const saving = ref(false)

function close() {
  if (saving.value) return
  open.value = false
  file.value = null
  removed.value = false
}

async function save() {
  if (!file.value) return
  saving.value = true
  try {
    const body = new FormData()
    body.append('photo', file.value)
    const updated = await pb.collection(props.collection).update(props.recordId, body)
    toast.success('Photo added')
    emit('saved', updated.photo as string)
    saving.value = false
    close()
  } catch (err: any) {
    toast.error(err.message || 'Failed to upload the photo')
    saving.value = false
  }
}

// Escape is safe: it loses at most a picked file, not a one-time secret.
useEscapeKey(open, close)
</script>

<template>
  <!-- 140px, RecordPhoto's plate size, so adding a photo does not shift the layout. -->
  <button
    type="button"
    class="w-[140px] h-[140px] rounded-lg border-2 border-dashed border-base-300 text-base-content/60 flex flex-col items-center justify-center gap-1 flex-shrink-0 hover:border-primary hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary transition-colors"
    @click="open = true"
  >
    <span class="text-2xl leading-none" aria-hidden="true">+</span>
    <span class="text-sm">Add photo</span>
  </button>

  <Teleport to="body">
    <dialog v-if="open" class="modal modal-open">
      <div class="modal-box max-w-sm">
        <h3 class="font-bold text-lg mb-4">Add photo</h3>
        <div class="flex flex-col items-center gap-3">
          <ImageUploadField
            v-model:file="file"
            v-model:removed="removed"
            :size="240"
            :disabled="saving"
            add-label="Choose photo"
            empty-label="No photo chosen"
          />
          <p v-if="hint" class="text-xs text-base-content/60 text-center">{{ hint }}</p>
        </div>
        <div class="modal-action">
          <button type="button" class="btn btn-ghost" :disabled="saving" @click="close">Cancel</button>
          <button type="button" class="btn btn-primary" :disabled="!file || saving" @click="save">
            <span v-if="saving" class="loading loading-spinner loading-sm"></span>
            Save
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click.prevent="close">close</button>
      </form>
    </dialog>
  </Teleport>
</template>
