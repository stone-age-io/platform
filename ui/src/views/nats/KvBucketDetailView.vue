<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, formatNanos } from '@/composables/useJetStreamManager'
import { useConfirm } from '@/composables/useConfirm'
import { formatBytes, formatDate } from '@/utils/format'
import type { KvStatus } from '@nats-io/kv'
import BaseCard from '@/components/ui/BaseCard.vue'
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
          <div>
            <h1 class="text-3xl font-bold font-mono break-words">{{ bucketName }}</h1>
            <p v-if="status.description" class="text-base-content/70 mt-1">{{ status.description }}</p>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
          </div>
        </div>
      </div>

      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
        <!-- Left Column: Config & Status -->
        <div class="space-y-6">
          <BaseCard title="Configuration">
            <dl class="space-y-4">
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Storage</dt>
                  <dd class="mt-1"><span class="badge badge-sm badge-outline">{{ status.storage }}</span></dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Replicas</dt>
                  <dd class="mt-1 text-sm">{{ status.replicas }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">History</dt>
                  <dd class="mt-1 text-sm">{{ status.history }} revisions</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Bytes</dt>
                  <dd class="mt-1 text-sm font-mono">{{ status.max_bytes === -1 ? 'Unlimited' : formatBytes(status.max_bytes) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Value Size</dt>
                  <dd class="mt-1 text-sm font-mono">{{ formatLimit(status.maxValueSize) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">TTL</dt>
                  <dd class="mt-1 text-sm font-mono">{{ status.ttl ? formatNanos(status.ttl * 1000000) : 'None' }}</dd>
                </div>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(status.streamInfo.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Status">
            <div class="grid grid-cols-2 gap-4">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Keys</span>
                <span class="font-bold text-lg">{{ status.values.toLocaleString() }}</span>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Size</span>
                <span class="font-bold text-lg">{{ formatBytes(status.size) }}</span>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Key Browser (takes 2 cols on lg) -->
        <div class="lg:col-span-2">
          <KvDashboard :bucket="bucketName" />
        </div>
      </div>
    </template>
  </div>
</template>
