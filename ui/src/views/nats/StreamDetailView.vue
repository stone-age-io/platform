<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, formatNanos } from '@/composables/useJetStreamManager'
import { useConfirm } from '@/composables/useConfirm'
import { formatBytes, formatDate } from '@/utils/format'
import type { StreamInfo } from '@nats-io/jetstream'
import type { ConsumerSummary } from '@/types/jetstream'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const natsStore = useNatsStore()
const { confirm } = useConfirm()
const {
  getStreamInfo, listConsumers, deleteStream, purgeStream,
  deleteConsumer, pauseConsumer, resumeConsumer,
} = useJetStreamManager()

const streamName = route.params.name as string

const streamInfo = ref<StreamInfo | null>(null)
const consumers = ref<ConsumerSummary[]>([])
const loading = ref(true)
const consumersLoading = ref(false)
const showPurgeModal = ref(false)
const purgeSubject = ref('')
const purging = ref(false)

async function loadData() {
  loading.value = true
  try {
    streamInfo.value = await getStreamInfo(streamName)
    await loadConsumers()
  } catch {
    router.push('/nats/streams')
  } finally {
    loading.value = false
  }
}

async function loadConsumers() {
  consumersLoading.value = true
  try {
    consumers.value = await listConsumers(streamName)
  } catch {
    // Error toasted by composable
  } finally {
    consumersLoading.value = false
  }
}

async function handleDelete() {
  const confirmed = await confirm({
    title: 'Delete Stream',
    message: `Are you sure you want to delete "${streamName}"?`,
    details: 'This will permanently delete the stream and all its messages and consumers.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  try {
    await deleteStream(streamName)
    router.push('/nats/streams')
  } catch {
    // Error toasted
  }
}

async function handlePurge() {
  purging.value = true
  try {
    const opts = purgeSubject.value.trim() ? { filter: purgeSubject.value.trim() } : undefined
    await purgeStream(streamName, opts)
    showPurgeModal.value = false
    purgeSubject.value = ''
    await loadData()
  } catch {
    // Error toasted
  } finally {
    purging.value = false
  }
}

async function handleDeleteConsumer(consumer: ConsumerSummary) {
  const confirmed = await confirm({
    title: 'Delete Consumer',
    message: `Delete consumer "${consumer.name}"?`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  try {
    await deleteConsumer(streamName, consumer.name)
    await loadConsumers()
  } catch {
    // Error toasted
  }
}

async function handlePauseConsumer(consumer: ConsumerSummary) {
  try {
    if (consumer.paused) {
      await resumeConsumer(streamName, consumer.name)
    } else {
      await pauseConsumer(streamName, consumer.name)
    }
    await loadConsumers()
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

    <template v-else-if="streamInfo">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/streams">Streams</router-link></li>
            <li class="truncate max-w-[200px] font-mono">{{ streamName }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div>
            <h1 class="text-3xl font-bold font-mono break-words">{{ streamName }}</h1>
            <p v-if="streamInfo.config.description" class="text-base-content/70 mt-1">{{ streamInfo.config.description }}</p>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/streams/${streamName}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
            <button @click="showPurgeModal = true" class="btn btn-warning flex-1 sm:flex-initial">Purge</button>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
          </div>
        </div>
      </div>

      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Configuration">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Subjects</dt>
                <dd class="mt-1 flex flex-wrap gap-1">
                  <span v-for="sub in streamInfo.config.subjects" :key="sub" class="badge badge-outline font-mono text-xs">{{ sub }}</span>
                </dd>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Retention</dt>
                  <dd class="mt-1"><span class="badge badge-sm badge-outline">{{ streamInfo.config.retention }}</span></dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Storage</dt>
                  <dd class="mt-1"><span class="badge badge-sm badge-outline">{{ streamInfo.config.storage }}</span></dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Discard</dt>
                  <dd class="mt-1"><span class="badge badge-sm badge-outline">{{ streamInfo.config.discard }}</span></dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Replicas</dt>
                  <dd class="mt-1 text-sm">{{ streamInfo.config.num_replicas }}</dd>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Messages</dt>
                  <dd class="mt-1 text-sm font-mono">{{ formatLimit(streamInfo.config.max_msgs) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Bytes</dt>
                  <dd class="mt-1 text-sm font-mono">{{ streamInfo.config.max_bytes === -1 ? 'Unlimited' : formatBytes(streamInfo.config.max_bytes) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Msg Size</dt>
                  <dd class="mt-1 text-sm font-mono">{{ streamInfo.config.max_msg_size === -1 ? 'Unlimited' : formatBytes(streamInfo.config.max_msg_size) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Age</dt>
                  <dd class="mt-1 text-sm font-mono">{{ streamInfo.config.max_age ? formatNanos(streamInfo.config.max_age) : 'Unlimited' }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Consumers</dt>
                  <dd class="mt-1 text-sm font-mono">{{ formatLimit(streamInfo.config.max_consumers) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Max Msgs/Subject</dt>
                  <dd class="mt-1 text-sm font-mono">{{ formatLimit(streamInfo.config.max_msgs_per_subject) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Duplicate Window</dt>
                  <dd class="mt-1 text-sm font-mono">{{ formatNanos(streamInfo.config.duplicate_window) }}</dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Allow Direct</dt>
                  <dd class="mt-1 text-sm">{{ streamInfo.config.allow_direct ? 'Yes' : 'No' }}</dd>
                </div>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(streamInfo.created) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="State">
            <div class="grid grid-cols-2 gap-4">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Messages</span>
                <span class="font-bold text-lg">{{ streamInfo.state.messages.toLocaleString() }}</span>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Size</span>
                <span class="font-bold text-lg">{{ formatBytes(streamInfo.state.bytes) }}</span>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Consumers</span>
                <span class="font-bold text-lg">{{ streamInfo.state.consumer_count }}</span>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Subjects</span>
                <span class="font-bold text-lg">{{ streamInfo.state.num_subjects || 0 }}</span>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-4 mt-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">First Seq</dt>
                <dd class="mt-1 text-sm font-mono">{{ streamInfo.state.first_seq }}</dd>
                <dd class="text-xs text-base-content/50">{{ streamInfo.state.first_ts ? formatDate(streamInfo.state.first_ts) : '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Last Seq</dt>
                <dd class="mt-1 text-sm font-mono">{{ streamInfo.state.last_seq }}</dd>
                <dd class="text-xs text-base-content/50">{{ streamInfo.state.last_ts ? formatDate(streamInfo.state.last_ts) : '-' }}</dd>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Consumers -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-4">
                <h3 class="card-title text-base">Consumers ({{ consumers.length }})</h3>
                <button @click="loadConsumers" class="btn btn-ghost btn-sm" :disabled="consumersLoading">
                  <span v-if="consumersLoading" class="loading loading-spinner loading-xs"></span>
                  <span v-else>Refresh</span>
                </button>
              </div>
            </template>

            <div v-if="consumersLoading && consumers.length === 0" class="flex justify-center p-8">
              <span class="loading loading-spinner loading-md"></span>
            </div>

            <div v-else-if="consumers.length === 0" class="text-center py-8">
              <span class="text-4xl">📭</span>
              <p class="text-base-content/70 mt-2">No consumers</p>
            </div>

            <div v-else class="space-y-3">
              <div
                v-for="consumer in consumers"
                :key="consumer.id"
                class="bg-base-200 rounded-lg p-3 border border-base-300"
              >
                <div class="flex justify-between items-start">
                  <div class="min-w-0 flex-1">
                    <div class="font-medium font-mono text-sm truncate">{{ consumer.name }}</div>
                    <div v-if="consumer.description" class="text-xs text-base-content/60 mt-0.5">{{ consumer.description }}</div>
                  </div>
                  <div class="flex gap-1 ml-2 flex-shrink-0">
                    <span v-if="consumer.paused" class="badge badge-sm badge-warning">Paused</span>
                    <span v-if="consumer.durableName" class="badge badge-sm badge-outline">Durable</span>
                  </div>
                </div>

                <div class="grid grid-cols-3 gap-2 mt-2 text-xs">
                  <div>
                    <span class="text-base-content/50 block">Deliver</span>
                    <span>{{ consumer.deliverPolicy }}</span>
                  </div>
                  <div>
                    <span class="text-base-content/50 block">Ack</span>
                    <span>{{ consumer.ackPolicy }}</span>
                  </div>
                  <div>
                    <span class="text-base-content/50 block">Pending</span>
                    <span>{{ consumer.numPending.toLocaleString() }}</span>
                  </div>
                </div>

                <div v-if="consumer.filterSubjects.length" class="mt-2">
                  <div class="flex flex-wrap gap-1">
                    <span v-for="f in consumer.filterSubjects" :key="f" class="badge badge-xs badge-ghost font-mono">{{ f }}</span>
                  </div>
                </div>

                <div class="flex gap-1 mt-2">
                  <button
                    @click="handlePauseConsumer(consumer)"
                    class="btn btn-xs flex-1"
                  >
                    {{ consumer.paused ? 'Resume' : 'Pause' }}
                  </button>
                  <button
                    @click="handleDeleteConsumer(consumer)"
                    class="btn btn-xs text-error flex-1"
                  >
                    Delete
                  </button>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>

    <!-- Purge Modal -->
    <dialog class="modal" :class="{ 'modal-open': showPurgeModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Purge Stream Messages</h3>
        <p class="py-4">
          This will permanently delete messages from <code class="font-mono">{{ streamName }}</code>.
          Optionally specify a subject filter to only purge messages matching that subject.
        </p>
        <div class="form-control">
          <label class="label"><span class="label-text">Subject Filter (optional)</span></label>
          <input
            v-model="purgeSubject"
            type="text"
            class="input input-bordered font-mono"
            placeholder="Leave empty to purge all messages"
          />
        </div>
        <div class="modal-action">
          <button class="btn" @click="showPurgeModal = false" :disabled="purging">Cancel</button>
          <button class="btn btn-warning" @click="handlePurge" :disabled="purging">
            <span v-if="purging" class="loading loading-spinner"></span>
            Purge
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showPurgeModal = false">close</button>
      </form>
    </dialog>
  </div>
</template>
