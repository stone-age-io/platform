<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ThingMapViz from '@/components/things/ThingMapViz.vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()
const authStore = useAuthStore()

// This screen is reachable by `viewer`, which writes nothing. Create/edit is
// manageInventory; delete is decommissionInventory, which members do NOT hold
// either -- the Delete button here was ungated and the server was rejecting it
// for every member who clicked it.
const canWrite = computed(() => authStore.can.manageInventory)
const canDelete = computed(() => authStore.can.decommissionInventory)

// View Mode
const viewMode = ref<'list' | 'map'>('list')

// Pagination
const {
  items: things,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<Thing>('things', 20)

// Search. This is a SERVER-side filter, not the client-side pass it used to be.
//
// The old version filtered the 20 records already on screen and admitted as much
// in the UI ("searching current page only"). Adding metadata to that would have
// been a false promise: the whole point of searching metadata is questions like
// "which devices were serviced before 2025", and an answer drawn from one page of
// results is not an answer. PocketBase filters a json column as text, so one `~`
// covers both keys and values without knowing any type's schema.
const searchQuery = ref('')

const searchFilter = computed(() => {
  const q = searchQuery.value.trim()
  if (!q) return undefined
  // pb.filter binds and escapes the parameter — never interpolate a user string
  // into a filter expression by hand.
  return pb.filter(
    'name ~ {:q} || description ~ {:q} || code ~ {:q} || metadata ~ {:q}' +
      ' || type.name ~ {:q} || location.name ~ {:q}',
    { q },
  )
})

// Every call into usePagination needs the same filter and expand, including the
// pager buttons — passing only `expand` there would silently drop the filter and
// page 2 would show unfiltered results.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  expand: 'type,location',
}))

const deleting = ref(false)

// Column configuration for responsive list
const columns: Column<Thing>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.type.name',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'expand.location.name',
    label: 'Location',
    mobileLabel: 'Location',
  },
  {
    key: 'code',
    label: 'Code',
    mobileLabel: 'Code',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

/**
 * Load things from API
 * Backend automatically filters by current organization via API rules
 */
async function loadThings() {
  // Backend handles org filtering via API rules; this adds only the search.
  await load(queryOptions.value)
}

// Typing a query re-queries the server, so debounce it and go back to page 1 —
// staying on page 3 of the old result set would show an empty list.
watchDebounced(
  searchQuery,
  () => {
    page.value = 1
    loadThings()
  },
  { debounce: 300 },
)

/**
 * Handle row/card click - navigate to detail view
 */
function handleRowClick(thing: Thing) {
  router.push(`/things/${thing.id}`)
}

/**
 * Handle delete
 */
async function handleDelete(thing: Thing) {
  const confirmed = await confirm({
    title: 'Delete Thing',
    message: `Are you sure you want to delete "${thing.name}"?`,
    details: 'This action cannot be undone.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('things').delete(thing.id)
    toast.success('Thing deleted')
    loadThings()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete thing')
  } finally {
    deleting.value = false
  }
}

/**
 * Handle organization change event
 */
function handleOrgChange() {
  // Reset search when org changes
  searchQuery.value = ''
  loadThings()
}

/**
 * Initialize on mount
 */
onMounted(() => {
  loadThings()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Controls -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <!-- Left Side: Title & Mobile Toggle -->
      <div class="w-full sm:w-auto">
        <div class="flex justify-between items-center">
          <h1 class="text-3xl font-bold">Things</h1>

          <!-- Mobile Toggle: Next to title -->
          <div class="join shadow-sm border border-base-300 sm:hidden">
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'list' }"
              @click="viewMode = 'list'"
            >
              📋
            </button>
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'map' }"
              @click="viewMode = 'map'"
            >
              🗺️
            </button>
          </div>
        </div>
        <p class="text-base-content/70 mt-1">
          Manage IoT devices and sensors
        </p>
      </div>

      <!-- Right Side: Desktop Toggle & New Button -->
      <div class="flex gap-3 w-full sm:w-auto">
        <!-- Desktop Toggle: Hidden on mobile -->
        <div class="hidden sm:inline-flex join shadow-sm border border-base-300">
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'list' }"
            @click="viewMode = 'list'"
          >
            📋 List
          </button>
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'map' }"
            @click="viewMode = 'map'"
          >
            🗺️ Map
          </button>
        </div>

        <router-link v-if="canWrite" to="/things/new" class="btn btn-primary w-full sm:w-auto">
          <span class="text-lg">+</span>
          <span>New Thing</span>
        </router-link>
      </div>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, code, type, location, or metadata..."
        class="input input-bordered w-full"
      />
      <label v-if="searchQuery && viewMode === 'list'" class="label">
        <span class="label-text-alt">
          {{ totalItems }} match{{ totalItems === 1 ? '' : 'es' }} across all things
        </span>
      </label>
    </div>

    <!-- LIST VIEW -->
    <template v-if="viewMode === 'list'">

    <!-- Loading State -->
    <div v-if="loading && things.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && things.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load things</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadThings" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State. Guarded on !searchQuery: with a server-side filter an empty
         result during a search means "no match", not "no inventory", and offering
         "create your first thing" to someone who has 400 of them is nonsense. -->
    <BaseCard v-else-if="things.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <span class="text-6xl">📦</span>
        <h3 class="text-xl font-bold mt-4">No things found</h3>
        <p class="text-base-content/70 mt-2">
          {{ canWrite ? 'Create your first thing to get started' : 'This organization has no things yet' }}
        </p>
        <router-link v-if="canWrite" to="/things/new" class="btn btn-primary mt-4">
          Create Thing
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="things.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching things</h3>
        <p class="text-base-content/70 mt-2">
          Try a different search term
        </p>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="things"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!--
          Deactivated state rides along with the name rather than taking its own
          column: the list is already five columns wide, and a deactivated Thing
          is the exception, not a value you scan down a column for.
        -->
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium flex items-center gap-2">
              <span :class="{ 'text-base-content/50': item.active === false }">
                {{ item.name || 'Unnamed' }}
              </span>
              <span v-if="item.active === false" class="badge badge-error badge-outline badge-sm">
                Deactivated
              </span>
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <!-- Custom mobile card for name (make it prominent) - no longer a link -->
        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base flex items-center gap-2">
              <span :class="{ 'text-base-content/50': item.active === false }">
                {{ item.name || 'Unnamed' }}
              </span>
              <span v-if="item.active === false" class="badge badge-error badge-outline badge-sm">
                Deactivated
              </span>
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Custom cell for type (badge) -->
        <template #cell-expand.type.name="{ item }">
          <span v-if="item.expand?.type" class="badge badge-ghost">
            {{ item.expand.type.name }}
          </span>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <template #card-expand.type.name="{ item }">
         <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">
           {{ item.expand.type.name }}
         </span>
         <span v-else>-</span>
        </template>

        <!-- Custom cell for code (mono font) -->
        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Actions - @click.stop is handled in ResponsiveList -->
        <template #actions="{ item }">
          <router-link
            v-if="canWrite"
            :to="`/things/${item.id}/edit`"
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            Edit
          </router-link>
          <button
            v-if="canDelete"
            @click="handleDelete(item)"
            class="btn btn-xs text-error flex-1 sm:flex-initial"
            :disabled="deleting"
          >
            Delete
          </button>
          <router-link
            v-if="!canWrite && !canDelete"
            :to="`/things/${item.id}`"
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            View
          </router-link>
        </template>
      </ResponsiveList>
      
      <!-- Pagination. Shown while searching too: the server paginates the matches,
           so page 2 of a search is a real page. The pager passes queryOptions
           rather than a bare expand — dropping the filter here is what would make
           page 2 revert to unfiltered results. -->
      <div class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ things.length }} of {{ totalItems }}
          {{ searchQuery ? 'matches' : 'things' }}
        </span>
        <div class="join">
          <button
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage(queryOptions)"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage(queryOptions)"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>

    </template>

    <!-- MAP VIEW -->
    <div v-else-if="viewMode === 'map'">
      <ThingMapViz :search-query="searchQuery" />
    </div>
  </div>
</template>
