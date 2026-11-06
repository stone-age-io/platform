<script setup lang="ts">
/**
 * One NATS subject pattern, as a chip.
 *
 * `h-auto` is load-bearing: a daisyUI badge has a fixed height, and a long
 * subject that wraps would spill outside the border. The padding replaces it.
 *
 * Outline, not solid: a permission is a rule, not a record state (see
 * StatusBadge). Deny rules are the same chip in error colour.
 */
defineProps<{
  subject: string
  /** A deny rule rather than an allow rule. */
  deny?: boolean
}>()
</script>

<template>
  <code
    class="badge badge-outline font-mono text-xs h-auto py-1 px-2"
    :class="{ 'badge-error': deny }"
  >{{ subject }}</code>
</template>
