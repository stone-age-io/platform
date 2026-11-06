import { ref, computed } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { pb } from '@/utils/pb'

/**
 * Server-side search for a paginated list view (never filter only the page on
 * screen). The query is debounced. `onSearch` must reset to page one, and the
 * same filter must reach every usePagination call, pager buttons included, so
 * views build one `queryOptions` computed and pass it everywhere:
 *
 *   const { searchQuery, filter } = useServerSearch(
 *     ['name', 'description', 'code'],
 *     () => { page.value = 1; loadThings() },
 *   )
 *   const queryOptions = computed(() => ({ filter: filter.value, expand: 'type' }))
 *
 * Fields may go through relations (`type.name`) or name a json column
 * (`metadata`), which PocketBase matches as text.
 *
 * The input's placeholder names exactly these fields, no more and no fewer:
 * `Search by a, b, or c...`, without a noun.
 */
export function useServerSearch(fields: string[], onSearch: () => void, debounce = 300) {
  const searchQuery = ref('')

  const filter = computed(() => {
    const q = searchQuery.value.trim()
    if (!q) return undefined

    // Field names are developer constants; the user's query must go through
    // pb.filter's binding, never by hand, or it is a filter-injection bug.
    const expr = fields.map(f => `${f} ~ {:q}`).join(' || ')
    return pb.filter(expr, { q })
  })

  watchDebounced(searchQuery, onSearch, { debounce })

  return { searchQuery, filter }
}
