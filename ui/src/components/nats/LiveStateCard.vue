<!-- ui/src/components/nats/LiveStateCard.vue -->
<script setup lang="ts">
/**
 * Live State for one Thing or Location: the digital twin, presented as what an
 * operator needs rather than as a key-value browser.
 *
 * ONE LIST OF PROPERTIES. Desired is not a peer of reported here — it is an
 * annotation on the few rows that have one, plus an action.
 *
 * The two buckets exist because bidirectional sync needs exactly one writer
 * each. That is a sync-layer concern, and earlier versions of this card let it
 * become the organizing principle of the UI — two stacked lists, then two
 * columns — which gave half the width and a peer-level heading to something
 * that is empty for most properties. Real twins are mostly telemetry
 * (`analytics`, `install-date`) with a handful of controllable settings. So a
 * property with no desired value renders as an ordinary value row and costs
 * nothing; only a desired value that DIFFERS from what the device reports earns
 * screen space, because that is the one genuinely interesting state: asked for
 * on, still reports off.
 *
 * Reported values are READ-ONLY here. The edge overwrites them, so an edit
 * button would be a lie — the value comes back on the next sync. Desired is the
 * writable half and the operator's actual control.
 *
 * The raw, fully general key browser is still KvDashboard, under NATS → KV
 * Buckets. This component hides bucket names and key prefixes, because the card
 * already knows which Thing it is on.
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
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import TwinValue from '@/components/nats/TwinValue.vue'

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
  error: reportedError,
  init: initReported,
  createBucket: createReportedBucket,
  getHistory: getReportedHistory,
} = useNatsKv(TWIN_BUCKET, prefix)

const {
  entries: desiredEntries,
  loading: desiredLoading,
  exists: desiredExists,
  error: desiredError,
  init: initDesired,
  createBucket: createDesiredBucket,
  put: putDesired,
  del: delDesired,
} = useNatsKv(TWIN_DESIRED_BUCKET, prefix)

interface PropRow {
  id: string // property name — ResponsiveList keys on this
  prop: string
  reported?: KvEntry
  desired?: KvEntry
  /** Both sides present and disagreeing: the device has not converged. */
  differs: boolean
}

const rows = computed<PropRow[]>(() => {
  const byProp = new Map<string, PropRow>()

  const upsert = (key: string, side: 'reported' | 'desired', entry: KvEntry) => {
    const prop = twinPropName(key, prefix)
    const row = byProp.get(prop) ?? { id: prop, prop, differs: false }
    row[side] = entry
    byProp.set(prop, row)
  }

  for (const e of reportedEntries.value.values()) upsert(e.key, 'reported', e)
  for (const e of desiredEntries.value.values()) upsert(e.key, 'desired', e)

  for (const row of byProp.values()) {
    row.differs =
      !!row.desired &&
      (!row.reported ||
        JSON.stringify(row.reported.value) !== JSON.stringify(row.desired.value))
  }

  return Array.from(byProp.values()).sort((a, b) => a.prop.localeCompare(b.prop))
})

const columns: Column<PropRow>[] = [
  { key: 'prop', label: 'Property', class: 'w-1/3' },
  { key: 'value', label: 'Value', mobileLabel: 'Value' },
]

const loading = computed(() => reportedLoading.value || desiredLoading.value)
const anyError = computed(() => reportedError.value || desiredError.value)

const bucketsMissing = computed(
  () => !loading.value && !reportedExists.value && !desiredExists.value,
)

/** Newest reported timestamp — the freshness signal in the header. */
const lastReported = computed<Date | null>(() => {
  let newest: Date | null = null
  for (const e of reportedEntries.value.values()) {
    if (!newest || e.created > newest) newest = e.created
  }
  return newest
})

const pendingCount = computed(() => rows.value.filter((r) => r.differs).length)

// --- detail ------------------------------------------------------------------

const detailProp = ref<string | null>(null)
const detailRow = computed(() => rows.value.find((r) => r.prop === detailProp.value) ?? null)
const historyRows = ref<KvEntry[]>([])
const historyLoading = ref(false)

async function openDetail(row: PropRow) {
  detailProp.value = row.prop
  historyRows.value = []
  if (!row.reported) return
  historyLoading.value = true
  try {
    historyRows.value = await getReportedHistory(row.reported.key)
  } finally {
    historyLoading.value = false
  }
}

// --- desired-state editing ----------------------------------------------------

const editorOpen = ref(false)
const editingProp = ref('')
const editingPropLocked = ref(false)
const editorValue = ref('')
const saving = ref(false)

/** From a row: the property is known, so don't make anyone retype it. */
function openSet(row: PropRow) {
  editingPropLocked.value = true
  editingProp.value = row.prop
  editorValue.value = row.desired
    ? toEditable(row.desired.value)
    : row.reported
      ? toEditable(row.reported.value) // seed from reported: usually a small edit
      : ''
  editorOpen.value = true
}

/** From the header: a property that may exist on neither side yet. */
function openNew() {
  editingPropLocked.value = false
  editingProp.value = ''
  editorValue.value = ''
  editorOpen.value = true
}

function toEditable(v: any): string {
  return typeof v === 'object' && v !== null ? JSON.stringify(v, null, 2) : String(v ?? '')
}

/**
 * Values are stored as JSON. A bare word like `on` is not valid JSON, and
 * demanding quotes around it would be a poor trade for the common case, so an
 * unparseable value is stored as a string. That fallback is quiet and easy to
 * get wrong, which is what `parsedPreview` below is for — it says out loud what
 * will actually be written.
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

const parsedPreview = computed<{ type: string; note: string }>(() => {
  const t = editorValue.value.trim()
  if (t === '') return { type: 'string', note: 'empty' }
  try {
    const v = JSON.parse(t)
    if (v === null) return { type: 'null', note: '' }
    if (Array.isArray(v)) return { type: 'array', note: `${v.length} items` }
    if (typeof v === 'object') return { type: 'object', note: `${Object.keys(v).length} fields` }
    return { type: typeof v, note: String(v) }
  } catch {
    return { type: 'string', note: 'not valid JSON — stored as text' }
  }
})

/** Same quick-set affordances as the KV browser's editor, plus a few. */
const quickSets: { label: string; apply: () => string }[] = [
  { label: 'true', apply: () => 'true' },
  { label: 'false', apply: () => 'false' },
  { label: 'null', apply: () => 'null' },
  { label: '0', apply: () => '0' },
  { label: '""', apply: () => '""' },
  { label: '{}', apply: () => '{\n  \n}' },
  { label: 'now()', apply: () => JSON.stringify(new Date().toISOString()) },
]

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

async function clearDesired(row: PropRow) {
  if (!row.desired) return
  const ok = await confirm({
    title: 'Clear desired value?',
    message: `Remove "${row.prop}". The device keeps its last received value until something sets a new one.`,
    confirmText: 'Clear',
    variant: 'warning',
  })
  if (ok) await delDesired(row.desired.key)
}

// --- lifecycle ----------------------------------------------------------------

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
      <div class="flex items-center gap-3">
        <span v-if="pendingCount" class="badge badge-warning badge-sm">
          {{ pendingCount }} pending
        </span>
        <span v-if="lastReported" class="text-xs text-base-content/70 hidden sm:inline">
          updated {{ formatRelativeTime(lastReported) }}
        </span>
        <button v-if="!bucketsMissing && !loading" class="btn btn-xs btn-primary" @click="openNew">
          + Set value
        </button>
      </div>
    </template>

    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Neither bucket exists. The platform server cannot create them (it holds
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

    <div v-else-if="anyError" class="text-center py-12 px-4">
      <div class="text-5xl mb-3">⚠️</div>
      <h3 class="font-bold text-lg">Failed to load Live State</h3>
      <p class="text-sm text-base-content/70 mt-2 mb-4">{{ anyError }}</p>
      <button @click="initAll" class="btn btn-sm">Retry</button>
    </div>

    <div v-else class="p-4">
      <ResponsiveList :items="rows" :columns="columns" @row-click="openDetail">
        <template #cell-prop="{ item }">
          <span class="font-mono text-sm font-semibold text-primary break-all">{{ item.prop }}</span>
        </template>

        <!--
          The value cell carries the whole story. No desired value: just the
          value. Desired that matches: a quiet marker, so Clear makes sense.
          Desired that differs: the arrow, which is the only case worth space.
        -->
        <template #cell-value="{ item }">
          <span class="inline-flex items-center gap-2 min-w-0">
            <TwinValue v-if="item.reported" :value="item.reported.value" />
            <span v-else class="text-xs opacity-40 italic">awaiting device</span>

            <template v-if="item.differs">
              <span class="opacity-30" aria-hidden="true">→</span>
              <TwinValue :value="item.desired!.value" />
              <span class="badge badge-warning badge-xs" title="Not yet reported by the device">
                pending
              </span>
            </template>
            <span
              v-else-if="item.desired"
              class="badge badge-ghost badge-xs"
              title="Desired value set, and the device agrees"
            >set</span>
          </span>
        </template>

        <template #card-prop="{ item }">
          <div class="font-mono text-sm font-bold text-primary truncate">{{ item.prop }}</div>
        </template>
        <template #card-value="{ item }">
          <span class="inline-flex items-center gap-1.5 min-w-0">
            <TwinValue v-if="item.reported" :value="item.reported.value" />
            <span v-else class="text-xs opacity-40 italic">awaiting device</span>
            <template v-if="item.differs">
              <span class="opacity-30" aria-hidden="true">→</span>
              <TwinValue :value="item.desired!.value" />
              <span class="badge badge-warning badge-xs">pending</span>
            </template>
          </span>
        </template>

        <!-- Rows with a desired value always show their controls. Rows without
             reveal Set on hover, so a list of pure telemetry stays quiet. Mobile
             has no hover, so the lg: prefixes keep it visible there. -->
        <template #actions="{ item }">
          <template v-if="item.desired">
            <button class="btn btn-xs btn-ghost" @click="openSet(item)">Edit</button>
            <button class="btn btn-xs btn-ghost text-error" @click="clearDesired(item)">Clear</button>
          </template>
          <button
            v-else
            class="btn btn-xs btn-ghost lg:opacity-0 lg:group-hover:opacity-100 lg:focus:opacity-100 transition-opacity"
            @click="openSet(item)"
          >Set</button>
        </template>

        <template #empty>
          <div class="flex flex-col items-center gap-2 opacity-50">
            <span class="text-4xl">🛰️</span>
            <span class="text-sm">Nothing reported yet, and no desired values set.</span>
          </div>
        </template>
      </ResponsiveList>
    </div>

    <!-- Property detail: one flow, not two panes. The desired block appears only
         when there is one; otherwise a single action to add it. -->
    <dialog class="modal" :class="{ 'modal-open': !!detailProp }">
      <div class="modal-box max-w-2xl">
        <h3 class="font-bold text-lg font-mono">{{ detailProp }}</h3>

        <template v-if="detailRow">
          <section class="mt-4">
            <div class="flex items-baseline justify-between mb-1">
              <span class="text-xs font-bold uppercase tracking-widest">Value</span>
              <span class="badge badge-ghost badge-xs">reported by the device</span>
            </div>
            <template v-if="detailRow.reported">
              <JsonViewer :data="detailRow.reported.value" />
              <p class="text-xs text-base-content/70 mt-1">
                rev #{{ detailRow.reported.revision }} · {{ formatDate(detailRow.reported.created) }}
              </p>
            </template>
            <p v-else class="text-sm text-base-content/70">
              The device has not reported this property.
            </p>
          </section>

          <section v-if="detailRow.desired" class="mt-6">
            <div class="flex items-baseline justify-between mb-1">
              <span class="text-xs font-bold uppercase tracking-widest">Desired</span>
              <div class="flex gap-1">
                <button class="btn btn-xs btn-ghost" @click="openSet(detailRow)">Edit</button>
                <button class="btn btn-xs btn-ghost text-error" @click="clearDesired(detailRow)">
                  Clear
                </button>
              </div>
            </div>
            <JsonViewer :data="detailRow.desired.value" />
            <p class="text-xs text-base-content/70 mt-1">
              rev #{{ detailRow.desired.revision }} · {{ formatDate(detailRow.desired.created) }}
            </p>
            <div v-if="detailRow.differs" class="alert alert-warning mt-2 py-2 text-xs">
              Waiting for the device to apply this.
            </div>
          </section>

          <section v-else class="mt-6">
            <button class="btn btn-sm btn-outline" @click="openSet(detailRow)">
              Set a desired value
            </button>
          </section>

          <section class="mt-6">
            <h4 class="text-xs font-bold uppercase tracking-widest mb-2">History</h4>
            <div v-if="historyLoading" class="flex justify-center p-4">
              <span class="loading loading-spinner loading-sm"></span>
            </div>
            <p v-else-if="!historyRows.length" class="text-sm text-base-content/70">
              No revisions retained.
            </p>
            <div v-else class="overflow-x-auto max-h-64">
              <table class="table table-xs table-pin-rows">
                <thead>
                  <tr><th>Rev</th><th>Value</th><th>When</th></tr>
                </thead>
                <tbody>
                  <tr v-for="rev in historyRows" :key="rev.revision">
                    <td class="font-mono">#{{ rev.revision }}</td>
                    <td class="font-mono break-all">
                      <template v-if="rev.operation === 'PUT'">
                        {{ typeof rev.value === 'object' ? JSON.stringify(rev.value) : rev.value }}
                      </template>
                      <span v-else class="badge badge-ghost badge-xs">{{ rev.operation }}</span>
                    </td>
                    <td class="whitespace-nowrap text-base-content/70">{{ formatDate(rev.created) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </template>

        <div class="modal-action">
          <button class="btn btn-ghost" @click="detailProp = null">Close</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="detailProp = null"><button>close</button></form>
    </dialog>

    <!-- Desired-value editor -->
    <dialog class="modal" :class="{ 'modal-open': editorOpen }">
      <div class="modal-box">
        <h3 class="font-bold text-lg">
          {{ editingPropLocked ? `Desired value — ${editingProp}` : 'Set desired value' }}
        </h3>

        <div v-if="!editingPropLocked" class="form-control mt-4">
          <label class="label"><span class="label-text">Property</span></label>
          <input
            v-model="editingProp"
            class="input input-bordered w-full font-mono text-sm"
            placeholder="setpoint"
          />
        </div>

        <div class="form-control mt-3">
          <div class="flex items-center justify-between mb-1">
            <label class="label p-0"><span class="label-text">Value</span></label>
            <div class="flex gap-1">
              <button
                v-for="q in quickSets"
                :key="q.label"
                type="button"
                class="btn btn-ghost btn-xs h-5 min-h-0 px-1.5 font-mono text-[10px]"
                :title="`Set to ${q.label}`"
                @click="editorValue = q.apply()"
              >{{ q.label }}</button>
            </div>
          </div>
          <textarea
            v-model="editorValue"
            class="textarea textarea-bordered w-full font-mono text-sm"
            rows="5"
            placeholder="23"
          ></textarea>
          <!-- The JSON-or-text fallback is quiet and easy to get wrong, so say
               out loud what will be written. -->
          <label class="label">
            <span class="label-text-alt text-base-content/70">
              stored as
              <span class="badge badge-ghost badge-xs mx-1">{{ parsedPreview.type }}</span>
              <span v-if="parsedPreview.note" class="opacity-70">{{ parsedPreview.note }}</span>
            </span>
          </label>
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
  </BaseCard>
</template>
