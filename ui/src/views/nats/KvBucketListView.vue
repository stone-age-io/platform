<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, formatNanos } from '@/composables/useJetStreamManager'
import { formatBytes } from '@/utils/format'
import type { KvBucketSummary } from '@/types/jetstream'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const natsStore = useNatsStore()
const { listKvBuckets } = useJetStreamManager()

const buckets = ref<KvBucketSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const searchQuery = ref('')

const filteredBuckets = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return buckets.value
  return buckets.value.filter(b =>
    b.name.toLowerCase().includes(q) ||
    b.description.toLowerCase().includes(q)
  )
})

const currentPage = ref(1)
const itemsPerPage = 20
const totalPages = computed(() => Math.max(1, Math.ceil(filteredBuckets.value.length / itemsPerPage)))
const paginatedBuckets = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return filteredBuckets.value.slice(start, start + itemsPerPage)
})

watch(searchQuery, () => { currentPage.value = 1 })

const columns: Column<KvBucketSummary>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'values', label: 'Keys', mobileLabel: 'Keys', format: (v: number) => v.toLocaleString() },
  { key: 'bytes', label: 'Size', mobileLabel: 'Size', format: (v: number) => formatBytes(v) },
  { key: 'history', label: 'History', mobileLabel: 'History' },
  { key: 'storage', label: 'Storage', mobileLabel: 'Storage' },
  { key: 'ttl', label: 'TTL', mobileLabel: 'TTL', format: (v: number) => v ? formatNanos(v) : 'None' },
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
          @row-click="handleRowClick"
        >
          <template #cell-name="{ item }">
            <div>
              <div class="font-medium font-mono">{{ item.name }}</div>
              <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">{{ item.description }}</div>
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
        <div v-if="filteredBuckets.length > itemsPerPage" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
          <span class="text-sm text-base-content/70">
            Showing {{ paginatedBuckets.length }} of {{ filteredBuckets.length }} buckets
          </span>
          <div class="join">
            <button class="join-item btn btn-sm" :disabled="currentPage === 1" @click="currentPage--">«</button>
            <button class="join-item btn btn-sm">{{ currentPage }} / {{ totalPages }}</button>
            <button class="join-item btn btn-sm" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
          </div>
        </div>
      </BaseCard>
    </template>
  </div>
</template>
