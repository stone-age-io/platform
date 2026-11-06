<script setup lang="ts">
import { Boxes, RadioTower, RefreshCw, SearchX, ShieldOff, TriangleAlert } from '@lucide/vue'
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNow } from '@vueuse/core'
import { useNatsStore } from '@/stores/nats'
import { discoverServices, groupServices, shortVersion, DiscoveryDeniedError } from '@/utils/natsServices'
import { formatAge } from '@/utils/format'
import { sortBy } from '@/utils/clientSort'
import type { ServiceGroup, ServiceInstance } from '@/types/services'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

// Asked, never stored: the list is whoever answered `$SRV.INFO` just now, so
// there is no "offline" row. One row per service; instances, told apart by
// their metadata, are on the detail page.

const router = useRouter()
const natsStore = useNatsStore()

const instances = ref<ServiceInstance[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const denied = ref(false)
const askedAt = ref<number | null>(null)
const searchQuery = ref('')
const sort = ref('name')

const now = useNow({ interval: 1000 })
const askedAge = computed(() => askedAt.value === null ? '' : formatAge(now.value.getTime() - askedAt.value))

// `instanceCount` is a field so the column can sort on it.
const groups = computed(() => groupServices(instances.value).map(g => ({ ...g, instanceCount: g.instances.length })))

// A group matches when the service does or any one of its instances does, so
// searching for a site code finds the services running there.
const filteredGroups = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return groups.value
  return groups.value.filter(g =>
    [g.name, g.description, ...g.versions].some(v => v.toLowerCase().includes(q)) ||
    g.instances.some(i => [i.id, ...Object.values(i.metadata)].some(v => String(v).toLowerCase().includes(q)))
  )
})

// Sorted before the slice, like the JetStream lists: discovery returns the
// whole account (see utils/clientSort).
const sortedGroups = computed(() => sortBy(filteredGroups.value, sort.value))

const currentPage = ref(1)
const itemsPerPage = 20
const totalPages = computed(() => Math.max(1, Math.ceil(filteredGroups.value.length / itemsPerPage)))
const paginatedGroups = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return sortedGroups.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })

function onSort(next: string) {
  sort.value = next
  currentPage.value = 1
}

const columns: Column<ServiceGroup>[] = [
  { key: 'name', sortable: 'name', label: 'Service', mobileLabel: 'Service' },
  { key: 'instanceCount', sortable: '-instanceCount', width: '7rem', label: 'Instances', mobileLabel: 'Instances' },
  { key: 'versions', width: '11rem', label: 'Version', mobileLabel: 'Version' },
  { key: 'requests', sortable: '-requests', width: '7rem', label: 'Requests', mobileLabel: 'Reqs', format: (v: number | null) => v === null ? '—' : v.toLocaleString() },
  { key: 'errors', sortable: '-errors', width: '6rem', label: 'Errors', mobileLabel: 'Errors' },
]

async function loadData() {
  if (!natsStore.isConnected || !natsStore.nc) {
    loading.value = false
    return
  }
  loading.value = true
  error.value = null
  denied.value = false
  try {
    instances.value = await discoverServices(natsStore.nc)
    askedAt.value = Date.now()
  } catch (e: any) {
    if (e instanceof DiscoveryDeniedError) denied.value = true
    else error.value = e.message
  } finally {
    loading.value = false
  }
}

function handleRowClick(group: ServiceGroup) {
  router.push(`/nats/services/${encodeURIComponent(group.name)}`)
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
      <div v-if="loading && instances.length === 0" class="flex justify-center p-12">
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
      <BaseCard v-else-if="error && instances.length === 0">
        <div class="text-center py-12">
          <TriangleAlert :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Discovery failed</h3>
          <p class="text-base-content/70 mt-2">{{ error }}</p>
          <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
        </div>
      </BaseCard>

      <!-- Empty State: silence, which is not the same as down -->
      <BaseCard v-else-if="instances.length === 0">
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
            placeholder="Search by service, version, instance id, or metadata..."
            class="input input-bordered w-full sm:flex-1"
          />
          <span class="text-sm text-base-content/60 whitespace-nowrap">
            {{ instances.length }} {{ instances.length === 1 ? 'instance' : 'instances' }} of
            {{ groups.length }} {{ groups.length === 1 ? 'service' : 'services' }} · asked {{ askedAge }}
          </span>
        </div>

        <!-- No Search Results -->
        <BaseCard v-if="filteredGroups.length === 0">
          <div class="text-center py-12">
            <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
            <h3 class="text-xl font-bold mt-4">No matching services</h3>
            <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
          </div>
        </BaseCard>

        <!-- Service List -->
        <BaseCard v-else :no-padding="true">
          <ResponsiveList
            :items="paginatedGroups"
            :columns="columns"
            :loading="loading"
            :sort="sort"
            @update:sort="onSort"
            @row-click="handleRowClick"
          >
            <template #cell-name="{ item }">
              <div class="min-w-0">
                <div class="font-medium font-mono">{{ item.name }}</div>
                <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
              </div>
            </template>

            <template #card-name="{ item }">
              <div>
                <div class="font-semibold font-mono text-base">{{ item.name }}</div>
                <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">{{ item.description }}</div>
              </div>
            </template>

            <!-- One version, or how many: a mixed fleet is the thing worth seeing here. -->
            <template #cell-versions="{ item }">
              <span v-if="item.versions.length > 1" class="badge badge-sm badge-warning" :title="item.versions.join('\n')">
                {{ item.versions.length }} versions
              </span>
              <span v-else class="font-mono text-xs whitespace-nowrap" :title="item.versions[0]">{{ shortVersion(item.versions[0]) || '—' }}</span>
            </template>
            <template #card-versions="{ item }">
              <span v-if="item.versions.length > 1" class="badge badge-sm badge-warning">{{ item.versions.length }} versions</span>
              <span v-else class="font-mono text-xs">{{ shortVersion(item.versions[0]) || '—' }}</span>
            </template>

            <template #cell-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }">{{ item.errors === null ? '—' : item.errors.toLocaleString() }}</span>
            </template>
            <template #card-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }">{{ item.errors === null ? '—' : item.errors.toLocaleString() }}</span>
            </template>
          </ResponsiveList>

          <ListPager
            v-if="filteredGroups.length > itemsPerPage"
            :page="currentPage"
            :total-pages="totalPages"
            :shown="paginatedGroups.length"
            :total="filteredGroups.length"
            noun="services"
            @prev="currentPage--"
            @next="currentPage++"
          />
        </BaseCard>
      </template>
    </template>
  </div>
</template>
