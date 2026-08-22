import { ref, computed } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { pb } from '@/utils/pb'

/**
 * Server-side search for a paginated list view.
 *
 * Every list view in this console used to filter `items.value` — the twenty
 * records already on screen — which makes a search box that quietly lies. Type a
 * name that lives on page three and you get "No results found", which is not
 * "not on this page" but reads exactly like it. Some views admitted it in small
 * grey text under the input; most did not, and the ones that did still put the
 * disclaimer where nobody looks after a zero-result answer.
 *
 * So the filter goes to the server. Three things have to be true for that to
 * work, and getting any one of them wrong is what made the client-side version
 * tempting in the first place:
 *
 *  1. The query is debounced, because every keystroke is now a request.
 *  2. Typing resets to page one. Staying on page three of the previous result
 *     set shows an empty list for a query that matches plenty.
 *  3. The SAME filter has to reach every call into usePagination, the pager
 *     buttons included — pass only `expand` to nextPage() and page two silently
 *     drops the filter.
 *
 * (1) and (2) are handled here. (3) is why views build one `queryOptions`
 * computed and pass it everywhere, rather than writing the options out per call.
 *
 * Usage:
 *
 *   const { searchQuery, filter } = useServerSearch(
 *     ['name', 'description', 'code'],
 *     () => { page.value = 1; loadThings() },
 *   )
 *   const queryOptions = computed(() => ({ filter: filter.value, expand: 'type' }))
 *
 * Field names may reach through relations (`type.name`, `user.email`) and may
 * name a json column (`metadata`), which PocketBase matches as text — so one `~`
 * covers both its keys and its values without knowing any schema.
 */
export function useServerSearch(fields: string[], onSearch: () => void, debounce = 300) {
  const searchQuery = ref('')

  const filter = computed(() => {
    const q = searchQuery.value.trim()
    if (!q) return undefined

    // The field names are developer-supplied constants and are interpolated
    // directly; the user's query is bound by pb.filter, which escapes it. Never
    // build the value side of a filter by hand — a quote in a search box would
    // otherwise be a filter-injection bug.
    const expr = fields.map(f => `${f} ~ {:q}`).join(' || ')
    return pb.filter(expr, { q })
  })

  watchDebounced(searchQuery, onSearch, { debounce })

  return { searchQuery, filter }
}
