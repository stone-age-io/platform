<!-- ui/src/components/common/RecordPhoto.vue -->
<script setup lang="ts">
/**
 * The photo on a thing or a location, wherever one is shown read-only.
 *
 * It exists because the answer to "is there a photo" has THREE states and the
 * obvious two-state rendering gets one of them wrong. A record may have no
 * photo; it may have one that is still resolving (every file field is
 * protected, so the URL needs a file token, which is a round trip on a cold
 * page); and it may have one that is ready. Drawing the empty state during
 * resolution makes a photographed device look unphotographed for a moment and
 * then flicker -- so `pending` renders the same neutral plate as `empty`,
 * sized identically, and only the caption differs.
 *
 * THE FALLBACK IS NOT DECORATION. Most things will not have a photo for a long
 * time, so what gets drawn when there is none is what this component mostly
 * does. It claims the same box either way, because a list where only some rows
 * have photos would otherwise have its text on two different left edges -- the
 * same reasoning as OrgLogo's 'blank' fallback.
 *
 * NOT FOR PEOPLE. A user's face is UserAvatar, which has an initial-circle
 * fallback and a viewRule that is not org-scoped. Pointing this at `users`
 * would draw an empty plate for every colleague a caller cannot read.
 */
import { computed, ref } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import { useEscapeKey } from '@/composables/useEscapeKey'

interface Props {
  /** The record holding the photo. */
  record?: { id: string; collectionId?: string; collectionName?: string } | null
  /** The stored filename, i.e. `record.photo`. */
  filename?: string
  /**
   * Which thumb to request. `things.photo` and `locations.photo` declare
   * 400x400, and PocketBase serves 100x100 for any file field. ANY OTHER VALUE
   * silently falls through to the full 2 MiB original with no error, so treat
   * this as a closed set.
   */
  thumb?: '100x100' | '400x400'
  /** Rendered edge length in px. Square, because the thumbs are. */
  size?: number
  /** Hide the box entirely when there is no photo, instead of a placeholder. */
  hideWhenEmpty?: boolean
  /**
   * Click the thumbnail to open the FULL image in a dialog.
   *
   * Off by default, and it should stay off anywhere the photo is chrome rather
   * than content: a 400x400 thumb is enough to recognise a device, but not to
   * read a serial number off a label or see which way a panel is oriented, and
   * those are the questions a detail view gets asked.
   */
  zoomable?: boolean
  alt?: string
}

const props = withDefaults(defineProps<Props>(), {
  record: null,
  filename: '',
  thumb: '400x400',
  size: 96,
  hideWhenEmpty: false,
  zoomable: false,
  alt: '',
})

const hasPhoto = computed(() => Boolean(props.record?.id && props.filename))

const url = useFileUrl(() =>
  hasPhoto.value ? { record: props.record!, filename: props.filename, thumb: props.thumb } : null,
)

const sizePx = computed(() => `${props.size}px`)

const zoomOpen = ref(false)

// The full image, WITHOUT a thumb parameter -- the point of opening it is to
// see more than the thumbnail showed.
//
// Resolved lazily: the source is null until the dialog opens, so a page with
// several photos on it does not fetch full-size copies of images nobody
// clicked. An upload is capped at 2 MiB and downscaled to 1600px on the long
// edge before it is stored (utils/imageResize.ts), so this is bounded.
const fullUrl = useFileUrl(() =>
  zoomOpen.value && hasPhoto.value ? { record: props.record!, filename: props.filename } : null,
)

// Escape closes it. Safe here, unlike the four dialogs useEscapeKey warns off:
// nothing is lost by dismissing a picture that can be reopened by clicking it
// again.
useEscapeKey(zoomOpen, () => {
  zoomOpen.value = false
})
</script>

<template>
  <!--
    A button when zoomable, a plain div otherwise. Deliberately not a div with a
    click handler in both cases: the zoom is a real control and has to be
    reachable by keyboard and announced as activatable, which only a button
    gets for free.
  -->
  <component
    :is="zoomable && hasPhoto ? 'button' : 'div'"
    v-if="hasPhoto || !hideWhenEmpty"
    :type="zoomable && hasPhoto ? 'button' : undefined"
    :aria-label="zoomable && hasPhoto ? `View ${alt || 'photo'} full size` : undefined"
    class="rounded-lg border border-base-300 bg-base-200 overflow-hidden flex items-center justify-center flex-shrink-0"
    :class="
      zoomable && hasPhoto
        ? 'cursor-zoom-in hover:border-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary transition-colors'
        : ''
    "
    :style="{ width: sizePx, height: sizePx }"
    @click="zoomable && hasPhoto ? (zoomOpen = true) : undefined"
  >
    <!-- object-cover: these are camera photos and the box is square, so
         letterboxing every one of them inside a bordered plate reads as a
         broken image. ImageUploadField makes the opposite call for the opposite
         reason -- there, the point is checking what you uploaded. -->
    <img v-if="url" :src="url" :alt="alt" class="w-full h-full object-cover" />
    <!-- Same plate whether the photo is absent or still resolving: see the note
         at the top about the third state. -->
    <span v-else-if="!hasPhoto" class="text-[10px] text-base-content/40 px-1 text-center">
      No photo
    </span>
  </component>

  <!--
    Teleported to <body>, the same shape QrLabelModal uses: the daisyUI CSS
    `modal-open` form rather than dialogEl.showModal(), so the open state stays
    a plain ref. That costs the browser's focus trap and Escape, which is what
    useEscapeKey above is for, and the backdrop click below.
  -->
  <Teleport to="body">
    <dialog v-if="zoomOpen" class="modal modal-open">
      <div class="modal-box max-w-4xl p-0 bg-base-100 overflow-hidden">
        <div class="flex items-center justify-between px-4 py-2 border-b border-base-300">
          <span class="text-sm font-medium truncate">{{ alt || 'Photo' }}</span>
          <button type="button" class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="zoomOpen = false">
            ✕
          </button>
        </div>
        <!-- object-contain here, unlike the thumbnail: this is the view where
             the whole frame matters, so nothing may be cropped out of it. The
             viewport cap keeps a tall photo from running off the screen. -->
        <div class="flex items-center justify-center bg-base-200">
          <img
            v-if="fullUrl"
            :src="fullUrl"
            :alt="alt"
            class="max-h-[75vh] max-w-full object-contain"
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
