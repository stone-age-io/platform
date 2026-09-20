<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { ThingTypeOperation } from '@/types/pocketbase'
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
} = usePagination<ThingTypeOperation>('thing_type_operations', 20)

// Search runs on the SERVER. The client-side pass this replaces filtered the
// twenty records already on screen, so a match on page three answered "No
// results found" -- which is not the same claim at all.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description', 'subject_suffix'],
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

const columns: Column<ThingTypeOperation>[] = [
  // 'auto', not omitted: omitting it means 28% for column 0 (see Column.width).
  // This is the only free-text column here -- name plus a clamped description --
  // so it takes the slack and every other column keeps exactly what it declares.
  { key: 'name', sortable: 'name', width: 'auto', label: 'Name', mobileLabel: 'Name' },
  { key: 'capability', sortable: 'capability', width: '7rem', label: 'Capability', mobileLabel: 'Capability' },
  { key: 'subject_suffix', sortable: 'subject_suffix', width: '11rem', label: 'Subject Suffix', mobileLabel: 'Suffix' },
  { key: 'created', sortable: '-created', width: '8rem', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() {
  await load(queryOptions.value)
}

function handleRowClick(item: ThingTypeOperation) {
  router.push(`/things/operations/${item.id}/edit`)
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
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Thing Type Operations</h1>
        <p class="text-base-content/70 mt-1">Shareable verbs linked from Thing Types</p>
      </div>
      <router-link to="/things/operations/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Operation</span>
      </router-link>
    </div>

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search by name, suffix, or description..." class="input input-bordered w-full" />
    </div>

    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load operations</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <BaseCard v-else-if="items.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <span class="text-6xl">📝</span>
        <h3 class="text-xl font-bold mt-4">No operations found</h3>
        <p class="text-base-content/70 mt-2">Create your first operation to start composing Thing Types</p>
        <router-link to="/things/operations/new" class="btn btn-primary mt-4">Create Operation</router-link>
      </div>
    </BaseCard>

    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching operations</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
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
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
          </div>
        </template>

        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">{{ item.name }}</div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #cell-capability="{ item }">
          <span class="badge badge-sm badge-outline">{{ item.capability }}</span>
        </template>

        <template #cell-subject_suffix="{ item }">
          <code class="bg-base-200 px-1 rounded text-xs">{{ item.subject_suffix }}</code>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/things/operations/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">Edit</router-link>
        </template>
      </ResponsiveList>

      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="items.length"
        :total="totalItems"
        noun="operations"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
