<script setup lang="ts">
/**
 * Pick, preview, replace or remove one image on a form. The one image picker:
 * do not hand-roll another.
 *
 * Staged, like every other field: choosing and removing only change what Save
 * does. Two models (`file` and `removed`) because "no staged file" and "clear
 * the stored image" must stay distinguishable, or the removal is lost on save.
 *
 * Downscales on select (utils/imageResize.ts), here so no caller can forget.
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
  /** The image already on the record; null when there is none or not loaded yet. */
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
  /**
   * What the empty plate says. Prefer this to the `#fallback` slot, which is
   * for a real component in the gap (UserSettingsView draws a UserAvatar).
   */
  emptyLabel?: string
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
  emptyLabel: 'No image',
})

const emit = defineEmits<{
  'update:file': [File | null]
  'update:removed': [boolean]
}>()

const inputRef = ref<HTMLInputElement | null>(null)

// An object URL, not a FileReader data URL: a data URL cannot be released.
const objectUrl = ref<string | null>(null)

function releaseObjectUrl() {
  if (objectUrl.value) {
    URL.revokeObjectURL(objectUrl.value)
    objectUrl.value = null
  }
}

const storedUrl = useFileUrl(() => (props.removed ? null : props.source))

// The staged file wins: the preview shows what Save will upload.
const previewUrl = computed(() => objectUrl.value ?? storedUrl.value)
const hasImage = computed(() => Boolean(previewUrl.value))

const sizePx = computed(() => `${props.size}px`)
const roundedClass = computed(() => (props.shape === 'circle' ? 'rounded-full' : 'rounded-lg'))

// Tied to the prop, not the picker: the parent may clear `file` itself.
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
  // Reset so re-picking the same file after a Remove still fires `change`.
  target.value = ''
  if (!chosen) return

  emit('update:removed', false)
  emit('update:file', await downscaleImage(chosen, props.maxDimension))
}

function clear() {
  emit('update:file', null)
  // Flag removal only when there is a stored image to clear on the server.
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
      <!-- object-contain: cropping the preview would hide what the user is checking. -->
      <img v-if="previewUrl" :src="previewUrl" alt="" class="w-full h-full object-contain" />
      <slot v-else name="fallback">
        <span class="text-xs text-base-content/50 px-2 text-center">{{ emptyLabel }}</span>
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

    <!-- Otherwise a staged removal looks like a record that never had an image. -->
    <p v-if="removed" class="text-xs text-base-content/60">Removed on save</p>
    <p v-else-if="file" class="text-xs text-base-content/60">Uploads on save</p>
  </div>
</template>
