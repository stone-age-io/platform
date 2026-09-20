<!-- ui/src/components/ui/ListPager.vue -->
<script setup lang="ts">
/**
 * The footer under every paginated list.
 *
 * This was written out inline in nineteen views, twenty-odd identical lines each,
 * differing only in the noun at the end of the sentence.
 *
 * It stays presentational: it never changes a page itself, it asks. That is not
 * fussiness, it is the one thing the views do not share. The server-paginated
 * views have to hand the SAME `queryOptions` to `prevPage`/`nextPage` that the
 * initial load used -- pass only `expand` there and page two comes back
 * unfiltered and unsorted -- while the three client-paginated views (Locations,
 * KV Buckets, Streams) just move a ref. A pager that "helpfully" called a loader
 * itself would have to know which of those it was in.
 *
 * Visibility is the view's call too: the server-paginated views show the footer
 * even at one page (it doubles as the result count), the client-paginated ones
 * hide it. Keep that `v-if` where it is.
 */
interface Props {
  /** Current page, 1-based. */
  page: number
  totalPages: number
  /** Rows on this page, and rows in the whole result set. */
  shown: number
  total: number
  /** What is being counted: 'things', 'matches', 'leaf nodes'. */
  noun: string
  loading?: boolean
}

defineProps<Props>()
defineEmits<{ prev: []; next: [] }>()
</script>

<template>
  <div class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
    <span class="text-sm text-base-content/70 text-center sm:text-left">
      Showing {{ shown }} of {{ total }} {{ noun }}
    </span>
    <!--
      `pager-touch` sizes these for a thumb below `lg`; see the rule at the
      bottom of this file. The class goes on the join rather than the buttons
      because all three have to stay the same height or the join draws as three
      mismatched pills -- including the middle one, which is a label wearing a
      button's clothes.
    -->
    <div class="join pager-touch">
      <button
        class="join-item btn btn-sm"
        :disabled="page <= 1 || loading"
        aria-label="Previous page"
        @click="$emit('prev')"
      >
        «
      </button>
      <button class="join-item btn btn-sm no-animation cursor-default">
        {{ page }} / {{ totalPages }}
      </button>
      <button
        class="join-item btn btn-sm"
        :disabled="page >= totalPages || loading"
        aria-label="Next page"
        @click="$emit('next')"
      >
        »
      </button>
    </div>
  </div>
</template>

<style scoped>
/*
  `btn-sm` is 2rem -- 32px -- which is under both platform guidelines (iOS 44,
  Android 48), and this footer sits under every list in the app. It is the third
  place to want this rule and the first that cannot take the one in
  ResponsiveList: that one covers the mobile card, and this footer renders at
  every width.

  So the size is conditional, and `lg` is the right line rather than `sm`:
  that is where ResponsiveList stops drawing cards and starts drawing a table,
  which is this app's existing answer to "is this a touch layout". A tablet at
  800px gets cards and should get thumb-sized pager buttons with them.
*/
@media (max-width: 1023px) {
  .pager-touch .btn {
    height: 44px;
    min-height: 44px;
    min-width: 44px;
  }
}
</style>
