<script setup lang="ts">
import { Boxes, RadioTower, RefreshCw, SearchX, ShieldOff, TriangleAlert } from '@lucide/vue'
import { ref, computed, watch, onMounted } from 'vue'
import { useNow } from '@vueuse/core'
import { useNatsStore } from '@/stores/nats'
import { discoverServices, DiscoveryDeniedError } from '@/utils/natsServices'
import { formatAge } from '@/utils/format'
import { sortBy } from '@/utils/clientSort'
import type { ServiceInstance } from '@/types/services'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

// Asked, never stored: the list is whoever answered `$SRV.INFO` just now, the
// same shape as the site-connectivity recipe. There is no "offline" row,
// because a service that did not answer has left no record to show.

const natsStore = useNatsStore()

const services = ref<ServiceInstance[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const denied = ref(false)
const askedAt = ref<number | null>(null)
const searchQuery = ref('')
const sort = ref('name')

const now = useNow({ interval: 1000 })
const askedAge = computed(() => askedAt.value === null ? '' : formatAge(now.value.getTime() - askedAt.value))
const serviceCount = computed(() => new Set(services.value.map(s => s.name)).size)

const filteredServices = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return services.value
  return services.value.filter(s =>
    [s.name, s.id, s.version, s.description, ...Object.values(s.metadata), ...s.endpoints.map(e => e.subject)]
      .some(v => String(v).toLowerCase().includes(q))
  )
})

// Sorted before the slice, like the JetStream lists: discovery returns the
// whole account (see utils/clientSort).
const sortedServices = computed(() => sortBy(filteredServices.value, sort.value))

const currentPage = ref(1)
const itemsPerPage = 20
const totalPages = computed(() => Math.max(1, Math.ceil(filteredServices.value.length / itemsPerPage)))
const paginatedServices = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return sortedServices.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })

function onSort(next: string) {
  sort.value = next
  currentPage.value = 1
}

// Ascending `started` is longest-running first.
const columns: Column<ServiceInstance>[] = [
  { key: 'name', sortable: 'name', label: 'Service', mobileLabel: 'Service' },
  { key: 'version', sortable: 'version', width: '7rem', label: 'Version', mobileLabel: 'Version' },
  { key: 'endpoints', cardWide: true, label: 'Endpoints', mobileLabel: 'Endpoints' },
  { key: 'started', sortable: 'started', width: '6rem', label: 'Up', mobileLabel: 'Up' },
  { key: 'requests', sortable: '-requests', width: '7rem', label: 'Requests', mobileLabel: 'Reqs', format: (v: number | null) => v === null ? '—' : v.toLocaleString() },
  { key: 'errors', sortable: '-errors', width: '6rem', label: 'Errors', mobileLabel: 'Errors' },
]

function uptime(started: string | null): string {
  if (!started || askedAt.value === null) return '—'
  return formatAge(askedAt.value - new Date(started).getTime()).replace(/ ago$/, '')
}

async function loadData() {
  if (!natsStore.isConnected || !natsStore.nc) {
    loading.value = false
    return
  }
  loading.value = true
  error.value = null
  denied.value = false
  try {
    services.value = await discoverServices(natsStore.nc)
    askedAt.value = Date.now()
  } catch (e: any) {
    if (e instanceof DiscoveryDeniedError) denied.value = true
    else error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(() => { loadData() })

watch(() => natsStore.isConnected, (connected) => {
  if (connected) loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Services</h1>
        <p class="text-base-content/70 mt-1">NATS micro services answering in this account right now</p>
      </div>
      <button v-if="natsStore.isConnected" class="btn w-full sm:w-auto" :disabled="loading" @click="loadData">
        <RefreshCw :size="16" :class="{ 'animate-spin': loading }" />
        <span>Refresh</span>
      </button>
    </div>

    <!-- Not Connected -->
    <BaseCard v-if="!natsStore.isConnected">
      <div class="text-center py-12">
        <RadioTower :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">Not connected to NATS</h3>
        <p class="text-base-content/70 mt-2">Connect to NATS to discover services.</p>
      </div>
    </BaseCard>

    <template v-else>
      <!-- Loading State -->
      <div v-if="loading && services.length === 0" class="flex justify-center p-12">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <!-- Denied: the console's own credential, not the services -->
      <BaseCard v-else-if="denied">
        <div class="text-center py-12">
          <ShieldOff :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Your NATS user cannot ask</h3>
          <p class="text-base-content/70 mt-2 max-w-lg mx-auto">
            Discovery publishes to <code class="font-mono">$SRV.INFO</code> and <code class="font-mono">$SRV.STATS</code>.
            Give the role on your NATS user publish on <code class="font-mono">$SRV.&gt;</code>.
          </p>
        </div>
      </BaseCard>

      <!-- Error State -->
      <BaseCard v-else-if="error && services.length === 0">
        <div class="text-center py-12">
          <TriangleAlert :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Discovery failed</h3>
          <p class="text-base-content/70 mt-2">{{ error }}</p>
          <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
        </div>
      </BaseCard>

      <!-- Empty State: silence, which is not the same as down -->
      <BaseCard v-else-if="services.length === 0">
        <div class="text-center py-12">
          <Boxes :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Nothing answered</h3>
          <p class="text-base-content/70 mt-2 max-w-lg mx-auto">
            A service shows here only while it is running and its NATS user may subscribe to
            <code class="font-mono">$SRV.&gt;</code>. Behind a leaf node, the site's uplink credential needs
            <code class="font-mono">$SRV.&gt;</code> too, so silence is not proof that anything is down.
          </p>
        </div>
      </BaseCard>

      <template v-else>
        <!-- Search -->
        <div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search by name, id, version, metadata, or subject..."
            class="input input-bordered w-full sm:flex-1"
          />
          <span class="text-sm text-base-content/60 whitespace-nowrap">
            {{ services.length }} {{ services.length === 1 ? 'instance' : 'instances' }} of
            {{ serviceCount }} {{ serviceCount === 1 ? 'service' : 'services' }} · asked {{ askedAge }}
          </span>
        </div>

        <!-- No Search Results -->
        <BaseCard v-if="filteredServices.length === 0">
          <div class="text-center py-12">
            <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
            <h3 class="text-xl font-bold mt-4">No matching services</h3>
            <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
          </div>
        </BaseCard>

        <!-- Service List -->
        <BaseCard v-else :no-padding="true">
          <ResponsiveList
            :items="paginatedServices"
            :columns="columns"
            :loading="loading"
            :clickable="false"
            :sort="sort"
            @update:sort="onSort"
          >
            <template #cell-name="{ item }">
              <div class="min-w-0">
                <div class="font-medium font-mono">{{ item.name }}</div>
                <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
                <div class="text-xs font-mono text-base-content/50 truncate" :title="item.id">
                  {{ item.id }}<template v-for="(v, k) in item.metadata" :key="k"> · {{ k }}={{ v }}</template>
                </div>
              </div>
            </template>

            <template #card-name="{ item }">
              <div>
                <div class="font-semibold font-mono text-base">{{ item.name }}</div>
                <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">{{ item.description }}</div>
                <div class="text-xs font-mono text-base-content/50 mt-1 break-all">
                  {{ item.id }}<template v-for="(v, k) in item.metadata" :key="k"> · {{ k }}={{ v }}</template>
                </div>
              </div>
            </template>

            <template #cell-endpoints="{ item }">
              <div class="font-mono text-xs truncate" :title="item.endpoints.map(e => e.subject).join('\n')">
                {{ item.endpoints.map(e => e.subject).join(', ') || '—' }}
              </div>
            </template>

            <template #card-endpoints="{ item }">
              <div class="font-mono text-xs break-all">{{ item.endpoints.map(e => e.subject).join(', ') || '—' }}</div>
            </template>

            <template #cell-started="{ item }">{{ uptime(item.started) }}</template>
            <template #card-started="{ item }">{{ uptime(item.started) }}</template>

            <template #cell-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }" :title="item.lastError || undefined">
                {{ item.errors === null ? '—' : item.errors.toLocaleString() }}
              </span>
            </template>
            <template #card-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }" :title="item.lastError || undefined">
                {{ item.errors === null ? '—' : item.errors.toLocaleString() }}
              </span>
            </template>
          </ResponsiveList>

          <ListPager
            v-if="filteredServices.length > itemsPerPage"
            :page="currentPage"
            :total-pages="totalPages"
            :shown="paginatedServices.length"
            :total="filteredServices.length"
            noun="services"
            @prev="currentPage--"
            @next="currentPage++"
          />
        </BaseCard>
      </template>
    </template>
  </div>
</template>
