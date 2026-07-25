<!-- ui/src/components/nats/KvDashboard.vue -->
<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate, formatRelativeTime } from '@/utils/format'
import { TWIN_BUCKET, twinDrift, valueAtPath } from '@/utils/twin'
import BaseCard from '@/components/ui/BaseCard.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'

interface KvEntryWithId extends KvEntry {
  id: string
  /** Twin annotation, resolved in `allEntries`. Absent outside twin mode. */
  twin?: TwinMarker | null
}

interface TreeNode {
  name: string
  path: string
  fullKey?: string
  entry?: KvEntryWithId
  children: TreeNode[]
}

interface FlatTreeRow {
  node: TreeNode
  depth: number
  isLeaf: boolean
  expanded: boolean
  hasChildren: boolean
}

const props = withDefaults(defineProps<{
  bucket?: string
  baseKey?: string
  title?: string
  /**
   * Companion bucket holding DESIRED values for the same keys (`twin_desired`).
   * When set, this browser becomes the digital-twin view: each key gains a
   * desired counterpart, the reported side goes read-only because the edge owns
   * it, and rows are marked according to whether the assertion holds. Omit it
   * and everything below behaves exactly as the general key browser always has.
   */
  desiredBucket?: string
}>(), {
  baseKey: '',
  title: '',
})

const effectiveBucket = computed(() => props.bucket || TWIN_BUCKET)
const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket || TWIN_BUCKET, props.baseKey)

// Constructed unconditionally (composables cannot be called conditionally) but
// only ever opened when a desiredBucket was actually given.
const desiredKv = useNatsKv(props.desiredBucket || TWIN_BUCKET, props.baseKey)
const twinMode = computed(() => !!props.desiredBucket)

const toast = useToast()
const { confirm } = useConfirm()

const selectedEntry = ref<KvEntryWithId | null>(null)
const isAddingNew = ref(false)
const historyLoading = ref(false)
const historyEntries = ref<KvEntryWithId[]>([])
const expandedRevisions = ref<Set<string>>(new Set())

const searchQuery = ref('')
const debouncedSearch = ref('')
let searchTimer: number | null = null

const viewMode = ref<'flat' | 'tree'>(props.baseKey ? 'tree' : 'flat')
const expandedNodes = ref<Set<string>>(new Set())

const currentPage = ref(1)
const itemsPerPage = 20

const inputKey = ref('')
const inputValue = ref('')
const valueParseError = ref('')

/**
 * Which side the editor is pointed at. Only meaningful in twin mode; the
 * general browser stays on 'reported' forever and never renders the toggle.
 *
 * In twin mode the reported side is READ-ONLY — the edge owns it, so an edit
 * would be overwritten on the next sync. Desired is the writable half.
 */
const editorTarget = ref<'reported' | 'desired'>('reported')
const editorReadOnly = computed(() => twinMode.value && editorTarget.value === 'reported')

/** Desired entry for whatever the editor currently has open. */
const selectedDesired = computed(() =>
  selectedEntry.value ? desiredState.value.get(selectedEntry.value.key) : undefined,
)

const selectedDriftPairs = computed(() =>
  selectedEntry.value ? driftPairs(selectedEntry.value.key) : [],
)

function displayKey(fullKey: string) {
  if (!props.baseKey) return fullKey
  const prefix = `${props.baseKey}.`
  return fullKey.startsWith(prefix) ? fullKey.slice(prefix.length) : fullKey
}

/**
 * Three states, not two. 'unreported' is its own case because "the device
 * disagrees" and "the device has never said anything" want different words —
 * folding them together produced a differs marker naming a path that existed on
 * neither side.
 */
type TwinStatus = 'agrees' | 'differs' | 'unreported'

/** Desired entry + the paths on which the device disagrees, keyed by full key. */
const desiredState = computed(() => {
  const m = new Map<string, { entry: KvEntry; drift: string[]; status: TwinStatus }>()
  if (!twinMode.value) return m
  for (const [key, d] of desiredKv.entries.value) {
    const reported = entries.value.get(key)
    if (!reported) {
      m.set(key, { entry: d, drift: [], status: 'unreported' })
      continue
    }
    const drift = twinDrift(d.value, reported.value)
    m.set(key, { entry: d, drift, status: drift.length ? 'differs' : 'agrees' })
  }
  return m
})

const driftCount = computed(
  () => Array.from(desiredState.value.values()).filter(d => d.status === 'differs').length,
)

/**
 * Show the values, not the path names. "Differs on: mode" makes the reader go
 * look both values up; `"auto" → "manual"` is the same width and is the answer.
 * A top-level scalar comes back from twinDrift with an empty path, so it renders
 * as a bare pair with nothing to label.
 */
function driftPairs(key: string): { path: string; reported: any; desired: any }[] {
  const d = desiredState.value.get(key)
  const reported = entries.value.get(key)
  if (!d || !reported) return []
  return d.drift.map(path => ({
    path,
    reported: valueAtPath(reported.value, path),
    desired: valueAtPath(d.entry.value, path),
  }))
}

interface TwinMarker {
  agrees: boolean
  /** The desired value itself, when it is a primitive short enough to sit in the row. */
  inline: string | null
  /** Fallback wording when the desired value is an object and cannot sit inline. */
  badge: string
  title: string
}

/**
 * The row annotation, resolved once per key rather than branched in the
 * template. Rows without a desired value get null and render nothing at all —
 * the quiet case stays quiet.
 */
function twinMarker(key: string): TwinMarker | null {
  const d = desiredState.value.get(key)
  if (!d) return null

  if (d.status === 'agrees') {
    return { agrees: true, inline: null, badge: '', title: 'Desired value set, and the device agrees' }
  }

  const inline = isObject(d.entry.value) ? null : previewValue(d.entry.value)

  if (d.status === 'unreported') {
    return {
      agrees: false,
      inline,
      badge: 'desired set',
      title: 'Desired value set. The device has not reported this property.',
    }
  }

  return {
    agrees: false,
    inline,
    badge: 'differs',
    title: driftPairs(key)
      .map(p => `${p.path ? p.path + ': ' : ''}${previewValue(p.reported)} → ${previewValue(p.desired)}`)
      .join('\n'),
  }
}

const allEntries = computed<KvEntryWithId[]>(() => {
  const out = Array.from(entries.value.values()).map(entry => ({ ...entry, id: entry.key }))

  // A desired value set for a property the device has never reported still has
  // to appear, or it silently vanishes the moment you save it. Such rows carry
  // `value: undefined`, which `isReported()` below distinguishes from a value
  // that genuinely IS undefined-ish (null renders as null).
  if (twinMode.value) {
    for (const [key, d] of desiredKv.entries.value) {
      if (entries.value.has(key)) continue
      out.push({ key, value: undefined, revision: 0, created: d.created, operation: 'PUT', id: key })
    }
  }

  out.sort((a, b) => a.key.localeCompare(b.key))
  // Resolved here so both the table and the tree read one field instead of each
  // re-deriving the same three-way branch inline.
  return twinMode.value ? out.map(e => ({ ...e, twin: twinMarker(e.key) })) : out
})

function isReported(key: string): boolean {
  return entries.value.has(key)
}

const filteredEntries = computed(() => {
  const q = debouncedSearch.value
  if (!q) return allEntries.value
  return allEntries.value.filter(item => {
    const keyMatch = displayKey(item.key).toLowerCase().includes(q)
    const valMatch = JSON.stringify(item.value).toLowerCase().includes(q)
    return keyMatch || valMatch
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredEntries.value.length / itemsPerPage)))
const paginatedEntries = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return filteredEntries.value.slice(start, start + itemsPerPage)
})

const treeRoots = computed<TreeNode[]>(() => {
  const root: TreeNode = { name: '', path: '', children: [] }
  for (const entry of filteredEntries.value) {
    const display = displayKey(entry.key)
    const segments = display.split('.')
    let cur = root
    let path = ''
    for (let i = 0; i < segments.length; i++) {
      const seg = segments[i]
      path = path ? `${path}.${seg}` : seg
      let child = cur.children.find(c => c.name === seg)
      if (!child) {
        child = { name: seg, path, children: [] }
        cur.children.push(child)
      }
      if (i === segments.length - 1) {
        child.fullKey = entry.key
        child.entry = entry
      }
      cur = child
    }
  }
  return root.children
})

const flatTree = computed<FlatTreeRow[]>(() => {
  const out: FlatTreeRow[] = []
  const autoExpand = !!debouncedSearch.value
  function walk(nodes: TreeNode[], depth: number) {
    for (const node of nodes) {
      const hasChildren = node.children.length > 0
      const isLeaf = !hasChildren
      const expanded = isLeaf || autoExpand || expandedNodes.value.has(node.path)
      out.push({ node, depth, isLeaf, expanded, hasChildren })
      if (hasChildren && expanded) walk(node.children, depth + 1)
    }
  }
  walk(treeRoots.value, 0)
  return out
})

watch(searchQuery, (val) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    debouncedSearch.value = val.toLowerCase().trim()
    currentPage.value = 1
  }, 200)
})

watch(viewMode, () => {
  currentPage.value = 1
})

onMounted(() => {
  init()
  if (twinMode.value) desiredKv.init()
})

function toggleNode(path: string) {
  if (expandedNodes.value.has(path)) expandedNodes.value.delete(path)
  else expandedNodes.value.add(path)
}

function expandAll() {
  const all = new Set<string>()
  function collect(nodes: TreeNode[]) {
    for (const n of nodes) {
      if (n.children.length) {
        all.add(n.path)
        collect(n.children)
      }
    }
  }
  collect(treeRoots.value)
  expandedNodes.value = all
}

function collapseAll() {
  expandedNodes.value = new Set()
}

function valueToEditable(val: any): string {
  if (typeof val === 'string') return val
  return JSON.stringify(val, null, 2)
}

function parseInputValue(): { ok: true; value: any } | { ok: false; error: string } {
  const raw = inputValue.value
  if (raw === '') return { ok: true, value: '' }
  try {
    return { ok: true, value: JSON.parse(raw) }
  } catch {
    return { ok: true, value: raw }
  }
}

watch(inputValue, () => {
  valueParseError.value = ''
})

async function handleSave() {
  if (!inputKey.value.trim()) {
    toast.error('Property name required')
    return
  }
  if (editorReadOnly.value) return // guarded in the template too; belt and braces
  const parsed = parseInputValue()
  if (!parsed.ok) {
    valueParseError.value = parsed.error
    return
  }
  if (editorTarget.value === 'desired') {
    try {
      await desiredKv.put(inputKey.value, parsed.value)
      toast.success('Desired value saved')
      isAddingNew.value = false
    } catch (e: any) {
      toast.error(e?.message || 'Save failed')
    }
    return
  }
  try {
    await put(inputKey.value, parsed.value)
    toast.success('Property saved')
    if (isAddingNew.value) {
      isAddingNew.value = false
      const fullKey = props.baseKey ? `${props.baseKey}.${inputKey.value}` : inputKey.value
      const entry = entries.value.get(fullKey)
      if (entry) openEdit({ ...entry, id: entry.key })
    } else if (selectedEntry.value) {
      loadHistory(selectedEntry.value.key)
    }
  } catch (e: any) {
    toast.error(e?.message || 'Save failed')
  }
}

function openEdit(entry: KvEntryWithId) {
  isAddingNew.value = false
  selectedEntry.value = entry
  inputKey.value = displayKey(entry.key)
  // A desired-only row has no reported value to show, so open straight on the
  // side that actually has one.
  editorTarget.value = twinMode.value && !isReported(entry.key) ? 'desired' : 'reported'
  loadEditorValue()
  valueParseError.value = ''
  loadHistory(entry.key)
}

/** Pull the editor's text from whichever side it is pointed at. */
function loadEditorValue() {
  if (editorTarget.value === 'desired') {
    const d = selectedDesired.value
    inputValue.value = d ? valueToEditable(d.entry.value) : ''
    return
  }
  const v = selectedEntry.value?.value
  inputValue.value = v === undefined ? '' : valueToEditable(v)
}

function switchEditorTarget(target: 'reported' | 'desired') {
  if (editorTarget.value === target) return
  editorTarget.value = target
  valueParseError.value = ''
  loadEditorValue()
}

/** Seed a new desired value from what the device reports — usually a small edit. */
function seedDesiredFromReported() {
  editorTarget.value = 'desired'
  const v = selectedEntry.value?.value
  inputValue.value = v === undefined ? '' : valueToEditable(v)
}

function openAdd() {
  isAddingNew.value = true
  selectedEntry.value = null
  inputKey.value = ''
  inputValue.value = ''
  // Adding in twin mode means asserting a desired value; the device is the only
  // thing that creates reported keys.
  editorTarget.value = twinMode.value ? 'desired' : 'reported'
  valueParseError.value = ''
  historyEntries.value = []
}

function closeEditor() {
  selectedEntry.value = null
  isAddingNew.value = false
}

async function handleDelete() {
  if (!selectedEntry.value) return
  const key = selectedEntry.value.key

  if (editorTarget.value === 'desired') {
    const confirmed = await confirm({
      title: 'Clear desired value?',
      message: `Remove the desired value for "${displayKey(key)}".`,
      details: 'The device keeps its last received value until something sets a new one.',
      confirmText: 'Clear',
      variant: 'warning',
    })
    if (!confirmed) return
    try {
      await desiredKv.del(key)
      if (!isReported(key)) closeEditor() // the row existed only for this value
      else switchEditorTarget('reported')
    } catch (e: any) {
      toast.error(e?.message || 'Failed to clear desired value')
    }
    return
  }

  const confirmed = await confirm({
    title: 'Delete Property',
    message: `Are you sure you want to delete "${displayKey(key)}"?`,
    details: 'This will publish a delete marker to the KV bucket. Prior revisions remain in the history.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await del(key)
    closeEditor()
  } catch (e: any) {
    toast.error(e?.message || 'Failed to delete property')
  }
}

async function loadHistory(key: string) {
  historyLoading.value = true
  expandedRevisions.value = new Set()
  try {
    const raw = await getHistory(key)
    historyEntries.value = raw.map(h => ({ ...h, id: `${h.key}-${h.revision}` }))
  } finally {
    historyLoading.value = false
  }
}

function toggleRevision(id: string) {
  if (expandedRevisions.value.has(id)) expandedRevisions.value.delete(id)
  else expandedRevisions.value.add(id)
}

function copyRevisionToEditor(rev: KvEntryWithId) {
  inputValue.value = valueToEditable(rev.value)
  toast.info(`Loaded rev #${rev.revision} into editor (not saved)`)
}

async function restoreRevision(rev: KvEntryWithId) {
  if (rev.operation === 'DEL' || rev.operation === 'PURGE') {
    toast.error('Cannot restore a deletion event')
    return
  }
  const confirmed = await confirm({
    title: 'Restore Revision',
    message: `Restore "${displayKey(rev.key)}" to revision #${rev.revision}?`,
    details: 'This creates a new revision with the value from this older revision. Current value remains in history.',
    confirmText: 'Restore',
  })
  if (!confirmed) return
  try {
    await put(displayKey(rev.key), rev.value)
    toast.success(`Restored revision #${rev.revision}`)
    inputValue.value = valueToEditable(rev.value)
    await loadHistory(rev.key)
  } catch (e: any) {
    toast.error(e?.message || 'Restore failed')
  }
}

function quickSetValue(literal: string) {
  inputValue.value = literal
}

function quickSetNow() {
  inputValue.value = JSON.stringify(new Date().toISOString())
}

function isObject(val: any) {
  return typeof val === 'object' && val !== null
}

function previewValue(val: any): string {
  if (typeof val === 'string') return val
  if (val === null) return 'null'
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  if (typeof val === 'number') return String(val)
  return JSON.stringify(val)
}
</script>

<template>
  <BaseCard :title="title || (baseKey ? 'Live State' : 'Key Browser')" :no-padding="true" class="w-full overflow-hidden">
    <!-- Show the actual bucket/key prefix: the convention should be readable
         off the screen rather than reverse-engineered from a card title. -->
    <template v-if="baseKey" #header>
      <div class="flex items-center gap-3">
        <code class="font-mono text-xs text-base-content/70">{{ effectiveBucket }} / {{ baseKey }}.&gt;</code>
        <span v-if="driftCount" class="badge badge-warning badge-sm">{{ driftCount }} differs</span>
      </div>
    </template>

    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Bucket Not Initialized -->
    <div v-else-if="!exists && !error" class="text-center py-12 px-4">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">Bucket Not Initialized</h3>
      <p class="text-sm text-base-content/70 mt-2 mb-4">
        The <code class="font-mono text-xs">{{ effectiveBucket }}</code> bucket does not exist yet.
      </p>
      <button @click="createBucket()" class="btn btn-primary btn-sm">Initialize Bucket</button>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="text-center py-12 px-4">
      <div class="text-5xl mb-3">⚠️</div>
      <h3 class="font-bold text-lg">Failed to load bucket</h3>
      <p class="text-sm text-base-content/70 mt-2 mb-4">{{ error }}</p>
      <button @click="init()" class="btn btn-sm">Retry</button>
    </div>

    <!-- Main Layout -->
    <div v-else class="kv-layout">

      <!-- LEFT: Property List -->
      <div class="list-pane">
        <div class="pane-header gap-2">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Filter properties..."
            class="input input-xs input-bordered flex-1 min-w-0 font-sans"
          />
          <div class="join shrink-0">
            <button
              class="join-item btn btn-xs"
              :class="{ 'btn-primary': viewMode === 'flat' }"
              @click="viewMode = 'flat'"
              title="Flat list"
            >Flat</button>
            <button
              class="join-item btn btn-xs"
              :class="{ 'btn-primary': viewMode === 'tree' }"
              @click="viewMode = 'tree'"
              title="Tree view"
            >Tree</button>
          </div>
          <button @click="openAdd" class="btn btn-xs btn-primary shrink-0">+ Add</button>
        </div>

        <!-- Tree controls -->
        <div v-if="viewMode === 'tree'" class="px-3 py-1.5 border-b border-base-300 flex items-center gap-2 bg-base-200/20 text-[10px]">
          <button class="btn btn-ghost btn-xs h-6 min-h-0" @click="expandAll">Expand all</button>
          <button class="btn btn-ghost btn-xs h-6 min-h-0" @click="collapseAll">Collapse all</button>
          <span class="opacity-40 ml-auto">{{ filteredEntries.length }} keys</span>
        </div>

        <!-- FLAT VIEW -->
        <div v-if="viewMode === 'flat'" class="flex-1 overflow-y-auto">
          <table class="table table-sm w-full border-separate border-spacing-0">
            <thead class="sticky top-0 bg-base-100 z-10 shadow-sm">
              <tr>
                <th class="bg-base-100 py-3">Property</th>
                <th class="bg-base-100 py-3">Value</th>
                <th class="bg-base-100 py-3 text-right">Rev</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in paginatedEntries"
                :key="item.key"
                @click="openEdit(item)"
                class="hover:bg-primary/5 cursor-pointer transition-colors group"
                :class="{ 'bg-primary/10': selectedEntry?.key === item.key }"
              >
                <td class="py-3 align-top">
                  <span class="font-mono font-semibold text-primary group-hover:underline break-all">{{ displayKey(item.key) }}</span>
                </td>
                <td class="py-3 align-top">
                  <div class="flex items-center gap-2 min-w-0">
                    <span v-if="!isReported(item.key)" class="text-[10px] italic opacity-40">awaiting device</span>
                    <span v-else-if="typeof item.value === 'boolean'" class="badge badge-sm" :class="item.value ? 'badge-success' : 'badge-ghost'">
                      {{ item.value ? 'TRUE' : 'FALSE' }}
                    </span>
                    <span v-else-if="item.value === null" class="badge badge-sm badge-ghost">null</span>
                    <span v-else-if="typeof item.value === 'number'" class="text-xs font-mono font-medium">{{ item.value }}</span>
                    <span v-else-if="isObject(item.value)" class="text-[10px] font-mono opacity-70 truncate block max-w-xs">{{ JSON.stringify(item.value) }}</span>
                    <span v-else class="text-xs font-medium truncate block max-w-xs">{{ item.value }}</span>

                    <!-- Twin marker: only rows that carry an assertion show one,
                         and it shows the desired VALUE rather than a word about
                         it wherever the value is a primitive. -->
                    <template v-if="item.twin">
                      <span v-if="item.twin.agrees" class="text-success shrink-0 leading-none" :title="item.twin.title">✓</span>
                      <template v-else>
                        <span class="opacity-30 text-xs shrink-0">→</span>
                        <span
                          v-if="item.twin.inline !== null"
                          class="font-mono text-xs font-semibold text-warning shrink-0 truncate max-w-[12rem]"
                          :title="item.twin.title"
                        >{{ item.twin.inline }}</span>
                        <span v-else class="badge badge-warning badge-xs shrink-0" :title="item.twin.title">{{ item.twin.badge }}</span>
                      </template>
                    </template>
                  </div>
                </td>
                <td class="py-3 text-right align-top font-mono text-[10px] opacity-40">
                  {{ item.revision }}
                </td>
              </tr>
              <tr v-if="paginatedEntries.length === 0">
                <td colspan="3" class="py-20 text-center opacity-30 italic">
                  {{ debouncedSearch ? 'No properties match your search' : 'No properties yet' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- TREE VIEW -->
        <div v-else class="flex-1 overflow-y-auto">
          <ul class="py-1">
            <li
              v-for="row in flatTree"
              :key="row.node.path"
              class="flex items-center gap-3 pr-3 py-2.5 cursor-pointer hover:bg-primary/5 group transition-colors"
              :class="{ 'bg-primary/10': row.isLeaf && selectedEntry?.key === row.node.fullKey }"
              :style="{ paddingLeft: `${0.75 + row.depth * 1.5}rem` }"
              @click="row.isLeaf ? row.node.entry && openEdit(row.node.entry) : toggleNode(row.node.path)"
            >
              <span class="w-4 shrink-0 text-center text-xs opacity-50">
                <span v-if="row.hasChildren">{{ row.expanded ? '▾' : '▸' }}</span>
                <span v-else>·</span>
              </span>
              <span
                class="font-mono text-sm shrink-0 break-all"
                :class="row.isLeaf ? 'font-semibold text-primary group-hover:underline' : 'font-bold opacity-70'"
              >{{ row.node.name }}</span>
              <template v-if="row.isLeaf && row.node.entry">
                <span class="opacity-30 text-xs shrink-0">=</span>
                <!-- Desired-only rows have no reported value; without this they
                     render as a blank cell after the `=`. -->
                <span v-if="!isReported(row.node.entry.key)" class="text-[10px] italic opacity-40 shrink-0">awaiting device</span>
                <span v-else-if="typeof row.node.entry.value === 'boolean'" class="badge badge-sm shrink-0" :class="row.node.entry.value ? 'badge-success' : 'badge-ghost'">
                  {{ row.node.entry.value ? 'TRUE' : 'FALSE' }}
                </span>
                <span v-else-if="row.node.entry.value === null" class="badge badge-sm badge-ghost shrink-0">null</span>
                <span v-else class="text-xs opacity-70 truncate min-w-0 flex-1 font-mono">{{ previewValue(row.node.entry.value) }}</span>
                <template v-if="row.node.entry.twin">
                  <span v-if="row.node.entry.twin.agrees" class="text-success shrink-0 leading-none" :title="row.node.entry.twin.title">✓</span>
                  <template v-else>
                    <span class="opacity-30 text-xs shrink-0">→</span>
                    <span
                      v-if="row.node.entry.twin.inline !== null"
                      class="font-mono text-xs font-semibold text-warning shrink-0 truncate max-w-[12rem]"
                      :title="row.node.entry.twin.title"
                    >{{ row.node.entry.twin.inline }}</span>
                    <span v-else class="badge badge-warning badge-xs shrink-0" :title="row.node.entry.twin.title">{{ row.node.entry.twin.badge }}</span>
                  </template>
                </template>
                <span class="text-[10px] opacity-30 font-mono shrink-0 ml-auto">r{{ row.node.entry.revision }}</span>
              </template>
              <span v-else class="text-[10px] opacity-40 ml-auto shrink-0 font-mono">{{ row.node.children.length }}</span>
            </li>
            <li v-if="flatTree.length === 0" class="py-20 text-center opacity-30 italic text-sm">
              {{ debouncedSearch ? 'No properties match your search' : 'No properties yet' }}
            </li>
          </ul>
        </div>

        <!-- Pagination Footer (flat only) -->
        <div v-if="viewMode === 'flat'" class="p-3 border-t border-base-300 flex justify-between items-center bg-base-200/10">
          <span class="text-[10px] opacity-50">
            {{ filteredEntries.length }} keys • Page {{ currentPage }} of {{ totalPages }}
          </span>
          <div class="join">
            <button class="join-item btn btn-xs" :disabled="currentPage === 1" @click="currentPage--">«</button>
            <button class="join-item btn btn-xs" :disabled="currentPage >= totalPages" @click="currentPage++">»</button>
          </div>
        </div>
      </div>

      <!-- RIGHT: Editor Pane -->
      <div
        class="editor-pane"
        :class="{ 'mobile-active': selectedEntry || isAddingNew }"
      >
        <div class="mobile-backdrop lg:hidden" @click="closeEditor"></div>

        <div class="editor-content-wrapper">
          <div class="pane-header bg-base-200/50">
            <h3 class="font-bold uppercase text-[10px] tracking-widest opacity-70">
              {{ isAddingNew ? 'New Property' : (selectedEntry ? 'Property Details' : 'Editor') }}
            </h3>
            <button v-if="selectedEntry || isAddingNew" @click="closeEditor" class="btn btn-xs btn-ghost btn-circle" aria-label="Close">✕</button>
          </div>

          <div class="flex-1 overflow-y-auto p-5">
            <div v-if="selectedEntry || isAddingNew" class="space-y-6">
              <!-- Twin mode: which side of the pair the editor is pointed at.
                   Reported is read-only because the edge owns it — an edit here
                   would be overwritten on the next sync. -->
              <div v-if="twinMode && !isAddingNew" class="join w-full">
                <button
                  class="join-item btn btn-xs flex-1"
                  :class="{ 'btn-primary': editorTarget === 'reported' }"
                  @click="switchEditorTarget('reported')"
                >Reported</button>
                <button
                  class="join-item btn btn-xs flex-1"
                  :class="{ 'btn-primary': editorTarget === 'desired' }"
                  @click="switchEditorTarget('desired')"
                >
                  Desired
                  <span v-if="selectedDesired?.status === 'differs'" class="badge badge-warning badge-xs ml-1">!</span>
                </button>
              </div>

              <!-- The difference itself, not a sentence about it. Naming the
                   paths ("Differs on: mode") sends the reader off to look both
                   values up; showing `"auto" → "manual"` is the same width and
                   is the answer. Deliberately never "pending": nothing in this
                   platform applies desired values to devices, so state the
                   difference and predict nothing. -->
              <div
                v-if="twinMode && editorTarget === 'desired' && selectedDesired && selectedDesired.status !== 'agrees'"
                class="rounded-lg border border-warning/40 bg-warning/5 px-3 py-2.5"
              >
                <p v-if="selectedDesired.status === 'unreported'" class="text-[11px] opacity-70">
                  The device has not reported this property.
                </p>
                <div v-else class="grid grid-cols-[auto_1fr_auto_1fr] gap-x-2 gap-y-1 items-baseline">
                  <span></span>
                  <span class="text-[9px] uppercase tracking-widest opacity-50">Reported</span>
                  <span></span>
                  <span class="text-[9px] uppercase tracking-widest opacity-50">Desired</span>
                  <template v-for="p in selectedDriftPairs" :key="p.path">
                    <span class="font-mono text-[11px] opacity-50 break-all">{{ p.path }}</span>
                    <span class="font-mono text-[11px] opacity-80 break-all">{{ previewValue(p.reported) }}</span>
                    <span class="opacity-30 text-[11px]">→</span>
                    <span class="font-mono text-[11px] font-semibold text-warning break-all">{{ previewValue(p.desired) }}</span>
                  </template>
                </div>
              </div>

              <div class="space-y-4">
                <div class="form-control">
                  <label class="label p-0 mb-1"><span class="label-text text-[10px] font-bold opacity-50 uppercase">Key</span></label>
                  <div v-if="baseKey && isAddingNew" class="flex items-stretch">
                    <span class="px-2 flex items-center text-xs font-mono opacity-50 bg-base-200 border border-r-0 border-base-300 rounded-l">{{ baseKey }}.</span>
                    <input v-model="inputKey" type="text" class="input input-sm input-bordered font-mono flex-1 rounded-l-none" placeholder="property.name" />
                  </div>
                  <input v-else v-model="inputKey" type="text" class="input input-sm input-bordered font-mono" :disabled="!isAddingNew" />
                </div>

                <div class="form-control">
                  <div class="flex items-center justify-between mb-1">
                    <label class="label p-0"><span class="label-text text-[10px] font-bold opacity-50 uppercase">
                      {{ twinMode ? (editorTarget === 'desired' ? 'Desired value (JSON)' : 'Reported value (JSON)') : 'Value (JSON)' }}
                    </span></label>
                    <div v-if="!editorReadOnly" class="flex gap-1">
                      <button type="button" @click="quickSetValue('true')" class="btn btn-ghost btn-xs h-5 min-h-0 px-1.5 font-mono text-[10px]" title="Set to true">true</button>
                      <button type="button" @click="quickSetValue('false')" class="btn btn-ghost btn-xs h-5 min-h-0 px-1.5 font-mono text-[10px]" title="Set to false">false</button>
                      <button type="button" @click="quickSetValue('null')" class="btn btn-ghost btn-xs h-5 min-h-0 px-1.5 font-mono text-[10px]" title="Set to null">null</button>
                      <button type="button" @click="quickSetNow" class="btn btn-ghost btn-xs h-5 min-h-0 px-1.5 font-mono text-[10px]" title="Set to current time (ISO)">now()</button>
                    </div>
                  </div>
                  <textarea
                    v-model="inputValue"
                    :readonly="editorReadOnly"
                    class="textarea textarea-bordered font-mono text-xs h-40 leading-snug"
                    :class="{ 'opacity-70 cursor-default': editorReadOnly }"
                    placeholder='Examples: 42  ·  "hello"  ·  true  ·  {"foo":"bar"}'
                    spellcheck="false"
                  ></textarea>
                  <p v-if="editorReadOnly" class="text-[10px] opacity-50 mt-1">
                    Reported by the device — read-only. The edge overwrites this on every sync.
                  </p>
                  <p v-else class="text-[10px] opacity-50 mt-1">
                    Parsed as JSON when valid (numbers, booleans, null, objects, arrays). Otherwise stored as a string.
                  </p>
                  <p v-if="valueParseError" class="text-[10px] text-error mt-1">{{ valueParseError }}</p>
                </div>

                <!-- Reported side in twin mode has no writes; offer the one
                     action that does apply, pre-filled from what it reports. -->
                <div v-if="editorReadOnly" class="pt-2">
                  <button
                    v-if="!selectedDesired"
                    @click="seedDesiredFromReported"
                    class="btn btn-sm btn-outline w-full"
                  >Set a desired value</button>
                  <button v-else @click="switchEditorTarget('desired')" class="btn btn-sm btn-outline w-full">
                    Edit desired value
                  </button>
                </div>

                <div v-else class="flex gap-2 pt-2">
                  <button
                    v-if="!isAddingNew && (editorTarget === 'reported' || selectedDesired)"
                    @click="handleDelete"
                    class="btn btn-sm btn-outline flex-1"
                    :class="editorTarget === 'desired' ? 'btn-warning' : 'btn-error'"
                  >{{ editorTarget === 'desired' ? 'Clear' : 'Delete' }}</button>
                  <button @click="handleSave" class="btn btn-sm btn-primary flex-1">Save</button>
                </div>
              </div>

              <!-- History -->
              <div v-if="!isAddingNew" class="pt-4 border-t border-base-300">
                <div class="flex justify-between items-center mb-4">
                  <span class="text-[10px] font-bold uppercase opacity-50">Revision History</span>
                  <span v-if="historyEntries.length > 0" class="text-[10px] opacity-40">{{ historyEntries.length }} revisions</span>
                </div>

                <div v-if="historyLoading" class="flex justify-center"><span class="loading loading-dots loading-xs"></span></div>
                <div v-else-if="historyEntries.length === 0" class="text-center text-[10px] opacity-30 italic py-4">No history</div>
                <div v-else class="space-y-2">
                  <div
                    v-for="rev in historyEntries"
                    :key="rev.id"
                    class="relative pl-4 border-l border-base-300 group"
                  >
                    <div class="absolute -left-[4.5px] top-2 w-2 h-2 rounded-full bg-base-300 group-hover:bg-primary transition-colors"></div>

                    <!-- Row header: clickable to expand -->
                    <div
                      class="flex items-center gap-2 cursor-pointer rounded hover:bg-base-200/50 px-1 py-1 -mx-1"
                      @click="rev.operation !== 'DEL' && rev.operation !== 'PURGE' && toggleRevision(rev.id)"
                    >
                      <span class="text-[10px] opacity-50 w-3 shrink-0">
                        <template v-if="rev.operation !== 'DEL' && rev.operation !== 'PURGE'">{{ expandedRevisions.has(rev.id) ? '▾' : '▸' }}</template>
                      </span>
                      <span class="text-[10px] font-bold opacity-70 shrink-0">REV #{{ rev.revision }}</span>
                      <span v-if="rev.revision === selectedEntry?.revision" class="badge badge-xs badge-primary badge-outline shrink-0">current</span>
                      <span v-if="rev.operation === 'DEL' || rev.operation === 'PURGE'" class="badge badge-xs badge-ghost shrink-0">{{ rev.operation }}</span>
                      <span v-else class="text-[10px] font-mono opacity-60 truncate min-w-0 flex-1">{{ previewValue(rev.value) }}</span>
                      <span class="text-[10px] font-mono opacity-50 shrink-0" :title="formatDate(rev.created)">{{ formatRelativeTime(rev.created) }}</span>
                    </div>

                    <!-- Expanded full value -->
                    <div
                      v-if="expandedRevisions.has(rev.id) && rev.operation !== 'DEL' && rev.operation !== 'PURGE'"
                      class="mt-2 ml-1 mr-1 rounded border border-base-300 bg-base-200/40 overflow-hidden"
                    >
                      <div class="max-h-64 overflow-auto p-2">
                        <JsonViewer :data="rev.value" />
                      </div>
                      <div class="flex justify-end gap-2 px-2 py-1.5 border-t border-base-300 bg-base-200/60">
                        <button
                          @click.stop="copyRevisionToEditor(rev)"
                          class="btn btn-xs btn-ghost h-6 min-h-0"
                          title="Load this value into the editor without saving"
                        >Load into editor</button>
                        <button
                          v-if="rev.revision !== selectedEntry?.revision"
                          @click.stop="restoreRevision(rev)"
                          class="btn btn-xs btn-primary h-6 min-h-0"
                          title="Save this value as a new revision"
                        >Restore</button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Desktop Empty State -->
            <div v-else class="hidden lg:flex h-full flex-col items-center justify-center text-center opacity-30 py-20">
              <span class="text-4xl mb-2">👈</span>
              <p class="text-xs font-bold uppercase tracking-widest">Select a property<br>to view or edit</p>
            </div>
          </div>
        </div>
      </div>

    </div>
  </BaseCard>
</template>

<style scoped>
.kv-layout {
  display: flex;
  flex-direction: column;
  position: relative;
  min-height: 480px;
  height: 100%;
}

.pane-header {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid oklch(var(--b3));
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: oklch(var(--b2) / 0.2);
  min-height: 48px;
  flex-shrink: 0;
}

/* --- DESKTOP --- */
@media (min-width: 1024px) {
  .kv-layout {
    flex-direction: row;
  }
  .list-pane {
    flex: 1 1 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    border-right: 1px solid oklch(var(--b3));
  }
  .editor-pane {
    flex: 0 0 420px;
    width: 420px;
    display: flex;
    flex-direction: column;
    background: oklch(var(--b2) / 0.2);
  }
  .editor-content-wrapper {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
  }
  .mobile-backdrop { display: none; }
}

/* --- MOBILE --- */
@media (max-width: 1023px) {
  .list-pane {
    width: 100%;
    display: flex;
    flex-direction: column;
  }
  .editor-pane {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: none;
  }
  .editor-pane.mobile-active {
    display: flex;
  }
  .mobile-backdrop {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(4px);
  }
  .editor-content-wrapper {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 90%;
    max-width: 440px;
    background: oklch(var(--b1));
    display: flex;
    flex-direction: column;
    box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
    animation: slide-in 0.25s ease-out;
  }
}

@keyframes slide-in {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.overflow-y-auto::-webkit-scrollbar {
  width: 4px;
}
.overflow-y-auto::-webkit-scrollbar-thumb {
  background: oklch(var(--bc) / 0.1);
  border-radius: 10px;
}
</style>
