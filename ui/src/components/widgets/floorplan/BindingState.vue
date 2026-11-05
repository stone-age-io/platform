<template>
  <!--
    Renderless. It exists to own a subscription with a component's lifetime,
    which is the only way to open one per binding when the number of bindings is
    reactive: a composable cannot be called in a loop, and `useNatsKvWatcher` is
    a composable. The same shape MarkerItemText uses, one level up.
  -->
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
 * ONE subscription for every Thing of a Thing Type.
 *
 * This is the decision the floor plan widget stands on. `MapWidget` subscribes
 * per item, which is right for ten hand-placed markers; here the markers are
 * inventory and the bindings are per type, so sixty doors with two bindings
 * each would be 120 subscriptions and the easy configuration would be the
 * expensive one. Instead the parent resolves the binding's template with
 * `{thing}` wildcarded to `*` and hands it here, and each marker picks its own
 * messages out of the one buffer by its own resolved subject.
 *
 * The component emits a map keyed by subject (or KV key) rather than by Thing.
 * It does not know which Things exist -- that keeps the matching in
 * utils/floorplanBindings.ts, where it is a pure function the tests can reach.
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
  /** How many distinct markers this binding feeds, so the buffer can hold one each. */
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

/**
 * The buffer has to hold at least one message per marker or markers go dark in
 * rotation: a shared buffer sized for a single widget would evict door 1's last
 * message as soon as door 60 published. Twice the marker count leaves room for
 * a chatty device without pushing a quiet one out, and the floor is for the
 * case where the parent has not counted markers yet.
 */
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

// The config actually subscribed with, captured rather than rebuilt. Rebuilding
// it at unsubscribe time reads the NEW variable values -- the watcher below runs
// after they change -- so it unsubscribes from the new subject and leaves the
// old one feeding this binding. MarkerItemText carries the same note; it is the
// same bug in the same place.
let subscribedConfig: DataSourceConfig | null = null

function subscribe() {
  if (props.mode !== 'subject' || !natsStore.isConnected) return
  const config = buildConfig()
  if (!config) return
  dataStore.initializeBuffer(subscriptionId.value, bufferSize.value)
  // No jsonPath here: the manager would store the extracted value and discard
  // the subject with it, and the subject is how a message finds its marker.
  // The path is applied per sample below.
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
  // Last write wins per subject: the buffer is in arrival order, so walking it
  // forward leaves each subject holding its most recent message.
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
      timestamp: row.timestamp instanceof Date ? row.timestamp.getTime() : Date.now(),
    })
  }
  emit('update', out)
}

watch(buffer, () => { if (props.mode === 'subject') emitFromBuffer() }, { deep: true })
watch(rows, () => { if (props.mode === 'kv') emitFromRows() }, { deep: true, immediate: true })

onMounted(subscribe)
onUnmounted(unsubscribe)

watch(() => natsStore.isConnected, (connected) => {
  // Not unsubscribed on disconnect: the manager handles reconnection, and
  // tearing the buffer down would blank every marker on a brief wobble.
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

// Keep the buffer bound sized to the markers it has to cover.
watch(bufferSize, (size) => {
  if (props.mode === 'subject' && subscribedConfig) {
    dataStore.updateBufferConfig(subscriptionId.value, size)
  }
})
</script>

<style scoped>
.hidden { display: none; }
</style>
