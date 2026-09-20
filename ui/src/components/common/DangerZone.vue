<script setup lang="ts">
/**
 * The irreversible controls on a record's form, kept out of the header.
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
 * NO props, and the layout is why the last one went. It briefly took a
 * per-record heading -- "Delete this thing" above a button reading "Delete
 * Thing" -- which was survivable while the two were stacked at opposite corners
 * of a tall card, and became plainly silly once they sat on one line facing each
 * other. The heading is now the section label it should always have been, and
 * the button says which record and in which words; the confirm dialog names it
 * again and makes you type its code. Three sayings of one thing was two too
 * many.
 *
 * One row on a desktop width, stacked on a phone. It was a column at every
 * width, which left a heading in the top-left corner, a button in the
 * bottom-right, and a void between them the size of the card.
 *
 * The zone carries the placement half of the problem. The commitment half --
 * making the click deliberate -- is ConfirmDialog's `requireText`, which the
 * caller passes when the delete cannot be undone by re-creating the record.
 * Neither substitutes for the other.
 */
</script>

<template>
  <div class="card bg-base-100 border border-error/40 shadow-sm mt-6">
    <div class="card-body gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <h2 class="font-semibold text-error">Danger zone</h2>
        <p class="text-sm text-base-content/70 mt-1">Deleting is permanent and cannot be undone.</p>
      </div>
      <!-- shrink-0 so a long description cannot squeeze the button into two
           lines; full-bleed on a phone, matching every other action row. -->
      <div class="flex flex-col sm:flex-row gap-2 sm:shrink-0">
        <slot />
      </div>
    </div>
  </div>
</template>
