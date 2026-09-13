<script setup lang="ts">
/**
 * Base Card Component
 *
 * Simple wrapper around DaisyUI card with consistent styling.
 * Use this for all card-based layouts.
 */
defineProps<{
  title?: string
  noPadding?: boolean
}>()
</script>

<template>
  <!--
    Border plus a hairline shadow, not shadow-xl.

    This one line is the card treatment for the whole console -- 178 instances
    across 52 files go through it. shadow-xl is a landing-page elevation, and
    on a dense operations screen where cards are stacked and nested it reads as
    haze rather than depth. The border does the separating work that the page
    ground (base-200) and the card surface (base-100) only half do on their own.

    It also settles a split the app already had: ResponsiveList renders its own
    mobile cards as border + border-base-300 + shadow-sm, so the desktop and
    phone halves of the SAME list were drawn in two different languages. This
    is the mobile one, which was the better of the two.
  -->
  <div class="card bg-base-100 border border-base-300 shadow-sm">
    <div v-if="title || $slots.header || $slots.actions" class="card-body pb-0">
      <div class="flex items-center justify-between">
        <h2 v-if="title" class="card-title">{{ title }}</h2>
        <div v-if="$slots.actions" class="flex items-center gap-2">
          <slot name="actions" />
        </div>
      </div>
      <slot name="header" />
    </div>
    <div v-if="!noPadding" class="card-body">
      <slot />
    </div>
    <slot v-else />
  </div>
</template>
