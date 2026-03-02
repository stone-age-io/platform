<template>
  <BaseCard :no-padding="true">
    <template #header>
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-2">
        <div class="flex items-center gap-2">
          <h3 class="card-title">Live Occupancy</h3>
          <div v-if="occupants.length > 0" class="badge badge-success gap-1 font-mono">
            {{ occupants.length }} <span class="hidden sm:inline">Present</span>
          </div>
          <div v-else-if="!loading" class="badge badge-ghost text-base-content/70">Empty</div>
        </div>

        <!-- Simple Search -->
        <input
          v-model="search"
          type="text"
          placeholder="Filter users..."
          class="input input-xs input-bordered w-full sm:w-48"
        />
      </div>
    </template>

    <!-- Loading State -->
    <div v-if="loading && occupants.length === 0" class="flex justify-center p-8">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Empty State -->
    <div v-else-if="occupants.length === 0" class="text-center py-12 text-base-content/70">
      <span class="text-4xl block mb-2">👻</span>
      <span class="text-sm font-bold">No occupants detected</span>
      <p class="text-xs mt-1">Waiting for presence events...</p>
    </div>

    <!-- No Results -->
    <div v-else-if="filteredOccupants.length === 0" class="text-center py-12">
      <span class="text-4xl block mb-2">🔍</span>
      <span class="text-sm font-bold">No matching users</span>
      <button @click="search = ''" class="btn btn-ghost btn-xs mt-2">Clear Search</button>
    </div>

    <!-- List -->
    <template v-else>
      <ResponsiveList
        :items="paginatedOccupants"
        :columns="columns"
        :clickable="true"
        @row-click="handleRowClick"
      >
        <!-- User Identity Column -->
        <template #cell-user_id="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-8">
                <span class="text-xs font-bold">{{ item.user_id.substring(0,2).toUpperCase() }}</span>
              </div>
            </div>
            <div class="font-medium font-mono text-sm">{{ item.user_id }}</div>
          </div>
        </template>

        <!-- Time Column -->
        <template #cell-timestamp="{ item }">
          <div class="flex flex-col">
            <span class="text-sm font-medium">{{ formatRelativeTime(item.timestamp) }}</span>
            <span class="text-xs text-base-content/70 font-mono">{{ formatDate(item.timestamp, 'HH:mm:ss') }}</span>
          </div>
        </template>

        <!-- Mobile Card View Override -->
        <template #card-user_id="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-10">
                <span class="text-sm font-bold">{{ item.user_id.substring(0,2).toUpperCase() }}</span>
              </div>
            </div>
            <div>
              <div class="font-bold font-mono">{{ item.user_id }}</div>
              <div class="text-xs text-base-content/70">Seen {{ formatRelativeTime(item.timestamp) }}</div>
            </div>
          </div>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          {{ rangeStart }}–{{ rangeEnd }} of {{ filteredOccupants.length }} users
        </span>
        <div class="join">
          <button class="join-item btn btn-sm" :disabled="currentPage === 1" @click="currentPage--">«</button>
          <button class="join-item btn btn-sm">Page {{ currentPage }}</button>
          <button class="join-item btn btn-sm" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
        </div>
      </div>
    </template>

    <!-- Detail Modal -->
    <Teleport to="body">
      <div v-if="selectedOccupant" class="modal modal-open" @click.self="selectedOccupant = null">
        <div class="modal-box max-w-lg">
          <div class="flex justify-between items-center mb-4">
            <div class="flex items-center gap-3">
              <div class="avatar placeholder">
                <div class="bg-neutral text-neutral-content rounded-full w-10">
                  <span class="text-sm font-bold">{{ selectedOccupant.user_id.substring(0,2).toUpperCase() }}</span>
                </div>
              </div>
              <div>
                <h3 class="font-bold font-mono">{{ selectedOccupant.user_id }}</h3>
                <p class="text-xs text-base-content/70">Last seen {{ formatRelativeTime(selectedOccupant.timestamp) }}</p>
              </div>
            </div>
            <button class="btn btn-sm btn-circle btn-ghost" @click="selectedOccupant = null">✕</button>
          </div>
          <div class="bg-base-200 rounded-lg p-4 overflow-auto max-h-96 border border-base-300">
            <JsonViewer :data="selectedOccupant.rawData" />
          </div>
          <div class="modal-action">
            <button class="btn btn-sm" @click="selectedOccupant = null">Close</button>
          </div>
        </div>
      </div>
    </Teleport>
  </BaseCard>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useOccupancy, type Occupant } from '@/composables/useOccupancy'
import { formatRelativeTime, formatDate } from '@/utils/format'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'

const props = defineProps<{
  locationCode?: string
}>()

// Use the composable
const { occupants, loading } = useOccupancy(() => props.locationCode)
const search = ref('')
const selectedOccupant = ref<Occupant | null>(null)

// Pagination
const currentPage = ref(1)
const itemsPerPage = 10

const columns: Column<any>[] = [
  { key: 'user_id', label: 'User', mobileLabel: 'User' },
  { key: 'timestamp', label: 'Last Seen', mobileLabel: 'Seen' },
]

// Client-side filtering
const filteredOccupants = computed(() => {
  if (!search.value) return occupants.value
  const q = search.value.toLowerCase()
  return occupants.value.filter(o => o.user_id.toLowerCase().includes(q))
})

// Client-side pagination
const totalPages = computed(() => Math.ceil(filteredOccupants.value.length / itemsPerPage))

const rangeStart = computed(() => (currentPage.value - 1) * itemsPerPage + 1)
const rangeEnd = computed(() => Math.min(currentPage.value * itemsPerPage, filteredOccupants.value.length))

const paginatedOccupants = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return filteredOccupants.value.slice(start, start + itemsPerPage)
})

// Reset page when search changes or when occupant count shifts pages
watch(search, () => {
  currentPage.value = 1
})

watch(totalPages, (pages) => {
  if (currentPage.value > pages && pages > 0) {
    currentPage.value = pages
  }
})

// Clear stale modal when location changes
watch(() => props.locationCode, () => {
  selectedOccupant.value = null
})

function handleRowClick(item: Occupant) {
  selectedOccupant.value = item
}

// Escape key to close detail modal
function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape' && selectedOccupant.value) {
    selectedOccupant.value = null
  }
}

onMounted(() => window.addEventListener('keydown', handleEscape))
onUnmounted(() => window.removeEventListener('keydown', handleEscape))
</script>
