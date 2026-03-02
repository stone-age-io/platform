<template>
  <BaseCard :no-padding="true">
    <template #header>
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-2">
        <div class="flex items-center gap-2">
          <h3 class="card-title">Live Occupancy</h3>
          <div v-if="occupants.length > 0" class="badge badge-success gap-1 font-mono">
            {{ occupants.length }} <span class="hidden sm:inline">Present</span>
          </div>
          <div v-else-if="!loading" class="badge badge-ghost opacity-50">Empty</div>
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
    <div v-else-if="occupants.length === 0" class="text-center py-12 opacity-50">
      <span class="text-4xl block mb-2">👻</span>
      <span class="text-sm font-bold">No occupants detected</span>
      <p class="text-xs mt-1">Waiting for OwnTracks events...</p>
    </div>

    <!-- List -->
    <ResponsiveList
      v-else
      :items="filteredOccupants"
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
          <span class="text-xs opacity-50 font-mono">{{ formatDate(item.timestamp, 'HH:mm:ss') }}</span>
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
            <div class="text-xs opacity-60">Seen {{ formatRelativeTime(item.timestamp) }}</div>
          </div>
        </div>
      </template>
    </ResponsiveList>

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
                <p class="text-xs text-base-content/60">Last seen {{ formatRelativeTime(selectedOccupant.timestamp) }}</p>
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
import { ref, computed } from 'vue'
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

function handleRowClick(item: Occupant) {
  selectedOccupant.value = item
}
</script>
