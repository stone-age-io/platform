<!-- ui/src/components/common/StickyFormActions.vue -->
<script setup lang="ts">
// A form's Cancel / Save row, pinned to the bottom of the screen so Save is
// reachable without scrolling past every card first. The Thing form was 1,769px
// tall on a desktop and 3,184px on a phone with its only Save at the very end.
//
// Three placement facts, all load-bearing, all measured:
//
//   - It must be a child of the <form>, not of any card. DaisyUI's `.card` sets
//     `overflow: hidden`, which makes the card the containing block and stops
//     `position: sticky` dead (MetadataCard says the same about its own header).
//
//   - The DOCUMENT scrolls, not MainLayout's <main>. The drawer does not bound
//     `.drawer-content` to the viewport, so <main> grows with its content and
//     never scrolls -- but its `overflow-y-auto` still makes it a scroll
//     container, and a sticky element binds to its nearest one. Stuck to a box
//     that never moves, the bar sat at the end of the form exactly as before.
//     The rule below drops that overflow on pages that carry this bar and
//     nowhere else. Making <main> the real scroller would also work, but it
//     changes scrolling on every page (and how a phone hides its address bar),
//     which is a much larger decision than one bar. Where `:has()` is
//     unsupported the bar simply sits at the bottom of the form, as it used to.
//
//   - The negative margins cancel <main>'s `p-4 lg:p-6` so the bar spans its
//     full width, and the background is <main>'s own so cards slide under it
//     rather than show through.
//
// One row at every width. The forms' old action rows stacked their buttons
// below `sm`, which is harmless at the end of a page and 129px of a 812px phone
// screen once it is pinned there. Callers give their buttons `flex-1 sm:flex-none`
// so the two split the row on a phone, Cancel first.
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
