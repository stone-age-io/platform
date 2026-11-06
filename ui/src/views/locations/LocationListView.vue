<script setup lang="ts">
import { ListIcon, MapIcon, MapPin, SearchX, Tag } from '@lucide/vue'
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Location } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import LocationMapViz from '@/components/locations/LocationMapViz.vue'
import QrLabelModal from '@/components/common/QrLabelModal.vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const toast = useToast()
const authStore = useAuthStore()

// `viewer` reaches this screen and writes nothing, so Edit needs its own gate.
// Delete lives only in the detail view's danger zone (decommissionInventory).
const canWrite = computed(() => authStore.can.manageInventory)

const viewMode = ref<'list' | 'map'>('list')

const allLocations = ref<Location[]>([])
const loading = ref(false)
const searchQuery = ref('')

// Search and pagination are client-side: this screen holds every location.
const currentPage = ref(1)
const itemsPerPage = 20

const filteredLocations = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()

  if (!q) {
    // Roots only when not searching
    return allLocations.value.filter(l => !l.parent)
  }

  return allLocations.value.filter(l => {
    const nameMatch = l.name?.toLowerCase().includes(q)
    const codeMatch = l.code?.toLowerCase().includes(q)
    const descMatch = l.description?.toLowerCase().includes(q)
    const typeMatch = l.expand?.type?.name?.toLowerCase().includes(q)
    const parentMatch = (l.expand?.parent as Location)?.name?.toLowerCase().includes(q)
    // Serialised so metadata matches on keys and values without knowing the shape.
    const metaMatch = l.metadata ? JSON.stringify(l.metadata).toLowerCase().includes(q) : false

    return nameMatch || codeMatch || descMatch || typeMatch || parentMatch || metaMatch
  })
})

const paginatedLocations = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredLocations.value.slice(start, end)
})

const totalPages = computed(() => Math.ceil(filteredLocations.value.length / itemsPerPage))

// Labels print the whole current filter, not the page. With no search that is
// roots only, deliberately: what the list shows is what prints.
const showLabelModal = ref(false)
const labelRecords = computed(() =>
  filteredLocations.value.map((l) => ({
    code: l.code || '',
    name: l.name || '',
    kind: 'location' as const,
  })),
)

async function loadLocations() {
  loading.value = true
  try {
    allLocations.value = await pb.collection('locations').getFullList<Location>({
      sort: 'name',
      expand: 'type,parent',
    })
  } catch (err: any) {
    console.error('Failed to load locations', err)
    toast.error('Failed to load locations')
  } finally {
    loading.value = false
  }
}

const columns: Column<Location>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name',
    cardWide: true, label: 'Type', mobileLabel: 'Type' },
  { key: 'expand.parent.name',
    cardWide: true, label: 'Parent', mobileLabel: 'Parent', class: 'hidden md:table-cell' },
  { key: 'code', width: '11rem', label: 'Code', mobileLabel: 'Code' },
  { key: 'created', width: '8rem', label: 'Created', mobileLabel: 'Created', format: (value) => formatDate(value, 'PP') },
]

function handleRowClick(location: Location) {
  router.push(`/locations/${location.id}`)
}

watch(searchQuery, () => {
  currentPage.value = 1
})

onMounted(() => {
  loadLocations()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">

      <div class="w-full sm:w-auto">
        <div class="flex justify-between items-center">
          <h1 class="text-3xl font-bold">Locations</h1>

          <div class="join shadow-sm border border-base-300 sm:hidden">
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'list' }"
              @click="viewMode = 'list'"
            >
              <ListIcon :size="16" class="inline-block shrink-0" />
            </button>
            <button
              class="join-item btn btn-sm px-3"
              :class="{ 'btn-active': viewMode === 'map' }"
              @click="viewMode = 'map'"
            >
              <MapIcon :size="16" class="inline-block shrink-0" />
            </button>
          </div>
        </div>
        <p class="text-base-content/70 mt-1">
          Manage physical sites and facilities
        </p>
      </div>

      <!-- Buttons are `flex-1 sm:flex-initial`, not `w-full`: `.btn` has
           `flex-shrink: 0`, so a w-full button sharing the row overflows it. -->
      <div class="flex gap-3 w-full sm:w-auto">

        <div class="hidden sm:inline-flex join shadow-sm border border-base-300">
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'list' }"
            @click="viewMode = 'list'"
          >
            <ListIcon :size="16" class="inline-block shrink-0" /> List
          </button>
          <button
            class="join-item btn"
            :class="{ 'btn-active': viewMode === 'map' }"
            @click="viewMode = 'map'"
          >
            <MapIcon :size="16" class="inline-block shrink-0" /> Map
          </button>
        </div>

        <button
          v-if="viewMode === 'list' && filteredLocations.length > 0"
          class="btn btn-outline flex-1 sm:flex-initial"
          @click="showLabelModal = true"
        >
          <Tag :size="16" class="inline-block shrink-0" />
          <span>Labels ({{ filteredLocations.length }})</span>
        </button>

        <router-link v-if="canWrite" to="/locations/new" class="btn btn-primary flex-1 sm:flex-initial">
          <span class="text-lg">+</span>
          <span>New Location</span>
        </router-link>
      </div>
    </div>

    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, code, type, parent, description, or metadata..."
        class="input input-bordered w-full"
      />
      <label v-if="!searchQuery && viewMode === 'list'" class="label">
        <span class="label-text-alt text-base-content/60">
          Showing top-level locations. Use search to find sub-locations.
        </span>
      </label>
      <label v-else-if="searchQuery" class="label">
        <span class="label-text-alt text-base-content/60">
          Searching all {{ allLocations.length }} locations.
        </span>
      </label>
    </div>

    <div v-if="viewMode === 'list'" class="space-y-6 fade-in">
      <BaseCard :no-padding="true">
        <div v-if="loading && allLocations.length === 0" class="flex justify-center p-12">
          <span class="loading loading-spinner loading-lg"></span>
        </div>

        <div v-else-if="allLocations.length === 0" class="text-center py-12">
          <MapPin :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">No locations found</h3>
          <p class="text-base-content/70 mt-2">
            {{ canWrite ? 'Create your first root location to get started.' : 'This organization has no locations yet.' }}
          </p>
        </div>

        <div v-else-if="filteredLocations.length === 0" class="text-center py-12">
          <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">No matching locations</h3>
          <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
        </div>

        <template v-else>
          <ResponsiveList
            :items="paginatedLocations"
            :columns="columns"
            :clickable="true"
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
                <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">{{ item.description }}</div>
              </div>
            </template>

            <template #cell-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">{{ item.expand.type.name }}</span>
              <span v-else class="text-base-content/40">—</span>
            </template>

            <template #cell-expand.parent.name="{ item }">
              <span v-if="item.expand?.parent" class="text-sm text-base-content/80">{{ item.expand.parent.name }}</span>
              <span v-else class="text-base-content/30 text-xs italic">Root</span>
            </template>

            <template #card-expand.type.name="{ item }">
              <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">{{ item.expand.type.name }}</span>
              <span v-else class="text-base-content/40">—</span>
            </template>

            <template #cell-code="{ item }">
              <code v-if="item.code" class="text-xs bg-base-200 px-1 py-0.5 rounded">{{ item.code }}</code>
              <span v-else class="text-base-content/40">—</span>
            </template>

            <template #actions="{ item }">
              <router-link v-if="canWrite" :to="`/locations/${item.id}/edit`" class="btn btn-xs" @click.stop>Edit</router-link>
              <router-link v-if="!canWrite" :to="`/locations/${item.id}`" class="btn btn-xs" @click.stop>View</router-link>
            </template>
          </ResponsiveList>

          <ListPager
            v-if="totalPages > 1"
            :page="currentPage"
            :total-pages="totalPages"
            :shown="paginatedLocations.length"
            :total="filteredLocations.length"
            noun="locations"
            @prev="currentPage--"
            @next="currentPage++"
          />
        </template>
      </BaseCard>
    </div>

    <div v-else-if="viewMode === 'map'" class="fade-in">
      <LocationMapViz :search-query="searchQuery" />
    </div>

    <QrLabelModal
      v-if="showLabelModal"
      :records="labelRecords"
      @close="showLabelModal = false"
    />

  </div>
</template>

<style scoped>
.fade-in {
  animation: fadeIn 0.2s ease-in-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
