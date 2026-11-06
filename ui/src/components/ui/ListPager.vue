<script setup lang="ts">
/**
 * The footer under every paginated list.
 *
 * Presentational only: it emits prev/next and never loads. Server-paginated
 * views must pass the same `queryOptions` to `prevPage`/`nextPage` as the first
 * load (or page two is unfiltered and unsorted); client-paginated views just
 * move a ref.
 *
 * Visibility is the view's call: server-paginated views show it even at one
 * page (it is the result count), client-paginated ones hide it.
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
    <!-- `pager-touch` is on the join so all three buttons, label included, stay one height. -->
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
  `btn-sm` (32px) is too small for touch. Below `lg`, where ResponsiveList draws
  cards instead of a table (the app's touch-layout line), the buttons grow to 44px.
*/
@media (max-width: 1023px) {
  .pager-touch .btn {
    height: 44px;
    min-height: 44px;
    min-width: 44px;
  }
}
</style>
