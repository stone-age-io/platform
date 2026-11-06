<script setup lang="ts">
import { Building2, SearchX } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { Organization, User } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'

interface OrganizationWithExpand extends Organization {
  expand?: {
    owner?: User
  }
}

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
} = usePagination<OrganizationWithExpand>('organizations', 20)

// Server-side, so a match beyond the current page is found.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description', 'owner.email', 'owner.name'],
  () => {
    page.value = 1
    loadData()
  },
)

const sort = ref('name')

// Pass this to every usePagination call, the pager included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
  expand: 'owner',
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadData()
}

const columns: Column<OrganizationWithExpand>[] = [
  { key: 'name', sortable: 'name', width: 'auto', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.owner.email',
    cardWide: true, sortable: 'owner.email', width: '20rem', label: 'Owner', mobileLabel: 'Owner' },
  { key: 'active', width: '7rem', label: 'Status', mobileLabel: 'Status' },
  { key: 'created', sortable: '-created', width: '8rem', label: 'Created', mobileLabel: 'Created', format: (v) => formatDate(v, 'PP') },
]

async function loadData() {
  await load(queryOptions.value)
}

function handleRowClick(item: OrganizationWithExpand) {
  router.push(`/organizations/${item.id}`)
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Organizations</h1>
        <p class="text-base-content/70 mt-1">Manage platform organizations</p>
      </div>
      <router-link to="/organizations/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Organization</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search by name, description, or owner..." class="input input-bordered w-full" />
    </div>

    <!-- Loading State -->
    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load organizations</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="items.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Building2 :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No organizations found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first organization to get started
        </p>
        <router-link to="/organizations/new" class="btn btn-primary mt-4">
          Create Organization
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching organizations</h3>
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
        <!-- Description shown so a search match on it is explicable. -->
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

        <template #cell-expand.owner.email="{ item }">
          <div class="text-sm truncate" :title="item.expand?.owner?.email">
            {{ item.expand?.owner?.email || '—' }}
          </div>
        </template>

        <template #cell-active="{ item }">
          <StatusBadge :active="item.active" />
        </template>

        <template #actions="{ item }">
          <router-link :to="`/organizations/${item.id}/edit`" class="btn btn-xs">Edit</router-link>
        </template>
      </ResponsiveList>

      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="items.length"
        :total="totalItems"
        noun="organizations"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
