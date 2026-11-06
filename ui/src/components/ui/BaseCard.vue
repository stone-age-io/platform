<script setup lang="ts">
/** The console's standard card. Use it for all card-based layouts. */
defineProps<{
  title?: string
  noPadding?: boolean
}>()
</script>

<template>
  <!--
    Border plus a hairline shadow, not shadow-xl, which reads as haze on dense
    stacked screens. Matches ResponsiveList's mobile cards; keep them in step.
  -->
  <div class="card bg-base-100 border border-base-300 shadow-sm">
    <div v-if="title || $slots.header || $slots.actions" class="card-body pb-0">
      <!-- Rendered only when filled: an empty row still takes .card-body's flex gap. -->
      <div v-if="title || $slots.actions" class="flex items-center justify-between">
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
