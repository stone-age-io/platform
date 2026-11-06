<script setup lang="ts">
/**
 * The Active / Inactive chip for a record that carries an `active` flag.
 *
 * Badge fill convention:
 *   solid   -- the record's own stored state (this component).
 *   outline -- advisory or derived (ExpiryBadge).
 *   ghost   -- neutral metadata, not a state (a type, a count).
 *
 * No `badge-lg`: the chip shape comes from the theme's `--rounded-badge`
 * (tailwind.config.js), which is shared verbatim with the access-control console.
 */
withDefaults(defineProps<{
  /** Undefined counts as inactive: a record with no flag has not opted in. */
  active?: boolean
  size?: 'xs' | 'sm' | 'md'
}>(), { size: 'sm' })
</script>

<template>
  <span
    class="badge"
    :class="[
      active ? 'badge-success' : 'badge-error',
      size === 'md' ? '' : `badge-${size}`,
    ]"
  >
    {{ active ? 'Active' : 'Inactive' }}
  </span>
</template>
