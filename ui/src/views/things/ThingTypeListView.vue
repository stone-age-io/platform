<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { ThingType } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

const router = useRouter()

// Pagination
const {
  items,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<ThingType>('thing_types', 20)

// Search runs on the SERVER. The client-side pass this replaces filtered the
// twenty records already on screen, so a match on page three answered "No
// results found" -- which is not the same claim at all.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'code', 'description'],
  () => {
    page.value = 1
    loadData()
  },
)

// The active sort, in PocketBase's own syntax ('name', '-created'). It rides in
// queryOptions rather than being passed at the call site, for the same reason the
// filter does: the pager buttons reuse that object, and a sort that is not in it
// is a sort that page two forgets.
const sort = ref('name')

// One options object, passed to EVERY call into usePagination, the pager
// buttons included -- passing only expand/sort there drops the filter and
// page two comes back unfiltered.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1 // a new order makes the old page number meaningless
  loadData()
}

// Description is the column that absorbs the leftover width (see Column.width):
// it is the only free-text field here, and it is already one of the three fields
// the search above matches on -- so until it was shown, searching a description
// returned rows with nothing in them to explain the match. Without it the slack
// landed on Operations, which is a one- or two-digit badge, and Code wrapped at
// 7rem while 746px sat empty beside it.
//
// Same shape and same order as LocationTypeListView -- the two type lists are the
// same screen for different nouns and should not need reading twice.
const columns: Column<ThingType>[] = [
  { key: 'name', sortable: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', sortable: 'code', width: '11rem', label: 'Code', mobileLabel: 'Code' },
  { key: 'description', sortable: 'description', label: 'Description', mobileLabel: 'Desc', class: 'hidden md:table-cell' },
  { key: 'operations', width: '7rem', label: 'Operations', mobileLabel: 'Ops' },
  { key: 'created', sortable: '-created', width: '8rem', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() {
  await load(queryOptions.value)
}

function handleRowClick(item: ThingType) {
  router.push(`/things/types/${item.id}/edit`)
}

function handleOrgChange() {
  searchQuery.value = ''
  loadData()
}

onMounted(() => {
  loadData()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Thing Types</h1>
        <p class="text-base-content/70 mt-1">Classify your devices</p>
      </div>
      <router-link to="/things/types/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Type</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search thing types by name, code, or description..." class="input input-bordered w-full" />
    </div>

    <!-- Loading State -->
    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load thing types</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="items.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <span class="text-6xl">🏷️</span>
        <h3 class="text-xl font-bold mt-4">No thing types found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first thing type to classify devices
        </p>
        <router-link to="/things/types/new" class="btn btn-primary mt-4">
          Create Thing Type
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching thing types</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="items"
        :columns="columns"
        :loading="loading"
        :sort="sort"
        @update:sort="onSort"
        @row-click="handleRowClick"
      >
        <template #cell-code="{ item }">
          <code v-if="item.code" class="bg-base-200 px-1 rounded text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">—</span>
        </template>

        <template #cell-description="{ item }">
          <div v-if="item.description" :title="item.description" class="text-sm text-base-content/70 line-clamp-1">{{ item.description }}</div>
          <span v-else class="text-base-content/40">—</span>
        </template>

        <template #cell-operations="{ item }">
          <span v-if="item.operations?.length" class="badge badge-sm">
            {{ item.operations.length }}
          </span>
          <span v-else class="text-base-content/40 text-xs">—</span>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/things/types/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">Edit</router-link>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="items.length"
        :total="totalItems"
        noun="thing types"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
