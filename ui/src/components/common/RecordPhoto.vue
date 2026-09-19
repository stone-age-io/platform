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
import { computed } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'

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
  alt?: string
}

const props = withDefaults(defineProps<Props>(), {
  record: null,
  filename: '',
  thumb: '400x400',
  size: 96,
  hideWhenEmpty: false,
  alt: '',
})

const hasPhoto = computed(() => Boolean(props.record?.id && props.filename))

const url = useFileUrl(() =>
  hasPhoto.value ? { record: props.record!, filename: props.filename, thumb: props.thumb } : null,
)

const sizePx = computed(() => `${props.size}px`)
</script>

<template>
  <div
    v-if="hasPhoto || !hideWhenEmpty"
    class="rounded-lg border border-base-300 bg-base-200 overflow-hidden flex items-center justify-center flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
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
  </div>
</template>
