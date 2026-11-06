<script setup lang="ts">
import { ListIcon, MapIcon, Package, SearchX, Tag } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import ThingMapViz from '@/components/things/ThingMapViz.vue'
import QrLabelModal from '@/components/common/QrLabelModal.vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const toast = useToast()
const authStore = useAuthStore()

// `viewer` reaches this screen and writes nothing, so Edit needs its own gate.
// Delete lives only in the detail view's danger zone (decommissionInventory).
const canWrite = computed(() => authStore.can.manageInventory)

const viewMode = ref<'list' | 'map'>('list')

const {
  items: things,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<Thing>('things', 20)

// Search runs on the server, across all records, not the page on screen.
// PocketBase matches a json column as text, so one `~` on `metadata` covers its
// keys and values without knowing any schema.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description', 'code', 'metadata', 'type.name', 'location.name'],
  () => {
    page.value = 1
    loadThings()
  },
)

// PocketBase sort syntax ('name', '-created').
const sort = ref('')

// Pass this to EVERY usePagination call, the pager buttons included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
  expand: 'type,location',
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1 // a new order makes the old page number meaningless
  loadThings()
}

// Labels print the whole current filter, not the page; the search box is the
// selection mechanism. No capability gate: it only reads what the screen already
// lets the caller read.
const showLabelModal = ref(false)
const loadingLabels = ref(false)
const labelRecords = ref<{ code: string; name: string; kind: 'thing' }[]>([])

async function openLabels() {
  loadingLabels.value = true
  try {
    // Not `things` (one page). Same filter and sort as the list, so labels print
    // in screen order.
    const all = await pb.collection('things').getFullList<Thing>({
      filter: searchFilter.value,
      sort: sort.value || 'name',
      fields: 'code,name',
    })
    labelRecords.value = all.map((t) => ({
      code: t.code || '',
      name: t.name || '',
      kind: 'thing' as const,
    }))
    showLabelModal.value = true
  } catch (err: any) {
    toast.error(err.message || 'Failed to load things for labels')
  } finally {
    loadingLabels.value = false
  }
}

const columns: Column<Thing>[] = [
  {
    key: 'name',
    sortable: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.type.name',
    cardWide: true,
    sortable: 'type.name',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'expand.location.name',
    cardWide: true,
    sortable: 'location.name',
    label: 'Location',
    mobileLabel: 'Location',
  },
  {
    key: 'code',
    sortable: 'code',
    width: '11rem',
    label: 'Code',
    mobileLabel: 'Code',
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

async function loadThings() {
  // API rules scope to the org; this adds only the search.
  await load(queryOptions.value)
}

function handleRowClick(thing: Thing) {
  router.push(`/things/${thing.id}`)
}

onMounted(() => {
  loadThings()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div class="w-full sm:w-auto">
        <div class="flex justify-between items-center">
          <h1 class="text-3xl font-bold">Things</h1>

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
          Manage IoT devices and sensors
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
          v-if="viewMode === 'list' && totalItems > 0"
          class="btn btn-outline flex-1 sm:flex-initial"
          :disabled="loadingLabels"
          @click="openLabels"
        >
          <span v-if="loadingLabels" class="loading loading-spinner loading-xs"></span>
          <Tag v-else :size="16" class="inline-block shrink-0" />
          <span>Labels ({{ totalItems }})</span>
        </button>

        <router-link v-if="canWrite" to="/things/new" class="btn btn-primary flex-1 sm:flex-initial">
          <span class="text-lg">+</span>
          <span>New Thing</span>
        </router-link>
      </div>
    </div>
    
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, code, description, type, location, or metadata..."
        class="input input-bordered w-full"
      />
      <label v-if="searchQuery && viewMode === 'list'" class="label">
        <span class="label-text-alt">
          {{ totalItems }} match{{ totalItems === 1 ? '' : 'es' }} across all things
        </span>
      </label>
    </div>

    <template v-if="viewMode === 'list'">

    <div v-if="loading && things.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error && things.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load things</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadThings" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Guarded on !searchQuery: an empty search result means no match, not no inventory. -->
    <BaseCard v-else-if="things.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Package :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No things found</h3>
        <p class="text-base-content/70 mt-2">
          {{ canWrite ? 'Create your first thing to get started' : 'This organization has no things yet' }}
        </p>
        <router-link v-if="canWrite" to="/things/new" class="btn btn-primary mt-4">
          Create Thing
        </router-link>
      </div>
    </BaseCard>
    
    <BaseCard v-else-if="things.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching things</h3>
        <p class="text-base-content/70 mt-2">
          Try a different search term
        </p>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :sort="sort"
        @update:sort="onSort"
        :items="things"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <!-- Deactivated rides with the name: it is an exception, not a column to scan. -->
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium flex items-center gap-2">
              <span :class="{ 'text-base-content/50': item.active === false }">
                {{ item.name || 'Unnamed' }}
              </span>
              <span v-if="item.active === false" class="badge badge-error badge-outline badge-sm">
                Deactivated
              </span>
            </div>
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base flex items-center gap-2">
              <span :class="{ 'text-base-content/50': item.active === false }">
                {{ item.name || 'Unnamed' }}
              </span>
              <span v-if="item.active === false" class="badge badge-error badge-outline badge-sm">
                Deactivated
              </span>
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <template #cell-expand.type.name="{ item }">
          <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">
            {{ item.expand.type.name }}
          </span>
          <span v-else class="text-base-content/40">—</span>
        </template>
        
        <template #card-expand.type.name="{ item }">
         <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">
           {{ item.expand.type.name }}
         </span>
         <span v-else class="text-base-content/40">—</span>
        </template>

        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">—</span>
        </template>
        
        <!-- ResponsiveList handles @click.stop -->
        <template #actions="{ item }">
          <router-link
            v-if="canWrite"
            :to="`/things/${item.id}/edit`"
            class="btn btn-xs"
          >
            Edit
          </router-link>
          <router-link
            v-if="!canWrite"
            :to="`/things/${item.id}`"
            class="btn btn-xs"
          >
            View
          </router-link>
        </template>
      </ResponsiveList>
      
      <!-- Pass queryOptions, not a bare expand, or page 2 loses the search. -->
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="things.length"
        :total="totalItems"
        :noun="searchQuery ? 'matches' : 'things'"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>

    </template>

    <div v-else-if="viewMode === 'map'">
      <ThingMapViz :search-query="searchQuery" />
    </div>

    <QrLabelModal
      v-if="showLabelModal"
      :records="labelRecords"
      @close="showLabelModal = false"
    />
  </div>
</template>
