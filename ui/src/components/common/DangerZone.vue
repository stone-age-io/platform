<script setup lang="ts">
/**
 * The irreversible controls on a record's form, kept out of the header action
 * row (where Delete sat beside Edit and Deactivate, usually the right choice).
 *
 * Deliberately not a BaseCard with an error class: BaseCard is the neutral
 * container, and a separate component cannot drift into other uses.
 *
 * No props: the slotted button names the record, and the confirm dialog names
 * it again. Making the click deliberate is ConfirmDialog's `requireText`, which
 * the caller passes; this component only handles placement.
 */
</script>

<template>
  <div class="card bg-base-100 border border-error/40 shadow-sm mt-6">
    <div class="card-body gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <h2 class="font-semibold text-error">Danger zone</h2>
        <p class="text-sm text-base-content/70 mt-1">Deleting is permanent and cannot be undone.</p>
      </div>
      <!-- shrink-0 so the description cannot wrap the button. -->
      <div class="flex flex-col sm:flex-row gap-2 sm:shrink-0">
        <slot />
      </div>
    </div>
  </div>
</template>
