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
 * are all nil, so there is nothing to offer an action for. The row click opens
 * a detail dialog rather than navigating, because an entry is not a record --
 * the thing it describes may since have been deleted.
 */
import { ref, computed, onMounted, onUnmounted } from 'vue'
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

// `actor` and `resource_id` are plain ids, not relations, so there is nothing
// to expand -- search the snapshotted labels instead, which is also what the
// list renders.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['actor_label', 'resource_label', 'resource', 'action'],
  () => { page.value = 1; loadEntries() },
)

const sort = ref('-created')

// "Everything that happened to this one record", set from the detail dialog.
// It is a SECOND filter rather than a search term because `resource_id` is an
// id and the search box matches labels: searching for a label would also catch
// every other record that happens to share the name, which is the opposite of
// what someone tracing one device wants.
const focusedResource = ref<{ id: string; label: string } | null>(null)

const resourceFilter = computed(() =>
  focusedResource.value
    ? pb.filter('resource_id = {:id}', { id: focusedResource.value.id })
    : undefined,
)

// ONE computed, passed to every call including the pager. Passing a subset to
// nextPage() silently drops the filter on page two.
const queryOptions = computed(() => {
  const parts = [searchFilter.value, resourceFilter.value].filter(Boolean)
  return {
    // Parenthesised before joining: the search filter is itself a chain of ORs,
    // and `a ~ x || b ~ x && resource_id = y` is not the query anyone meant.
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
  // Widths are CSS values, not Tailwind classes: ResponsiveList binds them to
  // `style="width: …"`, so `w-44` here is a declaration the browser drops and
  // the column silently falls back to its share of the fixed layout.
  //
  // "What" is the one column left un-widthed, so it absorbs the slack -- it
  // holds record names, which is the only free-text content here. "Who" used to
  // share that job and should not have: it holds a person's name, which is
  // bounded, so on a wide screen it took a quarter of the table and left a gap
  // between a name and the badge beside it that the eye had to travel. 16rem
  // fits a long name plus the "platform" badge a superuser actor carries.
  //
  // "Kind" keeps 10rem despite holding one short badge: the longest noun in
  // RESOURCE_ROUTES below is "thing operation", and in a fixed-layout table a
  // badge that does not fit wraps INSIDE the badge rather than widening the
  // column.
  { key: 'created', label: 'When', sortable: '-created', width: '11rem' },
  { key: 'actor_label', label: 'Who', sortable: 'actor_label', width: '16rem' },
  { key: 'action', label: 'Did', sortable: 'action', width: '7rem' },
  { key: 'resource_label', label: 'What', sortable: 'resource_label' },
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

// ---------------------------------------------------------------------------
// Detail dialog
// ---------------------------------------------------------------------------

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
 * Where a feed entry's record lives, keyed by the product noun the server
 * snapshotted into `resource` (the values of `activityCollections` in
 * hooks/activity.go).
 *
 * Each entry carries the capability its route is gated on, NOT the one that got
 * the reader onto this screen. The feed is readable by every role in an
 * organization, but the three type collections have no detail view -- their
 * only route is the edit form behind `manageDefinitions` -- so a `viewer`
 * offered that link would be bounced to `/` by the router guard. Same rule as
 * the Delete buttons on the inventory lists: gate a control on the capability
 * that matches its rule, not the one that matches its screen.
 *
 * An unrecognised noun yields no link, which is the right failure: `resource`
 * is a snapshot, so rows written before a noun was renamed still say the old
 * word and must not resolve to a guess.
 */
const RESOURCE_ROUTES: Record<string, { path: (id: string) => string; capability: 'viewInventory' | 'manageDefinitions' }> = {
  'thing': { path: (id) => `/things/${id}`, capability: 'viewInventory' },
  'location': { path: (id) => `/locations/${id}`, capability: 'viewInventory' },
  'thing type': { path: (id) => `/things/types/${id}/edit`, capability: 'manageDefinitions' },
  'location type': { path: (id) => `/locations/types/${id}/edit`, capability: 'manageDefinitions' },
  'thing operation': { path: (id) => `/things/operations/${id}/edit`, capability: 'manageDefinitions' },
}

// The link, or null. Null for a deleted record (there is nothing to open), for
// a noun with no route, for an entry with no resource id, and for a reader
// without the capability that route needs.
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

function handleOrgChange() {
  searchQuery.value = ''
  focusedResource.value = null
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
        🔄 Refresh
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

    <!-- The one-record filter, set from the detail dialog. It is a separate
         control from the search box because it is a different question, and it
         is visible because a filter the reader cannot see is a feed that looks
         empty for no reason. -->
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

    <!-- Empty State. Guarded on the filters: with a server-side filter an empty
         result means "no match", not "nothing has happened here". -->
    <BaseCard v-else-if="entries.length === 0 && !searchQuery && !focusedResource">
      <div class="text-center py-12">
        <span class="text-6xl">🧭</span>
        <h3 class="text-xl font-bold mt-4">No activity yet</h3>
        <p class="text-base-content/70 mt-2">
          Changes to things, locations and their types will appear here.
        </p>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="entries.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
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
        <!-- Relative on top, exact underneath -- the same shape AuditLogView
             uses, and the reason this screen can be read beside a record's
             Created / Last updated without converting anything. -->
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

    <!-- Details Modal.
         The feed row is deliberately narrow -- when / who / did / what / kind --
         and everything the row cannot hold lives here: the exact timestamp, the
         actor and record IDS (which are what a support conversation actually
         needs), and the two things you can DO with an entry. -->
    <dialog id="activity_modal" class="modal">
      <div class="modal-box w-11/12 max-w-2xl">
        <h3 class="font-bold text-lg mb-4">Activity Details</h3>

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
                <!-- `superuser` is not a tenant role: a platform operator acting
                     through the PocketBase dashboard bypasses every API rule, so
                     an entry attributed to one is saying something different from
                     an entry attributed to a member. -->
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
              <!-- A SNAPSHOT of the name at the time of the write, not a live
                   join: hooks/activity.go stores the label precisely so the line
                   keeps reading correctly after a rename or a delete. Saying so
                   matters -- a reader who assumes it is live will think the feed
                   is stale when it is being accurate. -->
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
          <router-link v-if="recordLink" :to="recordLink" class="btn btn-primary">
            Open record
          </router-link>
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
