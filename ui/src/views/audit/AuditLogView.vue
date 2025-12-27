<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { usePagination } from '@/composables/usePagination'
import { formatDate, formatRelativeTime, formatConstant } from '@/utils/format'
import type { AuditLog } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

// Pagination
const {
  items: logs,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<AuditLog>('audit_logs', 20)

// Search & Filter
const searchQuery = ref('')
const selectedLog = ref<AuditLog | null>(null)

/**
 * Filtered logs based on client-side search
 * Searches in: user name/email, collection name, record id
 */
const filteredLogs = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  
  if (!query) return logs.value
  
  return logs.value.filter(log => {
    // Search User
    const userName = log.expand?.user?.name?.toLowerCase() || ''
    const userEmail = log.expand?.user?.email?.toLowerCase() || ''
    if (userName.includes(query) || userEmail.includes(query)) return true
    
    // Search Collection
    if (log.collection_name?.toLowerCase().includes(query)) return true
    
    // Search Record ID
    if (log.record_id?.toLowerCase().includes(query)) return true
    
    // Search Action
    if (log.event_type?.toLowerCase().includes(query)) return true
    
    return false
  })
})

// Column Configuration
const columns: Column<AuditLog>[] = [
  {
    key: 'created',
    label: 'Time',
    mobileLabel: 'Time',
    // Custom format logic handled in slot
  },
  {
    key: 'expand.user.name',
    label: 'User',
    mobileLabel: 'User',
  },
  {
    key: 'event_type',
    label: 'Event Type',
    mobileLabel: 'Event Type',
  },
  {
    key: 'collection_name',
    label: 'Collection',
    mobileLabel: 'Collection',
  },
  {
    key: 'request_ip',
    label: 'IP Address',
    mobileLabel: 'IP',
    class: 'hidden xl:table-cell', // Hide IP on smaller desktops to save space
  }
]

/**
 * Load logs from API
 */
async function loadLogs() {
  // Expand user details to show who performed the action
  await load({ 
    expand: 'user',
    sort: '-created'
  })
}

/**
 * Handle Modal
 */
function openDetails(log: AuditLog) {
  selectedLog.value = log
  // DaisyUI modal toggle
  const modal = document.getElementById('audit_modal') as HTMLDialogElement
  if (modal) modal.showModal()
}

/**
 * Handle row click
 */
function handleRowClick(log: AuditLog) {
  openDetails(log)
}

/**
 * Get badge class based on event type
 */
function getEventBadgeClass(eventType: string) {
  if (eventType.includes('create')) return 'badge-success'
  if (eventType.includes('update')) return 'badge-warning'
  if (eventType.includes('delete')) return 'badge-error'
  if (eventType.includes('auth')) return 'badge-info'
  return 'badge-ghost'
}

/**
 * Handle organization change event
 * Audit logs might be global or org-specific depending on backend rules,
 * but refreshing ensures UI consistency.
 */
function handleOrgChange() {
  searchQuery.value = ''
  loadLogs()
}

onMounted(() => {
  loadLogs()
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
        <h1 class="text-3xl font-bold">Audit Logs</h1>
        <p class="text-base-content/70 mt-1">
          System activity and change history
        </p>
      </div>
      
      <!-- Refresh Button -->
      <button @click="loadLogs" class="btn btn-ghost btn-sm" title="Refresh Logs">
        🔄 Refresh
      </button>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by user, collection, or ID..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && logs.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="logs.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📋</span>
        <h3 class="text-xl font-bold mt-4">No audit logs found</h3>
        <p class="text-base-content/70 mt-2">
          Activity will appear here when users interact with the system.
        </p>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="filteredLogs.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching logs</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :items="filteredLogs" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Time Column -->
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

        <!-- User Column -->
        <template #cell-expand.user.name="{ item }">
          <div class="flex items-center gap-2">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-8">
                <span class="text-xs">{{ item.expand?.user?.name?.[0] || 'S' }}</span>
              </div>
            </div>
            <div class="flex flex-col">
              <span class="text-sm font-medium">
                {{ item.expand?.user?.name || item.expand?.user?.email || 'System' }}
              </span>
            </div>
          </div>
        </template>
        <template #card-expand.user.name="{ item }">
          <div class="flex items-center gap-2 font-medium">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-6">
                <span class="text-xs">{{ item.expand?.user?.name?.[0] || 'S' }}</span>
              </div>
            </div>
            {{ item.expand?.user?.name || item.expand?.user?.email || 'System' }}
          </div>
        </template>

        <!-- Action Column -->
        <template #cell-event_type="{ item }">
          <span class="badge badge-sm" :class="getEventBadgeClass(item.event_type)">
            {{ formatConstant(item.event_type) }}
          </span>
        </template>
        <template #card-event_type="{ item }">
          <div class="flex justify-between items-center">
            <span class="text-xs text-base-content/70">Event Type</span>
            <span class="badge badge-sm" :class="getEventBadgeClass(item.event_type)">
              {{ formatConstant(item.event_type) }}
            </span>
          </div>
        </template>

        <!-- Collection Column -->
        <template #cell-collection_name="{ item }">
          <code class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.collection_name }}</code>
        </template>
        <template #card-collection_name="{ item }">
          <div class="flex justify-between items-center mt-1">
            <span class="text-xs text-base-content/70">Collection</span>
            <code class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.collection_name }}</code>
          </div>
        </template>

        <!-- View Button -->
        <template #actions="{ item }">
          <button @click.stop="openDetails(item)" class="btn btn-ghost btn-xs">
            Details
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ logs.length }} of {{ totalItems }} entries
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ expand: 'user', sort: '-created' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ expand: 'user', sort: '-created' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>

    <!-- Details Modal -->
    <dialog id="audit_modal" class="modal">
      <div class="modal-box w-11/12 max-w-4xl">
        <h3 class="font-bold text-lg mb-4">Log Details</h3>
        
        <div v-if="selectedLog" class="space-y-4">
          <!-- Meta Grid -->
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
            <!-- Before -->
            <div>
              <h4 class="font-bold text-sm mb-2 opacity-70">Before Changes</h4>
              <div v-if="selectedLog.before_changes && Object.keys(selectedLog.before_changes).length" 
                   class="mockup-code bg-base-300 text-xs min-h-[100px]">
                <pre><code>{{ JSON.stringify(selectedLog.before_changes, null, 2) }}</code></pre>
              </div>
              <div v-else class="text-sm italic opacity-50 p-4 bg-base-200 rounded">
                No previous state
              </div>
            </div>

            <!-- After -->
            <div>
              <h4 class="font-bold text-sm mb-2 opacity-70">After Changes</h4>
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
