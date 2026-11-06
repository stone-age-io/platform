<script setup lang="ts">
/**
 * An organization's mark: its uploaded logo, or a fallback when it has none
 * (the common case). This is the tenant's identity; BrandLogo is the
 * operator's, and BrandLogo.vue says why the two must stay separate.
 *
 * It fetches no record of its own; callers pass an already-loaded organization.
 */
import { computed } from 'vue'
import { useFileUrl } from '@/composables/useFileUrl'
import type { Organization } from '@/types/pocketbase'

interface Props {
  org?: Organization | null
  /** Rendered edge length in px. */
  size?: number
  /**
   * What to draw with no logo. 'initial' (the letter tile) where the mark is the
   * only org identity on screen, such as the switcher trigger. 'blank' keeps the
   * box empty, for rows that already show the name: on a selected switcher row
   * (daisyUI `.menu li > *.active` is bg-neutral) the tile falls below 3:1
   * contrast. The box is kept so mixed lists stay aligned.
   */
  fallback?: 'initial' | 'blank'
}

const props = withDefaults(defineProps<Props>(), {
  org: null,
  size: 32,
  fallback: 'initial',
})

const sizePx = computed(() => `${props.size}px`)

// Use a thumb the `logo` field declares (schema.json); 100x100 covers this
// component's sizes at 2x DPR.
const logoUrl = useFileUrl(() => {
  const org = props.org
  if (!org?.logo || !org.id) return null
  return { record: org as { id: string }, filename: org.logo, thumb: '100x100' }
})

// '?' for a name not yet loaded; an empty box reads as a rendering bug.
const initial = computed(() => props.org?.name?.[0]?.toUpperCase() || '?')
</script>

<template>
  <!-- A neutral plate, so a transparent logo stays visible in both themes. -->
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
    <span class="font-bold leading-none" :style="{ fontSize: `${Math.round(size * 0.4)}px` }">
      {{ initial }}
    </span>
  </div>

  <div
    v-else
    class="flex-shrink-0"
    :style="{ width: sizePx, height: sizePx }"
    aria-hidden="true"
  ></div>
</template>
