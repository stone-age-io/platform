<template>
  <!-- Renderless: owns one binding's subscription for the component's lifetime,
       since a composable cannot be called in a loop. -->
  <span class="hidden" aria-hidden="true" />
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useNatsKvWatcher } from '@/composables/useNatsKvWatcher'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { extractJsonPath } from '@/utils/jsonPath'
import { resolveTemplate } from '@/utils/variables'
import type { DataSourceConfig } from '@/types/dashboard'
import type { StateSample } from '@/utils/floorplanBindings'

/**
 * One subscription (or KV watcher) for every Thing of a Thing Type. The parent
 * passes the wildcarded subject; this emits the latest sample keyed by subject
 * (or KV key), and the parent matches each key to its marker.
 */

const props = defineProps<{
  /** Stable, unique per binding: it is the subscription id and the buffer key. */
  bindingId: string
  mode: 'subject' | 'kv'
  /** subject mode: the wildcarded subject covering every Thing of the type. */
  subject?: string
  /** kv mode */
  kvBucket?: string
  kvPattern?: string
  jsonPath?: string
  /** How many markers this binding feeds, so the buffer can hold one each. */
  markerCount: number
}>()

const emit = defineEmits<{
  update: [samples: Map<string, StateSample>]
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const subManager = getSubscriptionManager()

const subscriptionId = computed(() => `fp-binding-${props.bindingId}`)

// At least one message per marker, or a chatty device evicts a quiet one's
// last reading. Twice the count leaves headroom.
const bufferSize = computed(() => Math.min(1000, Math.max(50, props.markerCount * 2)))

// --- subject mode -----------------------------------------------------------

const resolvedSubject = computed(() =>
  props.mode === 'subject' && props.subject
    ? resolveTemplate(props.subject, dashboardStore.currentVariableValues)
    : ''
)

function buildConfig(): DataSourceConfig | null {
  if (!resolvedSubject.value) return null
  return { type: 'subscription', subject: resolvedSubject.value, useJetStream: false }
}

// Kept so unsubscribe uses the config actually subscribed with, not one rebuilt
// from variable values that have already changed (same as MarkerItemText).
let subscribedConfig: DataSourceConfig | null = null

function subscribe() {
  if (props.mode !== 'subject' || !natsStore.isConnected) return
  const config = buildConfig()
  if (!config) return
  dataStore.initializeBuffer(subscriptionId.value, bufferSize.value)
  // No jsonPath on the subscription: the buffer must keep each message's
  // subject, which is how it finds its marker. The path is applied below.
  subManager.subscribe(subscriptionId.value, config)
  subscribedConfig = config
}

function unsubscribe() {
  if (subscribedConfig) subManager.unsubscribe(subscriptionId.value, subscribedConfig)
  subscribedConfig = null
  dataStore.removeBuffer(subscriptionId.value)
}

const buffer = computed(() =>
  props.mode === 'subject' ? dataStore.getBuffer(subscriptionId.value) : []
)

// --- kv mode ----------------------------------------------------------------

const kvBucket = computed(() => (props.mode === 'kv' ? props.kvBucket || '' : ''))
const kvPattern = computed(() => (props.mode === 'kv' ? props.kvPattern || '' : ''))

const { rows } = useNatsKvWatcher(kvBucket, kvPattern)

// --- emit -------------------------------------------------------------------

function emitFromBuffer() {
  const out = new Map<string, StateSample>()
  // The buffer is in arrival order, so the last write per subject wins.
  for (const msg of buffer.value) {
    if (!msg.subject) continue
    out.set(msg.subject, {
      value: extractJsonPath(msg.raw ?? msg.value, props.jsonPath),
      timestamp: msg.timestamp,
    })
  }
  emit('update', out)
}

function emitFromRows() {
  const out = new Map<string, StateSample>()
  for (const [key, row] of rows.value) {
    out.set(key, {
      value: extractJsonPath(row.data, props.jsonPath),
      timestamp: row.timestamp.getTime(),
    })
  }
  emit('update', out)
}

watch(buffer, () => { if (props.mode === 'subject') emitFromBuffer() }, { deep: true })
watch(rows, () => { if (props.mode === 'kv') emitFromRows() }, { deep: true, immediate: true })

onMounted(subscribe)
onUnmounted(unsubscribe)

watch(() => natsStore.isConnected, (connected) => {
  // Not unsubscribed on disconnect: the manager reconnects, and dropping the
  // buffer would blank every marker on a brief wobble.
  if (connected) subscribe()
})

watch(() => dashboardStore.currentVariableValues, () => {
  if (props.mode === 'subject' && natsStore.isConnected) {
    unsubscribe()
    subscribe()
  }
}, { deep: true })

watch(() => [props.mode, props.subject, props.bindingId], () => {
  unsubscribe()
  subscribe()
})

watch(bufferSize, (size) => {
  if (props.mode === 'subject' && subscribedConfig) {
    dataStore.updateBufferConfig(subscriptionId.value, size)
  }
})
</script>

<style scoped>
.hidden { display: none; }
</style>
