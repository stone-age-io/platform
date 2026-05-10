<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, formatNanos } from '@/composables/useJetStreamManager'
import { useConfirm } from '@/composables/useConfirm'
import { formatBytes, formatDate } from '@/utils/format'
import type { KvStatus } from '@nats-io/kv'
import KvDashboard from '@/components/nats/KvDashboard.vue'

const router = useRouter()
const route = useRoute()
const natsStore = useNatsStore()
const { confirm } = useConfirm()
const { getKvBucketStatus, deleteKvBucket } = useJetStreamManager()

const bucketName = route.params.name as string

const status = ref<KvStatus | null>(null)
const loading = ref(true)

async function loadData() {
  loading.value = true
  try {
    status.value = await getKvBucketStatus(bucketName)
  } catch {
    router.push('/nats/kv')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  const confirmed = await confirm({
    title: 'Delete KV Bucket',
    message: `Are you sure you want to delete "${bucketName}"?`,
    details: 'This will permanently delete the bucket and all its keys.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  try {
    await deleteKvBucket(bucketName)
    router.push('/nats/kv')
  } catch {
    // Error toasted
  }
}

function formatLimit(val: number): string {
  if (val === -1) return 'Unlimited'
  if (val === 0) return 'Default'
  return val.toLocaleString()
}

onMounted(() => { loadData() })

watch(() => natsStore.isConnected, (connected) => {
  if (connected) loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="status">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/kv">KV Buckets</router-link></li>
            <li class="truncate max-w-[200px] font-mono">{{ bucketName }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="min-w-0">
            <h1 class="text-3xl font-bold font-mono break-words">{{ bucketName }}</h1>
            <p v-if="status.description" class="text-base-content/70 mt-1">{{ status.description }}</p>
          </div>
          <div class="flex gap-2 w-full sm:w-auto shrink-0">
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
          </div>
        </div>
      </div>

      <!-- Stat Tiles -->
      <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-px bg-base-300 rounded-lg overflow-hidden border border-base-300">
        <div class="stat-tile">
          <span class="stat-label">Keys</span>
          <span class="stat-value">{{ status.values.toLocaleString() }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">Size</span>
          <span class="stat-value">{{ formatBytes(status.size) }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">History</span>
          <span class="stat-value">{{ status.history }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">Replicas</span>
          <span class="stat-value">{{ status.replicas }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">Storage</span>
          <span class="stat-value capitalize">{{ status.storage }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">Max Bytes</span>
          <span class="stat-value font-mono">{{ status.max_bytes === -1 ? '∞' : formatBytes(status.max_bytes) }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">Max Value</span>
          <span class="stat-value font-mono">{{ formatLimit(status.maxValueSize) }}</span>
        </div>
        <div class="stat-tile">
          <span class="stat-label">TTL</span>
          <span class="stat-value font-mono">{{ status.ttl ? formatNanos(status.ttl * 1000000) : 'None' }}</span>
        </div>
      </div>

      <p class="text-xs text-base-content/50">
        Created {{ formatDate(status.streamInfo.created) }}
      </p>

      <!-- Full-width KV Dashboard -->
      <KvDashboard :bucket="bucketName" />
    </template>
  </div>
</template>

<style scoped>
.stat-tile {
  background: oklch(var(--b1));
  padding: 0.75rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}
.stat-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: oklch(var(--bc) / 0.5);
  font-weight: 600;
}
.stat-value {
  font-size: 0.95rem;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
