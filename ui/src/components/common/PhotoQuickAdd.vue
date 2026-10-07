<!-- ui/src/components/common/PhotoQuickAdd.vue -->
<script setup lang="ts">
/**
 * The empty photo slot on a thing or location DETAIL view, for someone who can
 * fill it: a dashed plate where RecordPhoto would be, opening a small dialog
 * that uploads one photo and saves it at once.
 *
 * WRITERS ONLY. The caller shows this under `can.manageInventory` and nothing
 * otherwise, so a reader still sees what RecordPhoto promises -- no photo, no
 * plate, the fields taking the full width. An empty box tells someone who
 * cannot fill it nothing at all. That is the half of the earlier "no empty
 * state" decision that still holds; the half that changed is that the person
 * standing at the device with a phone is usually the one who can add it, and
 * the edit form was three taps away.
 *
 * NOT A SKELETON. A skeleton says "loading", and an empty slot that looks like
 * it is still loading reads as a broken page. This is a dashed outline with a
 * label, which reads as a place to put something.
 *
 * SAVES IMMEDIATELY, like MetadataCard's quick edit, because that is what a
 * control on a detail view does here. The edit forms stage the photo until Save
 * instead; the two share ImageUploadField, the picker, and nothing else -- one
 * component switching between immediate and staged on a prop would be the
 * mode flag RecordPhoto shed.
 *
 * THE BODY IS THE PHOTO AND NOTHING ELSE, and that matters for `things`: the
 * member branch of its updateRule freezes several fields, and multipart has no
 * null and no way to omit, so a FormData carrying more than `photo` could clear
 * one of them. A body with one field leaves every other field absent, which the
 * rule reads as unchanged. Section 23 of scripts/test-authz.sh asserts that a
 * member's photo-only multipart update is allowed.
 *
 * Add only, no replace or remove: the photo's own dialog is for looking, and
 * changing a photo that exists is the edit form's job.
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
// ImageUploadField's removal model. Always false here -- there is no stored
// photo to remove -- but the field still emits it.
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

// Escape is safe here: the worst it loses is a picked file, which is picked
// again in two taps. Not one of the one-time-secret dialogs useEscapeKey
// warns about.
useEscapeKey(open, close)
</script>

<template>
  <!-- 140px, RecordPhoto's plate size, so adding a photo does not move the
       fields beside it. -->
  <button
    type="button"
    class="w-[140px] h-[140px] rounded-lg border-2 border-dashed border-base-300 text-base-content/60 flex flex-col items-center justify-center gap-1 flex-shrink-0 hover:border-primary hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary transition-colors"
    @click="open = true"
  >
    <span class="text-2xl leading-none" aria-hidden="true">+</span>
    <span class="text-sm">Add photo</span>
  </button>

  <!-- Teleported, the same daisyUI `modal-open` shape RecordPhoto uses. -->
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
