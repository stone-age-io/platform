<script setup lang="ts">
// A form's Cancel / Save row, pinned to the bottom of the screen.
//
// Placement constraints:
//   - Put it in the <form>, not in a card: DaisyUI's `.card` sets
//     `overflow: hidden`, which breaks `position: sticky`.
//   - The document scrolls, not MainLayout's <main>, but <main>'s
//     `overflow-y-auto` still makes it the sticky container. The rule below
//     drops that overflow only on pages with this bar. Without `:has()` the bar
//     just sits at the end of the form.
//   - The negative margins cancel <main>'s `p-4 lg:p-6`.
//
// One row at every width; callers give buttons `flex-1 sm:flex-none`, Cancel first.
</script>

<template>
  <div
    class="sticky-form-actions sticky bottom-0 z-10 -mx-4 lg:-mx-6 px-4 lg:px-6 py-3
           bg-base-200/95 backdrop-blur border-t border-base-300
           flex justify-end gap-2 sm:gap-4"
  >
    <slot />
  </div>
</template>

<style>
/* Unscoped on purpose: it targets MainLayout's <main>. See above. */
main:has(.sticky-form-actions) {
  overflow: visible;
}
</style>
