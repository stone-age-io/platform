<script setup lang="ts">
import { RefreshCw, ScrollText, SearchX } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate, formatRelativeTime, formatConstant } from '@/utils/format'
import type { AuditLog } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'

const {
  items: logs,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<AuditLog>('audit_logs', 20)

// Server-side, so a match beyond the current page is found.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['collection_name', 'record_id', 'event_type', 'request_ip', 'user.name', 'user.email'],
  () => {
    page.value = 1
    loadLogs()
  },
)

const sort = ref('-created')

// Pass this to every usePagination call, the pager included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  expand: 'user',
  sort: sort.value,
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadLogs()
}
const selectedLog = ref<AuditLog | null>(null)

const columns: Column<AuditLog>[] = [
  {
    key: 'created',
    sortable: '-created',
    label: 'Time',
    mobileLabel: 'Time',
  },
  {
    key: 'expand.user.name',
    cardWide: true,
    sortable: 'user.name',
    label: 'User',
    mobileLabel: 'User',
  },
  {
    // Wide: the next column is wide too, so this costs no extra card row, and
    // 'EVENT TYPE:' plus its value does not fit in half.
    key: 'event_type',
    cardWide: true,
    sortable: 'event_type',
    label: 'Event Type',
    mobileLabel: 'Event Type',
  },
  {
    key: 'collection_name',
    cardWide: true,
    sortable: 'collection_name',
    label: 'Collection',
    mobileLabel: 'Collection',
  },
  {
    key: 'request_ip',
    sortable: 'request_ip',
    label: 'IP Address',
    mobileLabel: 'IP',
    class: 'hidden xl:table-cell',
  }
]

async function loadLogs() {
  await load(queryOptions.value)
}

function openDetails(log: AuditLog) {
  selectedLog.value = log
  const modal = document.getElementById('audit_modal') as HTMLDialogElement
  if (modal) modal.showModal()
}

function handleRowClick(log: AuditLog) {
  openDetails(log)
}

function getEventBadgeClass(eventType: string) {
  if (eventType.includes('create')) return 'badge-success'
  if (eventType.includes('update')) return 'badge-warning'
  if (eventType.includes('delete')) return 'badge-error'
  if (eventType.includes('auth')) return 'badge-info'
  return 'badge-ghost'
}

onMounted(() => {
  loadLogs()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Audit Logs</h1>
        <p class="text-base-content/70 mt-1">
          System activity and change history
        </p>
      </div>

      <button @click="loadLogs" class="btn btn-ghost btn-sm" title="Refresh Logs">
        <RefreshCw :size="16" class="inline-block shrink-0" /> Refresh
      </button>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by user, collection, record ID, event, or IP..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && logs.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && logs.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load audit logs</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadLogs" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="logs.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <ScrollText :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No audit logs found</h3>
        <p class="text-base-content/70 mt-2">
          Activity will appear here when users interact with the system.
        </p>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="logs.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching logs</h3>
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
        :items="logs" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-created="{ item }">
          <div class="flex flex-col">
            <span class="font-medium">{{ formatRelativeTime(item.created) }}</span>
            <span class="text-xs text-base-content/60">{{ formatDate(item.created, 'PP pp') }}</span>
          </div>
        </template>
        <template #card-created="{ item }">
          <div class="text-sm text-base-content/70 mb-1">
            {{ formatRelativeTime(item.created) }}
          </div>
        </template>

        <template #cell-expand.user.name="{ item }">
          <div class="flex items-center gap-2">
            <UserAvatar :user="item.expand?.user" :size="32" fallback-initial="S" />
            <div class="flex flex-col">
              <span class="text-sm font-medium">
                {{ item.expand?.user?.name || item.expand?.user?.email || 'System' }}
              </span>
            </div>
          </div>
        </template>
        <template #card-expand.user.name="{ item }">
          <div class="flex items-center gap-2 font-medium">
            <UserAvatar :user="item.expand?.user" :size="24" fallback-initial="S" />
            {{ item.expand?.user?.name || item.expand?.user?.email || 'System' }}
          </div>
        </template>

        <template #cell-event_type="{ item }">
          <span class="badge badge-sm" :class="getEventBadgeClass(item.event_type)">
            {{ formatConstant(item.event_type) }}
          </span>
        </template>

        <template #cell-collection_name="{ item }">
          <code class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.collection_name }}</code>
        </template>

        <template #actions="{ item }">
          <button @click.stop="openDetails(item)" class="btn btn-ghost btn-xs">
            Details
          </button>
        </template>
      </ResponsiveList>

      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="logs.length"
        :total="totalItems"
        noun="entries"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>

    <!-- Details Modal -->
    <dialog id="audit_modal" class="modal">
      <div class="modal-box w-11/12 max-w-4xl">
        <h3 class="font-bold text-lg mb-4">Log Details</h3>
        
        <div v-if="selectedLog" class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div class="flex flex-col">
              <span class="font-bold opacity-70">Timestamp</span>
              <span>{{ formatDate(selectedLog.created) }}</span>
            </div>
            <div class="flex flex-col">
              <span class="font-bold opacity-70">User</span>
              <span>{{ selectedLog.expand?.user?.email || 'System' }} ({{ selectedLog.user || 'N/A' }})</span>
            </div>
            <div class="flex flex-col">
              <span class="font-bold opacity-70">Action</span>
              <span class="badge badge-sm" :class="getEventBadgeClass(selectedLog.event_type)">
                {{ formatConstant(selectedLog.event_type) }}
              </span>
            </div>
            <div class="flex flex-col">
              <span class="font-bold opacity-70">Target</span>
              <span>{{ selectedLog.collection_name }} / {{ selectedLog.record_id }}</span>
            </div>
            <div class="flex flex-col md:col-span-2">
              <span class="font-bold opacity-70">Request Info</span>
              <code class="text-xs bg-base-200 p-2 rounded block mt-1">
                {{ selectedLog.request_method }} {{ selectedLog.request_url }} ({{ selectedLog.request_ip }})
              </code>
            </div>
          </div>

          <div class="divider"></div>

          <!-- Changes Diff -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <h4 class="font-bold text-sm mb-2 text-base-content/70">Before Changes</h4>
              <div v-if="selectedLog.before_changes && Object.keys(selectedLog.before_changes).length" 
                   class="mockup-code bg-base-300 text-xs min-h-[100px]">
                <pre><code>{{ JSON.stringify(selectedLog.before_changes, null, 2) }}</code></pre>
              </div>
              <div v-else class="text-sm italic opacity-50 p-4 bg-base-200 rounded">
                No previous state
              </div>
            </div>

            <div>
              <h4 class="font-bold text-sm mb-2 text-base-content/70">After Changes</h4>
              <div v-if="selectedLog.after_changes && Object.keys(selectedLog.after_changes).length" 
                   class="mockup-code bg-base-300 text-xs min-h-[100px]">
                <pre><code>{{ JSON.stringify(selectedLog.after_changes, null, 2) }}</code></pre>
              </div>
              <div v-else class="text-sm italic opacity-50 p-4 bg-base-200 rounded">
                No new state
              </div>
            </div>
          </div>
        </div>

        <div class="modal-action">
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
