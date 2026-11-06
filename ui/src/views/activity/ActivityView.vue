<script setup lang="ts">
import { HistoryIcon, RefreshCw, SearchX } from '@lucide/vue'
/**
 * The tenant activity feed. Not the audit log: `activity` is org-scoped and
 * stores no record values (hooks/activity.go decides what is in it), while
 * `audit_logs` is operator-only. Its write rules are all nil, so it is
 * read-only. A row opens a dialog, not the record, which may since be deleted.
 */
import { ref, computed, onMounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { useAuthStore } from '@/stores/auth'
import { pb } from '@/utils/pb'
import { formatDate, formatRelativeTime } from '@/utils/format'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import type { Activity } from '@/types/pocketbase'

const authStore = useAuthStore()

const {
  items: entries, page, totalPages, totalItems, loading, error,
  load, nextPage, prevPage,
} = usePagination<Activity>('activity', 25)

// `actor` and `resource_id` are plain ids, not relations; search the snapshotted labels.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['actor_label', 'resource_label', 'resource', 'action'],
  () => { page.value = 1; loadEntries() },
)

const sort = ref('-created')

// One record's history, set from the detail dialog. A separate filter on
// `resource_id`, not a search term: a label search also matches namesakes.
const focusedResource = ref<{ id: string; label: string } | null>(null)

const resourceFilter = computed(() =>
  focusedResource.value
    ? pb.filter('resource_id = {:id}', { id: focusedResource.value.id })
    : undefined,
)

// Pass this to every call, including the pager, or page two loses the filter.
const queryOptions = computed(() => {
  const parts = [searchFilter.value, resourceFilter.value].filter(Boolean)
  return {
    // Parenthesised: the search filter is itself a chain of ORs.
    filter: parts.length ? parts.map(p => `(${p})`).join(' && ') : undefined,
    sort: sort.value,
  }
})

async function loadEntries() {
  await load(queryOptions.value)
}

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadEntries()
}

const columns: Column<Activity>[] = [
  // Widths are CSS values, not Tailwind classes (ResponsiveList binds them to
  // `style`). "What" is left un-widthed to absorb the slack. "Kind" needs 10rem
  // for "thing operation": in a fixed layout a badge wraps inside itself.
  { key: 'created', label: 'When', sortable: '-created', width: '11rem' },
  { key: 'actor_label',
    cardWide: true, label: 'Who', sortable: 'actor_label', width: '16rem' },
  { key: 'action', label: 'Did', sortable: 'action', width: '7rem' },
  { key: 'resource_label',
    cardWide: true, label: 'What', sortable: 'resource_label' },
  { key: 'resource', label: 'Kind', sortable: 'resource', width: '10rem', class: 'hidden xl:table-cell' },
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

const selected = ref<Activity | null>(null)

function openDetails(entry: Activity) {
  selected.value = entry
  const modal = document.getElementById('activity_modal') as HTMLDialogElement | null
  modal?.showModal()
}

function closeDetails() {
  const modal = document.getElementById('activity_modal') as HTMLDialogElement | null
  modal?.close()
}

/**
 * Route per `resource` noun (the values of `activityCollections` in
 * hooks/activity.go). Each carries the capability its route is gated on, not
 * this screen's: the type collections only have an edit form behind
 * `manageDefinitions`, so a `viewer` would be bounced. An unknown noun (e.g.
 * one since renamed) yields no link.
 */
const RESOURCE_ROUTES: Record<string, { path: (id: string) => string; capability: 'viewInventory' | 'manageDefinitions' }> = {
  'thing': { path: (id) => `/things/${id}`, capability: 'viewInventory' },
  'location': { path: (id) => `/locations/${id}`, capability: 'viewInventory' },
  'thing type': { path: (id) => `/things/types/${id}/edit`, capability: 'manageDefinitions' },
  'location type': { path: (id) => `/locations/types/${id}/edit`, capability: 'manageDefinitions' },
  'thing operation': { path: (id) => `/things/operations/${id}/edit`, capability: 'manageDefinitions' },
}

const recordLink = computed(() => {
  const entry = selected.value
  if (!entry || !entry.resource_id || entry.action === 'deleted') return null
  const route = RESOURCE_ROUTES[entry.resource]
  if (!route || !authStore.can[route.capability]) return null
  return route.path(entry.resource_id)
})

function focusResource(entry: Activity) {
  if (!entry.resource_id) return
  focusedResource.value = {
    id: entry.resource_id,
    label: entry.resource_label || entry.resource_id,
  }
  page.value = 1
  closeDetails()
  loadEntries()
}

function clearFocus() {
  focusedResource.value = null
  page.value = 1
  loadEntries()
}

onMounted(() => {
  loadEntries()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Activity</h1>
        <p class="text-base-content/70 mt-1">
          Who changed what in this organization, and when
        </p>
      </div>

      <button @click="loadEntries" class="btn btn-ghost btn-sm" title="Refresh activity">
        <RefreshCw :size="16" class="inline-block shrink-0" /> Refresh
      </button>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by person, record, kind, or action..."
        class="input input-bordered w-full"
      />
      <label v-if="searchQuery" class="label">
        <span class="label-text-alt">
          {{ totalItems }} match{{ totalItems === 1 ? '' : 'es' }} across all activity
        </span>
      </label>
    </div>

    <!-- Shown so a hidden filter never makes the feed look empty. -->
    <div v-if="focusedResource" class="alert alert-info py-2">
      <span class="text-sm">
        Showing activity for <strong>{{ focusedResource.label }}</strong>
      </span>
      <button class="btn btn-xs btn-ghost" @click="clearFocus">Clear</button>
    </div>

    <!-- Loading State -->
    <div v-if="loading && entries.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && entries.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load activity</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadEntries" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State (with a filter set, empty means no match instead) -->
    <BaseCard v-else-if="entries.length === 0 && !searchQuery && !focusedResource">
      <div class="text-center py-12">
        <HistoryIcon :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No activity yet</h3>
        <p class="text-base-content/70 mt-2">
          Changes to things, locations and their types will appear here.
        </p>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="entries.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching activity</h3>
        <p class="text-base-content/70 mt-2">
          Try a different search term
        </p>
        <button v-if="searchQuery" @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
        <button v-if="focusedResource" @click="clearFocus" class="btn btn-ghost mt-4">
          Show all activity
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="entries"
        :columns="columns"
        :sort="sort"
        :loading="loading"
        @update:sort="onSort"
        @row-click="openDetails"
      >
        <template #cell-created="{ item }">
          <div class="flex flex-col">
            <span class="font-medium">{{ formatRelativeTime(item.created) }}</span>
            <span class="text-xs text-base-content/60">{{ formatDate(item.created, 'PP pp') }}</span>
          </div>
        </template>
        <template #card-created="{ item }">
          <span class="text-xs">{{ formatRelativeTime(item.created) }}</span>
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

        <template #cell-resource="{ item }">
          <span class="badge badge-ghost badge-sm">{{ item.resource }}</span>
        </template>
      </ResponsiveList>

      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="entries.length"
        :total="totalItems"
        :loading="loading"
        :noun="searchQuery || focusedResource ? 'matches' : 'entries'"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>

    <!-- Details Modal -->
    <dialog id="activity_modal" class="modal">
      <div class="modal-box w-11/12 max-w-2xl">
        <!-- "Open record" sits here, not in the footer: three footer buttons wrap
             into a staircase on every phone width. Full-size `btn` for a 44px+
             touch target. -->
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h3 class="font-bold text-lg">Activity Details</h3>
          <router-link v-if="recordLink" :to="recordLink" class="btn btn-primary">
            Open record ↗
          </router-link>
        </div>

        <div v-if="selected" class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div class="flex flex-col">
              <span class="font-bold opacity-70">When</span>
              <span>{{ formatDate(selected.created, 'PPpp') }}</span>
              <span class="text-xs opacity-60">{{ formatRelativeTime(selected.created) }}</span>
            </div>

            <div class="flex flex-col">
              <span class="font-bold opacity-70">Who</span>
              <span>
                {{ selected.actor_label || 'unknown' }}
                <!-- A superuser acts through the PocketBase dashboard and bypasses every API rule. -->
                <span v-if="selected.actor_type === 'superuser'" class="badge badge-ghost badge-xs ml-1">
                  platform
                </span>
              </span>
              <code v-if="selected.actor" class="text-xs opacity-60">{{ selected.actor }}</code>
              <span v-else class="text-xs opacity-60 italic">no account recorded</span>
            </div>

            <div class="flex flex-col">
              <span class="font-bold opacity-70">Action</span>
              <span>
                <span class="badge badge-sm" :class="actionBadge(selected.action)">
                  {{ selected.action }}
                </span>
              </span>
            </div>

            <div class="flex flex-col">
              <span class="font-bold opacity-70">Kind</span>
              <span>{{ selected.resource }}</span>
            </div>

            <div class="flex flex-col md:col-span-2">
              <span class="font-bold opacity-70">Record</span>
              <!-- A snapshot taken at write time (hooks/activity.go), not a live join. -->
              <span>{{ selected.resource_label || 'unnamed' }}</span>
              <code v-if="selected.resource_id" class="text-xs opacity-60">{{ selected.resource_id }}</code>
              <span class="text-xs opacity-60 italic">
                Name as it was when this happened; the record may have been renamed since.
              </span>
            </div>
          </div>

          <div v-if="selected.action === 'deleted'" class="alert alert-warning py-2 text-sm">
            This record was deleted, so there is nothing left to open.
          </div>
        </div>

        <div class="modal-action">
          <button
            v-if="selected?.resource_id"
            class="btn btn-ghost"
            @click="focusResource(selected)"
          >
            This record's history
          </button>
          <form method="dialog">
            <button class="btn">Close</button>
          </form>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button>close</button>
      </form>
    </dialog>
  </div>
</template>
