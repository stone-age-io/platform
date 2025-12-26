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
 *     load({ expand: 'type,location' })
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
   * @param options - PocketBase query options
   */
  async function load(options?: {
    filter?: string
    sort?: string
    expand?: string
  }) {
    loading.value = true
    error.value = null
    
    try {
      // Build options object, only including defined values
      const queryOptions: Record<string, any> = {}
      
      if (options?.filter) {
        queryOptions.filter = options.filter
      }
      
      if (options?.sort) {
        queryOptions.sort = options.sort
      }
      
      if (options?.expand) {
        queryOptions.expand = options.expand
      }
      
      const result = await pb.collection(collectionName).getList<T>(
        page.value, 
        perPage, 
        queryOptions
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
  async function nextPage(options?: {
    filter?: string
    sort?: string
    expand?: string
  }) {
    if (hasMore.value) {
      page.value++
      await load(options)
    }
  }
  
  /**
   * Go to previous page
   */
  async function prevPage(options?: {
    filter?: string
    sort?: string
    expand?: string
  }) {
    if (hasPrev.value) {
      page.value--
      await load(options)
    }
  }
  
  /**
   * Go to specific page
   */
  async function goToPage(
    pageNum: number, 
    options?: {
      filter?: string
      sort?: string
      expand?: string
    }
  ) {
    if (pageNum >= 1 && pageNum <= totalPages.value) {
      page.value = pageNum
      await load(options)
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
