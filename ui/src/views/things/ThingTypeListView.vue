<script setup lang="ts">
import { SearchX, Tag } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
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

// Search runs on the server, across all records, not the page on screen.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'code', 'description'],
  () => {
    page.value = 1
    loadData()
  },
)

// PocketBase sort syntax ('name', '-created').
const sort = ref('name')

// Pass this to EVERY usePagination call, the pager buttons included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1 // a new order makes the old page number meaningless
  loadData()
}

// Description is drawn under the name (see Column.width), so the name column
// takes the leftover width. Keep in step with LocationTypeListView.
const columns: Column<ThingType>[] = [
  { key: 'name', sortable: 'name', width: 'auto', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', sortable: 'code', width: '11rem', label: 'Code', mobileLabel: 'Code' },
  { key: 'operations', width: '7rem', label: 'Operations', mobileLabel: 'Ops' },
  { key: 'created', sortable: '-created', width: '8rem', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() {
  await load(queryOptions.value)
}

function handleRowClick(item: ThingType) {
  router.push(`/things/types/${item.id}/edit`)
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="space-y-6">
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

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search by name, code, or description..." class="input input-bordered w-full" />
    </div>

    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load thing types</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <BaseCard v-else-if="items.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Tag :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No thing types found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first thing type to classify devices
        </p>
        <router-link to="/things/types/new" class="btn btn-primary mt-4">
          Create Thing Type
        </router-link>
      </div>
    </BaseCard>

    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching thing types</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="items"
        :columns="columns"
        :loading="loading"
        :sort="sort"
        @update:sort="onSort"
        @row-click="handleRowClick"
      >
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">{{ item.name }}</div>
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">{{ item.name }}</div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #cell-code="{ item }">
          <code v-if="item.code" class="bg-base-200 px-1 rounded text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">—</span>
        </template>

        <template #cell-operations="{ item }">
          <span v-if="item.operations?.length" class="badge badge-sm">
            {{ item.operations.length }}
          </span>
          <span v-else class="text-base-content/40 text-xs">—</span>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/things/types/${item.id}/edit`" class="btn btn-xs">Edit</router-link>
        </template>
      </ResponsiveList>

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
