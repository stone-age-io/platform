<script setup lang="ts">
/**
 * The tenant activity feed: who changed what in the console, and when.
 *
 * NOT the audit log. AuditLogView reads `audit_logs`, which is operator-only,
 * carries full before/after record snapshots, and has no organization column.
 * This reads `activity`, which is org-scoped, stores no record values at all,
 * and only describes the five collections every role in an organization can
 * already read. The two look similar and are different products -- see
 * hooks/activity.go for the invariant that decides what is in here.
 *
 * Read-only by construction: the collection's create, update and delete rules
 * are all nil, so there is nothing to offer an action for.
 */
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate, formatRelativeTime } from '@/utils/format'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import type { Activity } from '@/types/pocketbase'

const {
  items: entries, page, totalPages, totalItems, loading, error,
  load, nextPage, prevPage,
} = usePagination<Activity>('activity', 25)

// `actor` and `resource_id` are plain ids, not relations, so there is nothing
// to expand -- search the snapshotted labels instead, which is also what the
// list renders.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['actor_label', 'resource_label', 'resource', 'action'],
  () => { page.value = 1; loadEntries() },
)

const sort = ref('-created')

// ONE computed, passed to every call including the pager. Passing a subset to
// nextPage() silently drops the filter on page two.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
}))

async function loadEntries() {
  await load(queryOptions.value)
}

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadEntries()
}

const columns: Column<Activity>[] = [
  { key: 'created', label: 'When', sortable: '-created', width: 'w-44' },
  { key: 'actor_label', label: 'Who', sortable: 'actor_label' },
  { key: 'action', label: 'Did', sortable: 'action', width: 'w-32' },
  { key: 'resource_label', label: 'What', sortable: 'resource_label' },
  { key: 'resource', label: 'Kind', sortable: 'resource', width: 'w-40', class: 'hidden xl:table-cell' },
]

function actionBadge(action: string): string {
  switch (action) {
    case 'created':
    case 'provisioned':
      return 'badge-success'
    case 'updated':
      return 'badge-warning'
    case 'deleted':
      return 'badge-error'
    default:
      return 'badge-ghost'
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  page.value = 1
  loadEntries()
}

onMounted(() => {
  loadEntries()
  window.addEventListener('organization-changed', handleOrgChange)
})
onUnmounted(() => window.removeEventListener('organization-changed', handleOrgChange))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">Activity</h1>
      <p class="text-sm opacity-70">
        Who changed what in this organization, and when.
      </p>
    </div>

    <div class="form-control max-w-md">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by person, record or action…"
        class="input input-bordered input-sm"
      />
    </div>

    <div v-if="loading && !entries.length" class="flex justify-center py-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error">
      <div class="text-center py-12">
        <p class="text-error mb-4">{{ error }}</p>
        <button class="btn btn-sm" @click="loadEntries">Retry</button>
      </div>
    </BaseCard>

    <BaseCard v-else-if="!entries.length && !searchQuery">
      <div class="text-center py-12">
        <div class="text-4xl mb-2">🧭</div>
        <p class="font-medium">No activity yet</p>
        <p class="text-sm opacity-70">
          Changes to things, locations and their types will appear here.
        </p>
      </div>
    </BaseCard>

    <BaseCard v-else-if="!entries.length">
      <div class="text-center py-12">
        <p class="font-medium mb-2">Nothing matches that search</p>
        <button class="btn btn-sm" @click="searchQuery = ''">Clear search</button>
      </div>
    </BaseCard>

    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="entries"
        :columns="columns"
        :sort="sort"
        @update:sort="onSort"
      >
        <template #cell-created="{ item }">
          <span :title="formatDate(item.created, 'PP pp')">
            {{ formatRelativeTime(item.created) }}
          </span>
        </template>

        <template #cell-actor_label="{ item }">
          <span v-if="item.actor_label">{{ item.actor_label }}</span>
          <span v-else class="opacity-50 italic">unknown</span>
          <span v-if="item.actor_type === 'superuser'" class="badge badge-ghost badge-xs ml-2">
            platform
          </span>
        </template>

        <template #cell-action="{ item }">
          <span class="badge badge-sm" :class="actionBadge(item.action)">
            {{ item.action }}
          </span>
        </template>

        <template #cell-resource_label="{ item }">
          {{ item.resource_label || item.resource_id }}
        </template>
      </ResponsiveList>

      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="entries.length"
        :total="totalItems"
        :loading="loading"
        noun="entries"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
