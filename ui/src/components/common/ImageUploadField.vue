<!-- ui/src/components/common/ImageUploadField.vue -->
<script setup lang="ts">
/**
 * Pick, preview, replace or remove one image on a form.
 *
 * WHY IT EXISTS. There are five of these -- the user avatar, the organization
 * logo, a location floorplan, and now a photo on things and locations -- and
 * before this there were three hand-rolled copies that had already drifted
 * apart in ways that matter. OrganizationFormView revoked its object URLs and
 * supported staged removal; UserSettingsView used a FileReader data URL, which
 * holds the whole image in memory as base64 and cannot be revoked at all;
 * LocationFormView had no removal path, so a floorplan uploaded by mistake
 * could only be replaced, never cleared. Each was correct-looking on its own.
 *
 * STAGED, NOT IMMEDIATE. Choosing a file and clicking Remove both change what
 * SAVE will do; neither touches the server. That is what every other field on
 * these forms does, and an image control that wrote through on click would be
 * the only one that could not be abandoned by navigating away.
 *
 * THE THREE STATES ARE NOT TWO. A stored image, a locally-chosen replacement,
 * and a pending removal all have to stay distinguishable, because "no pending
 * file" and "clear what is stored" look identical if removal is modelled as the
 * absence of a selection -- and the removal is then silently dropped on save.
 * Hence two models rather than one.
 *
 * DOWNSCALES ON SELECT. See utils/imageResize.ts. It happens here rather than
 * in each parent so no caller can forget it and quietly start storing 5 MB
 * camera originals.
 */
import { ref, computed, watch, onUnmounted } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import { downscaleImage } from '@/utils/imageResize'
import type { FileSource } from '@/utils/fileToken'

interface Props {
  /** The file staged for upload, or null. */
  file: File | null
  /** Whether the stored image is staged for removal. */
  removed?: boolean
  /**
   * The image already on the record. Null when there is none, or when the
   * record has not loaded yet. Resolved through useFileUrl because every file
   * field on this platform is protected and needs a file token.
   */
  source?: FileSource | null
  shape?: 'circle' | 'square'
  /** Rendered edge length in px. */
  size?: number
  accept?: string
  disabled?: boolean
  /** Longest edge kept on upload; see utils/imageResize.ts. */
  maxDimension?: number
  /** Label for the button that opens the picker when nothing is set. */
  addLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  removed: false,
  source: null,
  shape: 'square',
  size: 128,
  accept: 'image/jpeg,image/png,image/webp',
  disabled: false,
  maxDimension: undefined,
  addLabel: 'Add image',
})

const emit = defineEmits<{
  'update:file': [File | null]
  'update:removed': [boolean]
}>()

const inputRef = ref<HTMLInputElement | null>(null)

// An object URL for the staged file. Deliberately not a FileReader data URL:
// that base64-encodes the whole image into a string that cannot be released,
// which on a 5 MB camera photo is several megabytes of retained memory per
// picked file.
const objectUrl = ref<string | null>(null)

function releaseObjectUrl() {
  if (objectUrl.value) {
    URL.revokeObjectURL(objectUrl.value)
    objectUrl.value = null
  }
}

// The stored image, when there is one and it is not staged for removal.
const storedUrl = useFileUrl(() => (props.removed ? null : props.source))

// The staged file wins: once a file is chosen the preview has to show what Save
// will actually upload, not what is currently on the record.
const previewUrl = computed(() => objectUrl.value ?? storedUrl.value)
const hasImage = computed(() => Boolean(previewUrl.value))

const sizePx = computed(() => `${props.size}px`)
const roundedClass = computed(() => (props.shape === 'circle' ? 'rounded-full' : 'rounded-lg'))

// The parent may clear `file` itself (after a successful save, or on reset), so
// the object URL is tied to the prop rather than only to the picker.
watch(
  () => props.file,
  (file) => {
    releaseObjectUrl()
    if (file) objectUrl.value = URL.createObjectURL(file)
  },
)

onUnmounted(releaseObjectUrl)

async function onSelected(event: Event) {
  const target = event.target as HTMLInputElement
  const chosen = target.files?.[0]
  // Reset immediately, so re-picking the SAME file after a Remove still fires a
  // change event. Without it the control is dead until a different file is
  // chosen.
  target.value = ''
  if (!chosen) return

  emit('update:removed', false)
  emit('update:file', await downscaleImage(chosen, props.maxDimension))
}

function clear() {
  emit('update:file', null)
  // Only a STORED image needs removing on the server. Discarding a file that
  // was merely staged is a local undo, and flagging it would send a pointless
  // clear for a record that never had one.
  emit('update:removed', Boolean(props.source?.filename))
}

function openPicker() {
  inputRef.value?.click()
}
</script>

<template>
  <div class="flex flex-col items-center gap-3">
    <div
      class="overflow-hidden border border-base-300 bg-base-200 flex items-center justify-center flex-shrink-0"
      :class="roundedClass"
      :style="{ width: sizePx, height: sizePx }"
    >
      <!-- object-contain, not cover: this is the control where someone checks
           what they uploaded, and cropping the preview hides exactly the part
           of the frame they might need to correct. -->
      <img v-if="previewUrl" :src="previewUrl" alt="" class="w-full h-full object-contain" />
      <slot v-else name="fallback">
        <span class="text-xs text-base-content/50 px-2 text-center">No image</span>
      </slot>
    </div>

    <input
      ref="inputRef"
      type="file"
      :accept="accept"
      class="hidden"
      :disabled="disabled"
      @change="onSelected"
    />

    <div v-if="!disabled" class="flex gap-2">
      <button type="button" class="btn btn-xs btn-outline" @click="openPicker">
        {{ hasImage ? 'Change' : addLabel }}
      </button>
      <button v-if="hasImage" type="button" class="btn btn-xs btn-ghost text-error" @click="clear">
        Remove
      </button>
    </div>

    <!-- Says what Save will do. Without it, staging a removal looks identical to
         a record that never had an image, and the difference only shows up
         after saving. -->
    <p v-if="removed" class="text-xs text-base-content/60">Removed on save</p>
    <p v-else-if="file" class="text-xs text-base-content/60">Uploads on save</p>
  </div>
</template>
