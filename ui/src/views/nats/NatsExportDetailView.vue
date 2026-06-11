<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import type { NatsAccountExport } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const exportRecord = ref<NatsAccountExport | null>(null)
const loading = ref(true)
const deleting = ref(false)
const exportId = route.params.id as string

async function loadExport() {
  loading.value = true
  try {
    exportRecord.value = await pb.collection('nats_account_exports').getOne<NatsAccountExport>(exportId)
  } catch (err: any) {
    toast.error('Failed to load export')
    router.push('/nats/exports')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!exportRecord.value) return
  const confirmed = await confirm({
    title: 'Delete Export',
    message: `Are you sure you want to delete "${exportRecord.value.name}"?`,
    details: 'Accounts importing this subject will lose access.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('nats_account_exports').delete(exportRecord.value.id)
    toast.success('Export deleted')
    router.push('/nats/exports')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
  }
}

onMounted(loadExport)
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="exportRecord">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/exports">NATS Exports</router-link></li>
            <li class="truncate max-w-[200px]">{{ exportRecord.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ exportRecord.name }}</h1>
            <span
              class="badge"
              :class="exportRecord.type === 'stream' ? 'badge-info' : 'badge-success'"
            >
              {{ exportRecord.type }}
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/exports/${exportRecord.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial" :disabled="deleting">Delete</button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Subject</dt>
                <dd class="mt-1"><code class="text-sm">{{ exportRecord.subject }}</code></dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ exportRecord.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(exportRecord.created) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
                <dd class="mt-1 text-sm">{{ formatDate(exportRecord.updated) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Options">
            <dl class="space-y-3">
              <div>
                <dt class="text-xs text-base-content/70">Token Required</dt>
                <dd class="mt-1">
                  <span class="badge badge-sm" :class="exportRecord.token_req ? 'badge-warning' : 'badge-ghost'">
                    {{ exportRecord.token_req ? 'Yes' : 'No' }}
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Advertise</dt>
                <dd class="mt-1">
                  <span class="badge badge-sm" :class="exportRecord.advertise ? 'badge-primary' : 'badge-ghost'">
                    {{ exportRecord.advertise ? 'Yes' : 'No' }}
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Allow Trace</dt>
                <dd class="mt-1">
                  <span class="badge badge-sm" :class="exportRecord.allow_trace ? 'badge-primary' : 'badge-ghost'">
                    {{ exportRecord.allow_trace ? 'Yes' : 'No' }}
                  </span>
                </dd>
              </div>
              <div v-if="exportRecord.account_token_position != null && exportRecord.account_token_position > 0">
                <dt class="text-xs text-base-content/70">Account Token Position</dt>
                <dd class="font-mono text-sm font-medium">{{ exportRecord.account_token_position }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard v-if="exportRecord.type === 'service'" title="Service Response">
            <dl class="space-y-3">
              <div>
                <dt class="text-xs text-base-content/70">Response Type</dt>
                <dd class="mt-1">
                  <span v-if="exportRecord.response_type" class="badge badge-sm badge-outline">
                    {{ exportRecord.response_type }}
                  </span>
                  <span v-else class="text-sm text-base-content/40">Not set</span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Response Threshold</dt>
                <dd class="font-mono text-sm font-medium">
                  {{ exportRecord.response_threshold ? exportRecord.response_threshold + 'ms' : 'Not set' }}
                </dd>
              </div>
            </dl>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
