<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { NatsAccountExport } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import ManagedRecordNotice from '@/components/common/ManagedRecordNotice.vue'
import { isManagedExport } from '@/utils/managedExports'

const router = useRouter()

const {
  items: exports,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsAccountExport>('nats_account_exports', 20)

// Search runs on the SERVER. The client-side pass this replaces filtered the
// twenty records already on screen, so a match on page three answered "No
// results found" -- which is not the same claim at all.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description', 'subject'],
  () => {
    page.value = 1
    loadExports()
  },
)

// The active sort, in PocketBase's own syntax ('name', '-created'). It rides in
// queryOptions rather than being passed at the call site, for the same reason the
// filter does: the pager buttons reuse that object, and a sort that is not in it
// is a sort that page two forgets.
const sort = ref('')

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
  loadExports()
}

const columns: Column<NatsAccountExport>[] = [
  {
    key: 'name',
    sortable: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'subject',
    sortable: 'subject',
    label: 'Subject',
    mobileLabel: 'Subject',
  },
  {
    key: 'type',
    sortable: 'type',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'created',
    sortable: '-created',
    width: '8rem',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

async function loadExports() {
  await load(queryOptions.value)
}

function handleRowClick(exp: NatsAccountExport) {
  router.push(`/nats/exports/${exp.id}`)
}

function handleOrgChange() {
  searchQuery.value = ''
  loadExports()
}

onMounted(() => {
  loadExports()
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
        <h1 class="text-3xl font-bold">NATS Exports</h1>
        <p class="text-base-content/70 mt-1">
          Share subjects with other accounts
        </p>
      </div>
      <router-link to="/nats/exports/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Export</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, subject, or description..."
        class="input input-bordered w-full"
      />
    </div>

    <!-- Loading State -->
    <div v-if="loading && exports.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && exports.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load exports</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadExports" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="exports.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <span class="text-6xl">📤</span>
        <h3 class="text-xl font-bold mt-4">No exports found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first export to share subjects with other accounts
        </p>
        <router-link to="/nats/exports/new" class="btn btn-primary mt-4">
          Create Export
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="exports.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching exports</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :sort="sort"
        @update:sort="onSort"
        :items="exports"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium flex items-center gap-2">
              {{ item.name }}
              <ManagedRecordNotice v-if="isManagedExport(item.name)" kind="export" variant="inline" />
            </div>
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base flex items-center gap-2">
              {{ item.name }}
              <ManagedRecordNotice v-if="isManagedExport(item.name)" kind="export" variant="inline" />
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #cell-subject="{ item }">
          <code class="text-xs">{{ item.subject }}</code>
        </template>

        <template #cell-type="{ item }">
          <span
            class="badge badge-sm"
            :class="item.type === 'stream' ? 'badge-info' : 'badge-success'"
          >
            {{ item.type }}
          </span>
        </template>

        <!-- Actions -->
        <template #actions="{ item }">
          <router-link
            v-if="isManagedExport(item.name)"
            :to="`/nats/exports/${item.id}`"
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            View
          </router-link>
          <template v-else>
            <router-link
              :to="`/nats/exports/${item.id}/edit`"
              class="btn btn-xs flex-1 sm:flex-initial"
            >
              Edit
            </router-link>
          </template>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="exports.length"
        :total="totalItems"
        noun="exports"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
