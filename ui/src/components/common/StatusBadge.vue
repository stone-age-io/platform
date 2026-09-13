<!-- ui/src/components/common/StatusBadge.vue -->
<script setup lang="ts">
/**
 * The Active / Inactive chip for a record that carries an `active` flag.
 *
 * This was eight copies of `:class="item.active ? 'badge-success' : 'badge-error'"`
 * across five views -- and they had already drifted: six passed `badge-sm` and
 * two passed no size at all, which in daisyUI means `md`. An `md` badge is
 * taller than the `sm` ones beside it, so those two rows sat a couple of pixels
 * proud of every other row in the same table. That is most of what "the tables
 * look slightly ragged" actually was.
 *
 * Solid fill, deliberately, and that is the rule this component fixes in place:
 *
 *   solid   -- the record's own state. It is a fact about the row.
 *   outline -- advisory or derived, computed rather than stored (ExpiryBadge).
 *   ghost   -- neutral metadata that is not a state at all (a type, a count).
 *
 * Do not reach for `badge-lg` here. The shape comes from the theme
 * (`--rounded-badge: 0.375rem` squares these into chips, see tailwind.config.js)
 * and that block is shared verbatim with the access-control console.
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
