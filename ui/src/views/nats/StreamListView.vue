<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager } from '@/composables/useJetStreamManager'
import { formatBytes } from '@/utils/format'
import { sortBy } from '@/utils/clientSort'
import type { StreamSummary, JetStreamAccountSummary } from '@/types/jetstream'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

const router = useRouter()
const natsStore = useNatsStore()
const { listStreams, getAccountInfo } = useJetStreamManager()

const streams = ref<StreamSummary[]>([])
const accountInfo = ref<JetStreamAccountSummary | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const searchQuery = ref('')

// Alphabetical by default. JetStream answers in whatever order it holds them,
// which is stable enough to look deliberate and arbitrary enough to be useless.
const sort = ref('name')

// Client-side search
const filteredStreams = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return streams.value
  return streams.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.description.toLowerCase().includes(q) ||
    s.subjects.some(sub => sub.toLowerCase().includes(q))
  )
})

// Client-side sort, over the filtered set and BEFORE the slice. Sorting
// paginatedStreams instead would order rows that were already the wrong twenty
// -- on page one as much as page two -- and hide it completely on any account
// whose streams fit on a single page. Safe here only because listStreams()
// returns the whole account: see utils/clientSort.
const sortedStreams = computed(() => sortBy(filteredStreams.value, sort.value))

// Client-side pagination
const currentPage = ref(1)
const itemsPerPage = 20
const totalPages = computed(() => Math.max(1, Math.ceil(filteredStreams.value.length / itemsPerPage)))
const paginatedStreams = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return sortedStreams.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })

function onSort(next: string) {
  sort.value = next
  currentPage.value = 1 // a new order makes the old page number meaningless
}

// Subjects is the one column left un-widthed, so it absorbs the slack. It is
// the only column here whose content has no bound -- a stream can carry one
// subject or a dozen -- while retention, message count, size and consumer count
// are all short and known. They used to split the leftover width equally, which
// gave a one-digit consumer count the same 226px as a subject list and left the
// subject list truncating at 200px on a 1570px table.
//
// The 200px was a hardcoded max-w on the cell, which is why widening the column
// alone would have changed nothing. A cap inside a fixed-layout column is a
// second opinion about width that cannot agree with the first; the truncate and
// its title tooltip stay, and they now bite at the column edge.
//
// A leading `-` means "descending on the first click", the same convention the
// dated lists use. For a size or a count that is the end worth opening on: the
// question these columns get asked is which stream is eating the disk, never
// which one is emptiest.
const columns: Column<StreamSummary>[] = [
  { key: 'name', sortable: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'subjects', sortable: 'subjects', label: 'Subjects', mobileLabel: 'Subjects', format: (v: string[]) => v.join(', ') },
  { key: 'retention', sortable: 'retention', width: '7rem', label: 'Retention', mobileLabel: 'Retention' },
  { key: 'messages', sortable: '-messages', width: '9rem', label: 'Messages', mobileLabel: 'Msgs', format: (v: number) => v.toLocaleString() },
  { key: 'bytes', sortable: '-bytes', width: '7rem', label: 'Size', mobileLabel: 'Size', format: (v: number) => formatBytes(v) },
  { key: 'consumers', sortable: '-consumers', width: '7rem', label: 'Consumers', mobileLabel: 'Cons' },
]

async function loadData() {
  if (!natsStore.isConnected) {
    loading.value = false
    return
  }
  loading.value = true
  error.value = null
  try {
    const [streamList, acctInfo] = await Promise.all([
      listStreams(),
      getAccountInfo(),
    ])
    streams.value = streamList
    accountInfo.value = acctInfo
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function handleRowClick(stream: StreamSummary) {
  router.push(`/nats/streams/${stream.name}`)
}

function formatLimit(used: number, limit: number): string {
  if (limit === -1) return `${formatBytes(used)} / Unlimited`
  return `${formatBytes(used)} / ${formatBytes(limit)}`
}

function usagePercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.round((used / limit) * 100))
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
        <h1 class="text-3xl font-bold">Streams</h1>
        <p class="text-base-content/70 mt-1">Manage JetStream streams</p>
      </div>
      <router-link to="/nats/streams/new" class="btn btn-primary w-full sm:w-auto" v-if="natsStore.isConnected">
        <span class="text-lg">+</span>
        <span>New Stream</span>
      </router-link>
    </div>

    <!-- Not Connected -->
    <BaseCard v-if="!natsStore.isConnected">
      <div class="text-center py-12">
        <span class="text-6xl">📡</span>
        <h3 class="text-xl font-bold mt-4">Not connected to NATS</h3>
        <p class="text-base-content/70 mt-2">Connect to NATS to manage JetStream streams.</p>
      </div>
    </BaseCard>

    <template v-else>
      <!-- Account Usage -->
      <div v-if="accountInfo" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="bg-base-200 rounded-lg p-3 border border-base-300">
          <span class="text-xs text-base-content/50 uppercase block mb-1">Disk Storage</span>
          <span class="font-medium text-sm">{{ formatLimit(accountInfo.storageUsed, accountInfo.storageLimit) }}</span>
          <progress v-if="accountInfo.storageLimit > 0" class="progress progress-primary w-full mt-1" :value="usagePercent(accountInfo.storageUsed, accountInfo.storageLimit)" max="100"></progress>
        </div>
        <div class="bg-base-200 rounded-lg p-3 border border-base-300">
          <span class="text-xs text-base-content/50 uppercase block mb-1">Memory Storage</span>
          <span class="font-medium text-sm">{{ formatLimit(accountInfo.memoryUsed, accountInfo.memoryLimit) }}</span>
          <progress v-if="accountInfo.memoryLimit > 0" class="progress progress-primary w-full mt-1" :value="usagePercent(accountInfo.memoryUsed, accountInfo.memoryLimit)" max="100"></progress>
        </div>
        <div class="bg-base-200 rounded-lg p-3 border border-base-300">
          <span class="text-xs text-base-content/50 uppercase block mb-1">Streams</span>
          <span class="font-medium text-sm">{{ accountInfo.streamCount }} {{ accountInfo.streamLimit === -1 ? '/ Unlimited' : `/ ${accountInfo.streamLimit}` }}</span>
        </div>
        <div class="bg-base-200 rounded-lg p-3 border border-base-300">
          <span class="text-xs text-base-content/50 uppercase block mb-1">Consumers</span>
          <span class="font-medium text-sm">{{ accountInfo.consumerCount }} {{ accountInfo.consumerLimit === -1 ? '/ Unlimited' : `/ ${accountInfo.consumerLimit}` }}</span>
        </div>
      </div>

      <!-- Search -->
      <div class="form-control">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by name, description, or subject..."
          class="input input-bordered w-full"
        />
      </div>

      <!-- Loading State -->
      <div v-if="loading && streams.length === 0" class="flex justify-center p-12">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <!-- Error State -->
      <BaseCard v-else-if="error && streams.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">&#9888;</span>
          <h3 class="text-xl font-bold mt-4">Failed to load streams</h3>
          <p class="text-base-content/70 mt-2">{{ error }}</p>
          <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
        </div>
      </BaseCard>

      <!-- Empty State -->
      <BaseCard v-else-if="streams.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">📦</span>
          <h3 class="text-xl font-bold mt-4">No streams found</h3>
          <p class="text-base-content/70 mt-2">Create your first JetStream stream to get started.</p>
          <router-link to="/nats/streams/new" class="btn btn-primary mt-4">Create Stream</router-link>
        </div>
      </BaseCard>

      <!-- No Search Results -->
      <BaseCard v-else-if="filteredStreams.length === 0">
        <div class="text-center py-12">
          <span class="text-6xl">🔍</span>
          <h3 class="text-xl font-bold mt-4">No matching streams</h3>
          <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
        </div>
      </BaseCard>

      <!-- Stream List -->
      <BaseCard v-else :no-padding="true">
        <ResponsiveList
          :items="paginatedStreams"
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
              <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">{{ item.description }}</div>
            </div>
          </template>

          <template #cell-subjects="{ item }">
            <div class="font-mono text-xs truncate" :title="item.subjects.join(', ')">
              {{ item.subjects.join(', ') }}
            </div>
          </template>

          <template #cell-retention="{ item }">
            <span class="badge badge-sm badge-outline">{{ item.retention }}</span>
          </template>

          <template #card-retention="{ item }">
            <span class="badge badge-sm badge-outline">{{ item.retention }}</span>
          </template>
        </ResponsiveList>

        <!-- Pagination -->
        <ListPager
          v-if="filteredStreams.length > itemsPerPage"
          :page="currentPage"
          :total-pages="totalPages"
          :shown="paginatedStreams.length"
          :total="filteredStreams.length"
          noun="streams"
          @prev="currentPage--"
          @next="currentPage++"
        />
      </BaseCard>
    </template>
  </div>
</template>
