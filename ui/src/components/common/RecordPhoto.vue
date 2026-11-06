<script setup lang="ts">
/**
 * The read-only photo on a thing or location detail view.
 *
 * With no photo it renders nothing. Callers also gate on `v-if="record.photo"`,
 * which is load-bearing: it drops the flex wrapper so the card's gap leaves no
 * empty column. The writer's empty slot is PhotoQuickAdd, in the caller's v-else.
 *
 * While the file token resolves, the bordered plate is drawn at full size so the
 * card does not jump when the image arrives.
 *
 * Not for people: use UserAvatar, whose `users` viewRule is not org-scoped.
 */
import { computed, ref } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import { useEscapeKey } from '@/composables/useEscapeKey'

interface Props {
  record?: { id: string; collectionId?: string; collectionName?: string } | null
  /** The stored filename, i.e. `record.photo`. */
  filename?: string
  alt?: string
}

const props = withDefaults(defineProps<Props>(), {
  record: null,
  filename: '',
  alt: '',
})

// THUMB must be declared in the field's `thumbs` list, or PocketBase silently
// serves the full original. `TestPhotoThumbsAreDeclared` pins 400x400.
const THUMB = '400x400'
const SIZE_PX = '140px'

const hasPhoto = computed(() => Boolean(props.record?.id && props.filename))

const url = useFileUrl(() =>
  hasPhoto.value ? { record: props.record!, filename: props.filename, thumb: THUMB } : null,
)

const zoomOpen = ref(false)

// The full image, no thumb. Resolved only once the dialog opens, so unclicked
// photos are never fetched full size. Uploads are downscaled (utils/imageResize.ts).
const fullUrl = useFileUrl(() =>
  zoomOpen.value && hasPhoto.value ? { record: props.record!, filename: props.filename } : null,
)

// Escape is safe: nothing is lost by dismissing a picture.
useEscapeKey(zoomOpen, () => {
  zoomOpen.value = false
})
</script>

<template>
  <!-- A button, not a clickable div, so the zoom is keyboard-reachable. -->
  <button
    v-if="hasPhoto"
    type="button"
    :aria-label="`View ${alt || 'photo'} full size`"
    class="rounded-lg border border-base-300 bg-base-200 overflow-hidden flex items-center justify-center flex-shrink-0 cursor-zoom-in hover:border-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary transition-colors"
    :style="{ width: SIZE_PX, height: SIZE_PX }"
    @click="zoomOpen = true"
  >
    <!-- object-cover: letterboxed photos read as broken. Empty while the token
         resolves; the plate is the loading state. -->
    <img v-if="url" :src="url" :alt="alt" class="w-full h-full object-cover" />
  </button>

  <!--
    daisyUI's CSS `modal-open` rather than showModal(), so the open state is a
    plain ref. That loses the browser's Escape handling, hence useEscapeKey.
  -->
  <Teleport to="body">
    <dialog v-if="zoomOpen" class="modal modal-open">
      <div class="modal-box w-fit max-w-4xl p-0 bg-base-100 overflow-hidden">
        <div class="flex items-center justify-between px-4 py-2 border-b border-base-300">
          <span class="text-sm font-medium truncate">{{ alt || 'Photo' }}</span>
          <button type="button" class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="zoomOpen = false">
            ✕
          </button>
        </div>
        <!-- object-contain: the whole frame matters here. daisyUI caps
             .modal-box at calc(100vh - 5em) and it is overflow-hidden, so 10rem
             covers that plus the title bar, or a tall image is clipped. -->
        <div class="flex items-center justify-center bg-base-200">
          <img
            v-if="fullUrl"
            :src="fullUrl"
            :alt="alt"
            class="max-h-[calc(100vh-10rem)] max-w-full object-contain"
          />
          <div v-else class="h-64 flex items-center justify-center">
            <span class="loading loading-spinner loading-md"></span>
          </div>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click.prevent="zoomOpen = false">close</button>
      </form>
    </dialog>
  </Teleport>
</template>
