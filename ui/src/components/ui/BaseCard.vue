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
  <div class="card bg-base-100 shadow-xl">
    <div v-if="title || $slots.header" class="card-body pb-0">
      <h2 v-if="title" class="card-title">{{ title }}</h2>
      <slot name="header" />
    </div>
    <div v-if="!noPadding" class="card-body">
      <slot />
    </div>
    <slot v-else />
  </div>
</template>
