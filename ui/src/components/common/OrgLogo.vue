<!-- ui/src/components/common/OrgLogo.vue -->
<script setup lang="ts">
/**
 * An organization's mark: its uploaded logo, or the initial square when it has
 * none.
 *
 * This is where a TENANT's identity belongs. BrandLogo is the OPERATOR's, and
 * the two used to be the same component -- BrandLogo resolved `org logo >
 * branding > default`, so an organization with a logo replaced the product mark
 * in the sidebar, right beside the operator's own app name. See the comment in
 * BrandLogo.vue for why that was wrong and where it came from.
 *
 * THE FALLBACK IS THE COMMON CASE, not an afterthought. Most organizations will
 * never upload a logo, so what gets drawn when there is none decides how this
 * looks nearly everywhere. Which fallback is right depends on whether the mark
 * is carrying the identity on its own -- see the `fallback` prop, where the
 * contrast measurements that settled it are written down.
 *
 * NO RECORD FETCH OF ITS OWN. Every caller already holds a full organization
 * record: `authStore.currentOrg` for the trigger,
 * `membership.expand.organization` for each row of the switcher. pb.files needs
 * only the record's collection and id, both of which an expanded record
 * carries, so rendering a logo per row costs nothing beyond the image. The one
 * request it does make is for a file token, and that is cached across every
 * caller on the page (utils/fileToken.ts), so a switcher listing ten
 * organizations still makes at most one.
 */
import { computed } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import type { Organization } from '@/types/pocketbase'

interface Props {
  org?: Organization | null
  /** Rendered edge length in px. */
  size?: number
  /**
   * What to draw when the organization has no logo, which is the common case.
   *
   * 'initial' is the letter tile, and it is right wherever the mark is the ONLY
   * org identity on screen -- the switcher trigger, above all in compact mode,
   * where the name is hidden.
   *
   * 'blank' reserves the same box and draws nothing. It is for a row that
   * already names the organization beside the mark, and it is not merely a
   * taste call: the letter tile is `text-primary` on `bg-primary/15`, which
   * measures 4.67:1 on the trigger (over base-200) but only 2.15:1 in light
   * and 2.76:1 in dark on a SELECTED switcher row, because daisyUI paints
   * `.menu li > *.active` with bg-neutral. Both are under the 3:1 floor. The
   * box is still emitted so a list of organizations, some with logos and some
   * without, stays aligned.
   */
  fallback?: 'initial' | 'blank'
}

const props = withDefaults(defineProps<Props>(), {
  org: null,
  size: 32,
  fallback: 'initial',
})

const sizePx = computed(() => `${props.size}px`)

// The `logo` field declares 100x100 and 200x200 thumbs (schema.json). Anything
// this component renders is chrome-sized, so 100x100 covers it even at 2x DPR;
// asking for a thumb the field does not declare would make PocketBase generate
// one per request.
//
// Protected, so resolving the URL is async: see useFileUrl. The server checks
// the token against the organizations viewRule, which is operator-or-member --
// exactly the set of organizations the switcher can list in the first place.
const logoUrl = useFileUrl(() => {
  const org = props.org
  if (!org?.logo || !org.id) return null
  return { record: org as { id: string }, filename: org.logo, thumb: '100x100' }
})

// '?' rather than a blank square: an organization whose name has not loaded yet
// is a different state from one with a single-character name, and a silently
// empty box reads as a rendering bug.
const initial = computed(() => props.org?.name?.[0]?.toUpperCase() || '?')
</script>

<template>
  <!--
    The logo sits on a bordered base-200 plate, the same framing
    OrganizationDetailView and OrganizationFormView use. It is not decoration:
    an uploaded mark is usually transparent, and without a plate a dark logo
    disappears into the dark theme and a light one into the light theme. A fixed
    neutral backing means the operator does not have to upload two files.
  -->
  <div
    v-if="logoUrl"
    class="rounded-md border border-base-300 bg-base-200 overflow-hidden flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
  >
    <img :src="logoUrl" :alt="`${org?.name} logo`" class="w-full h-full object-contain" />
  </div>

  <div
    v-else-if="fallback === 'initial'"
    class="rounded-md bg-primary/15 text-primary border border-primary/20 flex items-center justify-center flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
    aria-hidden="true"
  >
    <!-- Scaled off the box rather than fixed, so the same component works at the
         switcher trigger's 32px and a dropdown row's 20px. -->
    <span class="font-bold leading-none" :style="{ fontSize: `${Math.round(size * 0.4)}px` }">
      {{ initial }}
    </span>
  </div>

  <!-- Nothing to draw, but the space is still claimed: without it a list where
       only some organizations have logos would have its names on two different
       left edges. -->
  <div
    v-else
    class="flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
    aria-hidden="true"
  ></div>
</template>
