<!-- ui/src/components/nats/LiveStateCard.vue -->
<script setup lang="ts">
/**
 * Live State for one Thing or Location: the digital twin, presented as what an
 * operator actually needs rather than as a key-value browser.
 *
 * Two sections because there are two buckets, and the split is the point:
 *
 *   REPORTED (`twin`)         written by the device — READ-ONLY here. The edge
 *                             overwrites it, so an edit button would be a lie.
 *   DESIRED  (`twin_desired`) written by operators — the writable half, and the
 *                             actual control surface.
 *
 * Freshness in the header comes from the newest reported value rather than a
 * leaf-node heartbeat lookup. It answers the question an operator is really
 * asking ("is this data current?") without depending on resolving a Thing to a
 * site, which is ambiguous with nested locations and meaningless for hub-only
 * organizations.
 *
 * The raw, fully general key browser still lives at KvDashboard — this component
 * is deliberately opinionated and hides the bucket names and key prefixes.
 */
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate, formatRelativeTime } from '@/utils/format'
import {
  TWIN_BUCKET,
  TWIN_DESIRED_BUCKET,
  twinPrefix,
  twinPropName,
  type TwinKind,
} from '@/utils/twin'
import BaseCard from '@/components/ui/BaseCard.vue'

const props = defineProps<{
  kind: TwinKind
  code: string
}>()

const natsStore = useNatsStore()
const toast = useToast()
const { confirm } = useConfirm()

// useNatsKv captures its baseKey at call time, so callers must remount this
// component when the code changes (`:key="thing.code"`), the same contract
// KvDashboard has.
const prefix = twinPrefix(props.kind, props.code)

const {
  entries: reportedEntries,
  loading: reportedLoading,
  exists: reportedExists,
  init: initReported,
  createBucket: createReportedBucket,
  getHistory: getReportedHistory,
} = useNatsKv(TWIN_BUCKET, prefix)

const {
  entries: desiredEntries,
  loading: desiredLoading,
  exists: desiredExists,
  init: initDesired,
  createBucket: createDesiredBucket,
  put: putDesired,
  del: delDesired,
} = useNatsKv(TWIN_DESIRED_BUCKET, prefix)

interface Row {
  key: string
  prop: string
  value: any
  revision: number
  created: Date
}

function toRows(entries: Map<string, KvEntry>): Row[] {
  return Array.from(entries.values())
    .map((e) => ({
      key: e.key,
      prop: twinPropName(e.key, prefix),
      value: e.value,
      revision: e.revision,
      created: e.created,
    }))
    .sort((a, b) => a.prop.localeCompare(b.prop))
}

const reportedRows = computed(() => toRows(reportedEntries.value))
const desiredRows = computed(() => toRows(desiredEntries.value))

/** Newest reported timestamp — the freshness signal in the header. */
const lastReported = computed<Date | null>(() => {
  let newest: Date | null = null
  for (const r of reportedRows.value) {
    if (!newest || r.created > newest) newest = r.created
  }
  return newest
})

const loading = computed(() => reportedLoading.value || desiredLoading.value)
const bucketsMissing = computed(
  () => !loading.value && !reportedExists.value && !desiredExists.value,
)

function displayValue(v: any): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

// --- desired-state editing ---------------------------------------------------

const editorOpen = ref(false)
const editingProp = ref('')
const editingIsNew = ref(false)
const editorValue = ref('')
const saving = ref(false)

function openNew() {
  editingIsNew.value = true
  editingProp.value = ''
  editorValue.value = ''
  editorOpen.value = true
}

function openEdit(row: Row) {
  editingIsNew.value = false
  editingProp.value = row.prop
  editorValue.value =
    typeof row.value === 'object' ? JSON.stringify(row.value, null, 2) : String(row.value ?? '')
  editorOpen.value = true
}

/**
 * Values are stored as JSON. A bare word like `on` is not valid JSON, and
 * demanding quotes around it would be a poor trade for the common case, so an
 * unparseable value is stored as a string.
 */
function parseValue(raw: string): any {
  const t = raw.trim()
  if (t === '') return ''
  try {
    return JSON.parse(t)
  } catch {
    return raw
  }
}

async function save() {
  const prop = editingProp.value.trim()
  if (!prop) {
    toast.error('Property name is required')
    return
  }
  saving.value = true
  try {
    await putDesired(prop, parseValue(editorValue.value))
    toast.success(`Set ${prop}`)
    editorOpen.value = false
  } catch {
    // useNatsKv already surfaced the failure
  } finally {
    saving.value = false
  }
}

async function clearDesired(row: Row) {
  const ok = await confirm({
    title: 'Clear desired value?',
    message: `Remove "${row.prop}". The device keeps its last received value until something sets a new one.`,
    confirmText: 'Clear',
  })
  if (ok) await delDesired(row.key)
}

// --- revision history --------------------------------------------------------

const historyOpen = ref(false)
const historyProp = ref('')
const historyRows = ref<KvEntry[]>([])
const historyLoading = ref(false)

async function openHistory(row: Row) {
  historyProp.value = row.prop
  historyOpen.value = true
  historyLoading.value = true
  try {
    historyRows.value = await getReportedHistory(row.key)
  } finally {
    historyLoading.value = false
  }
}

// --- lifecycle ---------------------------------------------------------------

async function initAll() {
  if (!natsStore.isConnected) return
  await Promise.all([initReported(), initDesired()])
}

async function initializeBuckets() {
  await createReportedBucket()
  await createDesiredBucket()
}

onMounted(initAll)
watch(() => natsStore.isConnected, initAll)
</script>

<template>
  <BaseCard title="Live State" :no-padding="true" class="w-full overflow-hidden">
    <template #actions>
      <span v-if="lastReported" class="text-xs text-base-content/70">
        updated {{ formatRelativeTime(lastReported) }}
      </span>
    </template>

    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Neither bucket exists: the platform server cannot create them (it holds
         the NATS operator, not the org's account), so this is the console's job
         or leaf-sync's. -->
    <div v-else-if="bucketsMissing" class="text-center py-12 px-4">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">Live State not initialized</h3>
      <p class="text-sm text-base-content/70 mt-2 mb-4">
        This organization has no
        <code class="font-mono text-xs">{{ TWIN_BUCKET }}</code> or
        <code class="font-mono text-xs">{{ TWIN_DESIRED_BUCKET }}</code> bucket yet.
      </p>
      <button @click="initializeBuckets" class="btn btn-primary btn-sm">Initialize</button>
    </div>

    <div v-else class="divide-y divide-base-300">
      <!-- REPORTED — read-only. The edge owns these. -->
      <section class="p-4">
        <div class="flex items-baseline justify-between mb-3">
          <div>
            <h3 class="text-xs font-bold uppercase tracking-widest">Reported</h3>
            <p class="text-xs text-base-content/70">from the device</p>
          </div>
          <span class="badge badge-ghost badge-sm">read-only</span>
        </div>

        <p v-if="!reportedRows.length" class="text-sm text-base-content/70 py-2">
          Nothing reported yet.
        </p>

        <div v-else class="overflow-x-auto">
          <table class="table table-sm">
            <tbody>
              <tr v-for="row in reportedRows" :key="row.key">
                <td class="font-mono text-xs whitespace-nowrap">{{ row.prop }}</td>
                <td class="font-mono text-sm break-all">{{ displayValue(row.value) }}</td>
                <td class="text-xs text-base-content/70 whitespace-nowrap">
                  {{ formatRelativeTime(row.created) }}
                </td>
                <td class="text-right whitespace-nowrap">
                  <button class="btn btn-xs btn-ghost" @click="openHistory(row)">History</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- DESIRED — the writable half. -->
      <section class="p-4">
        <div class="flex items-baseline justify-between mb-3">
          <div>
            <h3 class="text-xs font-bold uppercase tracking-widest">Desired</h3>
            <p class="text-xs text-base-content/70">set by you</p>
          </div>
          <button class="btn btn-xs btn-primary" @click="openNew">+ Set value</button>
        </div>

        <p v-if="!desiredRows.length" class="text-sm text-base-content/70 py-2">
          No desired values set.
        </p>

        <div v-else class="overflow-x-auto">
          <table class="table table-sm">
            <tbody>
              <tr v-for="row in desiredRows" :key="row.key">
                <td class="font-mono text-xs whitespace-nowrap">{{ row.prop }}</td>
                <td class="font-mono text-sm break-all">{{ displayValue(row.value) }}</td>
                <td class="text-xs text-base-content/70 whitespace-nowrap">
                  {{ formatRelativeTime(row.created) }}
                </td>
                <td class="text-right whitespace-nowrap">
                  <button class="btn btn-xs btn-ghost" @click="openEdit(row)">Edit</button>
                  <button class="btn btn-xs btn-ghost text-error" @click="clearDesired(row)">
                    Clear
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <!-- Desired-value editor -->
    <dialog class="modal" :class="{ 'modal-open': editorOpen }">
      <div class="modal-box">
        <h3 class="font-bold text-lg">
          {{ editingIsNew ? 'Set desired value' : `Edit ${editingProp}` }}
        </h3>

        <div class="form-control mt-4">
          <label class="label"><span class="label-text">Property</span></label>
          <input
            v-model="editingProp"
            :disabled="!editingIsNew"
            class="input input-bordered w-full font-mono text-sm"
            placeholder="setpoint"
          />
        </div>

        <div class="form-control mt-3">
          <label class="label">
            <span class="label-text">Value</span>
            <span class="label-text-alt text-base-content/70">JSON, or plain text</span>
          </label>
          <textarea
            v-model="editorValue"
            class="textarea textarea-bordered w-full font-mono text-sm"
            rows="4"
            placeholder="23"
          ></textarea>
        </div>

        <div class="modal-action">
          <button class="btn btn-ghost" @click="editorOpen = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving" @click="save">
            <span v-if="saving" class="loading loading-spinner loading-xs"></span>
            Save
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="editorOpen = false"><button>close</button></form>
    </dialog>

    <!-- Revision history -->
    <dialog class="modal" :class="{ 'modal-open': historyOpen }">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg">History — {{ historyProp }}</h3>

        <div v-if="historyLoading" class="flex justify-center p-8">
          <span class="loading loading-spinner"></span>
        </div>
        <p v-else-if="!historyRows.length" class="text-sm text-base-content/70 py-4">
          No revisions retained.
        </p>
        <div v-else class="overflow-x-auto mt-4 max-h-96">
          <table class="table table-sm table-pin-rows">
            <thead>
              <tr>
                <th>Rev</th>
                <th>Value</th>
                <th>When</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="rev in historyRows" :key="rev.revision">
                <td class="font-mono text-xs">#{{ rev.revision }}</td>
                <td class="font-mono text-xs break-all">
                  {{ rev.operation === 'PUT' ? displayValue(rev.value) : rev.operation }}
                </td>
                <td class="text-xs text-base-content/70 whitespace-nowrap">
                  {{ formatDate(rev.created) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="modal-action">
          <button class="btn btn-ghost" @click="historyOpen = false">Close</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="historyOpen = false"><button>close</button></form>
    </dialog>
  </BaseCard>
</template>
