<script setup lang="ts">
import { SearchX, Server } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import { fetchStaleHostIds } from '@/utils/nebula'
import type { NebulaHost } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import ExpiryBadge from '@/components/common/ExpiryBadge.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'

const router = useRouter()

const {
  items: hosts,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<NebulaHost>('nebula_hosts', 20)

// Search runs on the server, across all records, not the page on screen.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['hostname', 'overlay_ip', 'public_host_port', 'network_id.name'],
  () => {
    page.value = 1
    loadHosts()
  },
)

// PocketBase sort syntax ('name', '-created').
const sort = ref('')

// Pass this to EVERY usePagination call, the pager buttons included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
  expand: 'network_id',
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1 // a new order makes the old page number meaningless
  loadHosts()
}

/**
 * Hosts whose certificate does not match their network (the /32 defect, or an
 * overlay_ip edited since). Loaded once, not per page. Shown here because the
 * host otherwise looks entirely healthy.
 */
const staleHostIds = ref<Set<string>>(new Set())

async function loadStaleHosts() {
  staleHostIds.value = await fetchStaleHostIds()
}

const columns: Column<NebulaHost>[] = [
  {
    key: 'hostname',
    sortable: 'hostname',
    label: 'Hostname',
    mobileLabel: 'Hostname',
  },
  {
    key: 'overlay_ip',
    label: 'Overlay IP',
    mobileLabel: 'IP',
  },
  {
    key: 'expand.network_id.name',
    cardWide: true,
    sortable: 'network_id.name',
    label: 'Network',
    mobileLabel: 'Network',
  },
  {
    key: 'is_lighthouse',
    label: 'Type',
    mobileLabel: 'Type',
    format: (value) => value ? 'Lighthouse' : 'Node',
  },
  {
    key: 'active',
    label: 'Status',
    mobileLabel: 'Status',
    format: (value) => value ? 'Active' : 'Inactive',
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

async function loadHosts() {
  await load(queryOptions.value)
}

function handleRowClick(host: NebulaHost) {
  router.push(`/nebula/hosts/${host.id}`)
}

onMounted(() => {
  loadHosts()
  loadStaleHosts()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Nebula Hosts</h1>
        <p class="text-base-content/70 mt-1">
          Manage Nebula VPN hosts and lighthouses
        </p>
      </div>
      <router-link to="/nebula/hosts/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Host</span>
      </router-link>
    </div>
    
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by hostname, IP, endpoint, or network..."
        class="input input-bordered w-full"
      />
    </div>
    
    <div v-if="loading && hosts.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error && hosts.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load hosts</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadHosts" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <BaseCard v-else-if="hosts.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Server :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No Nebula hosts found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first Nebula host to get started
        </p>
        <router-link to="/nebula/hosts/new" class="btn btn-primary mt-4">
          Create Host
        </router-link>
      </div>
    </BaseCard>
    
    <BaseCard v-else-if="hosts.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching hosts</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :sort="sort"
        @update:sort="onSort"
        :items="hosts" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-hostname="{ item }">
          <div>
            <div class="font-medium font-mono">
              {{ item.hostname }}
            </div>
            <div v-if="item.public_host_port" class="text-sm text-base-content/60">
              {{ item.public_host_port }}
            </div>
          </div>
        </template>
        
        <template #card-hostname="{ item }">
          <div>
            <div class="font-semibold font-mono text-base">
              {{ item.hostname }}
            </div>
            <div v-if="item.public_host_port" class="text-sm text-base-content/60 mt-1 line-clamp-2">
              {{ item.public_host_port }}
            </div>
          </div>
        </template>
        
        <template #cell-overlay_ip="{ item }">
          <code class="text-sm">{{ item.overlay_ip }}</code>
        </template>
        
        <!-- Lighthouse and relay are independent; "Node" means neither. -->
        <template #cell-is_lighthouse="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <span v-if="item.is_lighthouse" class="badge badge-sm badge-primary">Lighthouse</span>
            <span v-if="item.is_relay" class="badge badge-sm badge-secondary">Relay</span>
            <span v-if="!item.is_lighthouse && !item.is_relay" class="badge badge-sm badge-ghost">Node</span>
          </div>
        </template>

        <template #card-is_lighthouse="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <span v-if="item.is_lighthouse" class="badge badge-sm badge-primary">Lighthouse</span>
            <span v-if="item.is_relay" class="badge badge-sm badge-secondary">Relay</span>
            <span v-if="!item.is_lighthouse && !item.is_relay" class="badge badge-sm badge-ghost">Node</span>
          </div>
        </template>

        <!-- Plus cert expiry: an expired cert drops the host with no other warning. -->
        <template #cell-active="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <StatusBadge :active="item.active" />
            <ExpiryBadge :value="item.expires_at" size="sm" />
            <span
              v-if="staleHostIds.has(item.id)"
              class="badge badge-sm badge-warning"
              title="This certificate's network does not match the host's network. Nebula builds the overlay route from it, so the host cannot reach any peer. Open the host to re-issue."
            >
              Wrong mask
            </span>
          </div>
        </template>

        <template #card-active="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <StatusBadge :active="item.active" />
            <ExpiryBadge :value="item.expires_at" size="sm" />
            <span
              v-if="staleHostIds.has(item.id)"
              class="badge badge-sm badge-warning"
              title="This certificate's network does not match the host's network. Open the host to re-issue."
            >
              Wrong mask
            </span>
          </div>
        </template>

        <template #actions="{ item }">
          <router-link 
            :to="`/nebula/hosts/${item.id}/edit`" 
            class="btn btn-xs"
          >
            Edit
          </router-link>
        </template>
      </ResponsiveList>
      
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="hosts.length"
        :total="totalItems"
        noun="hosts"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
