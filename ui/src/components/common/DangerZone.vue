<script setup lang="ts">
/**
 * The irreversible controls on a detail view, kept out of the header.
 *
 * Delete used to sit in the primary action row beside Edit, at the same size
 * and one gap away from Deactivate -- which on a Thing is the action that is
 * almost always the correct one. A button's position is part of what it says,
 * and "same row, same size as Edit" said the wrong thing.
 *
 * Deliberately not a BaseCard with an error class bolted on. A BaseCard is the
 * console's neutral container and 178 call sites read it that way; this is the
 * one place that has to look unlike the rest of the page, and giving it its own
 * component means it cannot drift into being used for anything else.
 *
 * ONE prop. The standing line and the top margin were props for about an hour,
 * until all twelve call sites turned out to pass the same two values -- which is
 * a constant wearing a prop's clothes, the same thing RecordPhoto was cut down
 * for. Give them back their props when a second value for either actually turns
 * up, not in anticipation of one.
 *
 * The zone carries the placement half of the problem. The commitment half --
 * making the click deliberate -- is ConfirmDialog's `requireText`, which the
 * caller passes when the delete cannot be undone by re-creating the record.
 * Neither substitutes for the other.
 */
defineProps<{
  title: string
}>()
</script>

<template>
  <div class="card bg-base-100 border border-error/40 shadow-sm mt-6">
    <div class="card-body gap-4">
      <div>
        <h2 class="card-title text-error text-base">{{ title }}</h2>
        <p class="text-sm text-base-content/70 mt-1">Deleting is permanent and cannot be undone.</p>
      </div>
      <!-- Right-aligned on a desktop width and full-bleed on a phone, matching
           every other action row in the console. -->
      <div class="flex flex-col sm:flex-row sm:justify-end gap-2">
        <slot />
      </div>
    </div>
  </div>
</template>
