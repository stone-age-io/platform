import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { BaseRecord } from '@/types/pocketbase'

/**
 * Pagination Composable
 * 
 * Provides simple pagination for PocketBase collections.
 * Handles loading state, errors, and page navigation.
 * 
 * Usage:
 *   const { items, page, totalPages, loading, load, nextPage, prevPage } = 
 *     usePagination<Thing>('things', 20)
 *   
 *   onMounted(() => {
 *     load('organization = "abc123"', '-created')
 *   })
 */
export function usePagination<T extends BaseRecord>(
  collectionName: string, 
  perPage = 20
) {
  // ============================================================================
  // STATE
  // ============================================================================
  
  const items = ref<T[]>([])
  const page = ref(1)
  const totalPages = ref(1)
  const totalItems = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)
  
  // ============================================================================
  // COMPUTED
  // ============================================================================
  
  const hasMore = computed(() => page.value < totalPages.value)
  const hasPrev = computed(() => page.value > 1)
  
  // ============================================================================
  // ACTIONS
  // ============================================================================
  
  /**
   * Load a page of data
   * @param filter - PocketBase filter string
   * @param sort - PocketBase sort string
   * @param expand - Relations to expand
   */
  async function load(filter?: string, sort?: string, expand?: string) {
    loading.value = true
    error.value = null
    
    try {
      const result = await pb.collection(collectionName).getList<T>(
        page.value, 
        perPage, 
        {
          filter,
          sort,
          expand,
        }
      )
      
      items.value = result.items
      totalPages.value = result.totalPages
      totalItems.value = result.totalItems
    } catch (err: any) {
      error.value = err.message
      console.error('Pagination error:', err)
    } finally {
      loading.value = false
    }
  }
  
  /**
   * Go to next page
   */
  async function nextPage(filter?: string, sort?: string, expand?: string) {
    if (hasMore.value) {
      page.value++
      await load(filter, sort, expand)
    }
  }
  
  /**
   * Go to previous page
   */
  async function prevPage(filter?: string, sort?: string, expand?: string) {
    if (hasPrev.value) {
      page.value--
      await load(filter, sort, expand)
    }
  }
  
  /**
   * Go to specific page
   */
  async function goToPage(
    pageNum: number, 
    filter?: string, 
    sort?: string, 
    expand?: string
  ) {
    if (pageNum >= 1 && pageNum <= totalPages.value) {
      page.value = pageNum
      await load(filter, sort, expand)
    }
  }
  
  /**
   * Reset pagination state
   */
  function reset() {
    page.value = 1
    items.value = []
    totalPages.value = 1
    totalItems.value = 0
    error.value = null
  }
  
  // ============================================================================
  // RETURN PUBLIC API
  // ============================================================================
  
  return {
    items,
    page,
    totalPages,
    totalItems,
    loading,
    error,
    hasMore,
    hasPrev,
    load,
    nextPage,
    prevPage,
    goToPage,
    reset,
  }
}
