<!-- ui/src/components/common/RecordPhoto.vue -->
<script setup lang="ts">
/**
 * The photo on a thing or a location, wherever one is shown read-only.
 *
 * TWO STATES, NOT THREE. A photo here is either resolving or ready. "No photo"
 * is not a state this draws -- it renders nothing at all. Both call sites
 * already gate on `v-if="record.photo"`, and that outer gate is load-bearing
 * rather than redundant: it also drops the flex wrapper, so the card's `gap-5`
 * does not leave a phantom column where a photo would have been. This
 * component's own `v-if` is the backstop for a caller that forgets.
 *
 * RESOLVING STILL NEEDS THE PLATE, which is the one piece of this worth
 * keeping. Every file field is protected, so the URL needs a file token, and
 * that is a round trip on a cold page. Rendering nothing during it and the
 * image afterwards makes the card jump, so the bordered box is drawn at its
 * final size the moment we know there IS a photo, and the image lands inside
 * it.
 *
 * Gone with the two ungated callers (ScannerWidget, ThingMapDrawer): a
 * `hideWhenEmpty` prop, a `No photo` caption, and the `<component :is>` root
 * that existed only to become a plain div when there was nothing to click.
 *
 * NOT FOR PEOPLE. A user's face is UserAvatar, which has an initial-circle
 * fallback and a viewRule that is not org-scoped. Pointing this at `users`
 * would draw a plate for every colleague a caller cannot read.
 */
import { computed, ref } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import { useEscapeKey } from '@/composables/useEscapeKey'

interface Props {
  /** The record holding the photo. */
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

// The thumb, the plate size and click-to-zoom were props until both call sites
// turned out to pass the same three values, which is a constant wearing a
// prop's clothes. They are constants now. Two of them were also carrying a
// caveat each, and the caveat goes with the knob:
//
// THUMB must stay a size the field declares. PocketBase serves 100x100 for any
// file field, but every other size has to be in the field's own `thumbs` list
// or the request silently falls through to the full 2 MiB original with no
// error at all. `TestPhotoThumbsAreDeclared` pins this 400x400 against
// schema.json; a value that disagreed with it would not fail anywhere visible.
//
// Zoom is now unconditional. It used to be opt-in so it could be OFF wherever
// the photo was chrome rather than content -- ScannerWidget and ThingMapDrawer
// -- and both of those are gone. Everything left is a detail view, where the
// whole point is reading a serial off a label the thumbnail cannot resolve.
const THUMB = '400x400'
const SIZE_PX = '140px'

const hasPhoto = computed(() => Boolean(props.record?.id && props.filename))

const url = useFileUrl(() =>
  hasPhoto.value ? { record: props.record!, filename: props.filename, thumb: THUMB } : null,
)

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
    Always a button, never a div with a click handler: the zoom is a real
    control and has to be reachable by keyboard and announced as activatable,
    which only a button gets for free. It can be unconditional now because
    nothing renders at all unless there is a photo to open.
  -->
  <button
    v-if="hasPhoto"
    type="button"
    :aria-label="`View ${alt || 'photo'} full size`"
    class="rounded-lg border border-base-300 bg-base-200 overflow-hidden flex items-center justify-center flex-shrink-0 cursor-zoom-in hover:border-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary transition-colors"
    :style="{ width: SIZE_PX, height: SIZE_PX }"
    @click="zoomOpen = true"
  >
    <!-- object-cover: these are camera photos and the box is square, so
         letterboxing every one of them inside a bordered plate reads as a
         broken image. ImageUploadField makes the opposite call for the opposite
         reason -- there, the point is checking what you uploaded.

         Nothing inside while the token resolves: the bordered box is the
         loading state, at the size the image will be. -->
    <img v-if="url" :src="url" :alt="alt" class="w-full h-full object-cover" />
  </button>

  <!--
    Teleported to <body>, the same shape QrLabelModal uses: the daisyUI CSS
    `modal-open` form rather than dialogEl.showModal(), so the open state stays
    a plain ref. That costs the browser's focus trap and Escape, which is what
    useEscapeKey above is for, and the backdrop click below.
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
        <!-- object-contain here, unlike the thumbnail: this is the view where
             the whole frame matters, so nothing may be cropped out of it.

             The height cap is derived from the chrome rather than picked as a
             fraction. daisyUI caps .modal-box at calc(100vh - 5em) and this box
             is overflow-hidden, so an image tall enough to push past that is
             CLIPPED with no way to scroll to the rest. Leaving 10rem covers
             that 5em plus the title bar with room to spare at every viewport
             height -- and on a desktop it is also taller than the 75vh it
             replaces, so the picture gets bigger rather than smaller. -->
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
