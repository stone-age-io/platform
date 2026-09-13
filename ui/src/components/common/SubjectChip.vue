<!-- ui/src/components/common/SubjectChip.vue -->
<script setup lang="ts">
/**
 * One NATS subject pattern, as a chip.
 *
 * Eight of these were written inline across the NATS user and role detail
 * views, and the two files disagreed: `h-auto py-1 px-2` in one,
 * `h-auto py-1.5 px-2` in the other, for the same chip on two screens an
 * operator moves between. Nobody chose that; it is what copy-paste does.
 *
 * `h-auto` is load-bearing rather than decorative. A daisyUI badge has a fixed
 * height, and a subject like `$JS.edge-s01.API.STREAM.INFO.>` is long enough to
 * wrap at narrow widths -- at a fixed height the second line renders outside
 * the chip's own border. The padding pair replaces the height the badge would
 * otherwise supply.
 *
 * Outline rather than solid: a permission is a rule the operator wrote, not a
 * state of the record. Deny rules are the same chip in error colour, because
 * they are the same KIND of thing -- what separates them is the subject list
 * they came from, and the heading above it already says which.
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
