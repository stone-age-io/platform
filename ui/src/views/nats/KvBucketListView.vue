<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, formatNanos } from '@/composables/useJetStreamManager'
import { formatBytes } from '@/utils/format'
import { sortBy } from '@/utils/clientSort'
import type { KvBucketSummary } from '@/types/jetstream'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

const router = useRouter()
const natsStore = useNatsStore()
const { listKvBuckets } = useJetStreamManager()

const buckets = ref<KvBucketSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const searchQuery = ref('')

// Alphabetical by default. JetStream answers in whatever order it holds them,
// which is stable enough to look deliberate and arbitrary enough to be useless.
const sort = ref('name')

const filteredBuckets = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return buckets.value
  return buckets.value.filter(b =>
    b.name.toLowerCase().includes(q) ||
    b.description.toLowerCase().includes(q)
  )
})

// Client-side sort, over the filtered set and BEFORE the slice -- sorting
// paginatedBuckets would order rows that were already the wrong twenty, on page
// one as much as page two, and hide it entirely on any account whose buckets fit
// on a single page. Safe here only because listKvBuckets() returns the whole
// account: see utils/clientSort.
const sortedBuckets = computed(() => sortBy(filteredBuckets.value, sort.value))

const currentPage = ref(1)
const itemsPerPage = 20
const totalPages = computed(() => Math.max(1, Math.ceil(filteredBuckets.value.length / itemsPerPage)))
const paginatedBuckets = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return sortedBuckets.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })

function onSort(next: string) {
  sort.value = next
  currentPage.value = 1 // a new order makes the old page number meaningless
}

const columns: Column<KvBucketSummary>[] = [
  // 'auto', not omitted: omitting it means 28% for column 0 (see Column.width).
  // This is the only free-text column here -- name plus a clamped description --
  // so it takes the slack and every other column keeps exactly what it declares.
  { key: 'name', sortable: 'name', width: 'auto', label: 'Name', mobileLabel: 'Name' },
  // `-` on the three "how big" columns so the first click opens on the end
  // worth looking at. TTL keeps plain ascending: 0 renders as "None", and
  // descending would open on the buckets that never expire, which is the least
  // interesting answer the column has.
  { key: 'values', sortable: '-values', width: '8rem', label: 'Keys', mobileLabel: 'Keys', format: (v: number) => v.toLocaleString() },
  { key: 'bytes', sortable: '-bytes', width: '8rem', label: 'Size', mobileLabel: 'Size', format: (v: number) => formatBytes(v) },
  { key: 'history', sortable: '-history', width: '7rem', label: 'History', mobileLabel: 'History' },
  { key: 'storage', sortable: 'storage', width: '7rem', label: 'Storage', mobileLabel: 'Storage' },
  { key: 'ttl', sortable: 'ttl', width: '8rem', label: 'TTL', mobileLabel: 'TTL', format: (v: number) => v ? formatNanos(v) : 'None' },
]

async function loadData() {
  if (!natsStore.isConnected) {
    loading.value = false
    return
  }
  loading.value = true
  error.value = null
  try {
    buckets.value = await listKvBuckets()
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function handleRowClick(bucket: KvBucketSummary) {
  router.push(`/nats/kv/${bucket.name}`)
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
        <h1 class="text-3xl font-bold">KV Buckets</h1>
        <p class="text-base-content/70 mt-1">Manage NATS Key-Value stores</p>
      </div>
      <router-link to="/nats/kv/new" class="btn btn-primary w-full sm:w-auto" v-if="natsStore.isConnected">
        <span class="text-lg">+</span>
        <span>New Bucket</span>
      </router-link>
    </div>

    <!-- Not Connected -->
    <BaseCard v-if="!natsStore.isConnected">
      <div class="text-center py-12">
        <span class="text-6xl">📡</span>
        <h3 class="text-xl font-bold mt-4">Not connected to NATS</h3>
        <p class="text-base-content/70 mt-2">Connect to NATS to manage KV buckets.</p>
      </div>
    </BaseCard>

    <template v-else>
      <!-- Search -->
      <div class="form-control">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by name or description..."
          class="input input-bordered w-full"
        />
      </div>

      <!-- Loading -->
      <div v-if="loading && buckets.length === 0" class="flex justify-center p-12">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <!-- Error -->
      <BaseCard v-else-if="error && buckets.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">&#9888;</span>
          <h3 class="text-xl font-bold mt-4">Failed to load KV buckets</h3>
          <p class="text-base-content/70 mt-2">{{ error }}</p>
          <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
        </div>
      </BaseCard>

      <!-- Empty -->
      <BaseCard v-else-if="buckets.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">🗄️</span>
          <h3 class="text-xl font-bold mt-4">No KV buckets found</h3>
          <p class="text-base-content/70 mt-2">Create your first KV bucket to get started.</p>
          <router-link to="/nats/kv/new" class="btn btn-primary mt-4">Create Bucket</router-link>
        </div>
      </BaseCard>

      <!-- No Results -->
      <BaseCard v-else-if="filteredBuckets.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">🔍</span>
          <h3 class="text-xl font-bold mt-4">No matching buckets</h3>
          <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
        </div>
      </BaseCard>

      <!-- Bucket List -->
      <BaseCard v-else :no-padding="true">
        <ResponsiveList
          :items="paginatedBuckets"
          :columns="columns"
          :loading="loading"
          :sort="sort"
          @update:sort="onSort"
          @row-click="handleRowClick"
        >
          <template #cell-name="{ item }">
            <div>
              <div class="font-medium font-mono">{{ item.name }}</div>
              <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
            </div>
          </template>

          <template #card-name="{ item }">
            <div>
              <div class="font-semibold font-mono text-base">{{ item.name }}</div>
              <div v-if="item.description" class="text-sm text-base-content/60 mt-1">{{ item.description }}</div>
            </div>
          </template>

          <template #cell-storage="{ item }">
            <span class="badge badge-sm badge-outline">{{ item.storage }}</span>
          </template>

          <template #card-storage="{ item }">
            <span class="badge badge-sm badge-outline">{{ item.storage }}</span>
          </template>
        </ResponsiveList>

        <!-- Pagination -->
        <ListPager
          v-if="filteredBuckets.length > itemsPerPage"
          :page="currentPage"
          :total-pages="totalPages"
          :shown="paginatedBuckets.length"
          :total="filteredBuckets.length"
          noun="buckets"
          @prev="currentPage--"
          @next="currentPage++"
        />
      </BaseCard>
    </template>
  </div>
</template>
