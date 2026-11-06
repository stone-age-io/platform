<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import JsonSchemaForm from './JsonSchemaForm.vue'
import JsonViewer from './JsonViewer.vue'
import { useToast } from '@/composables/useToast'
import { describedKeys, undescribedEntries, mergeRows } from '@/utils/metadataDoc'

// Edits (or, with `disabled`, displays) a free-form `metadata` object with a
// Form / JSON toggle. As in MetadataSchemaCard, the form is a convenience over
// the document, the JSON view is the escape hatch, and anything the form cannot
// represent says so instead of dropping it.
//
// With a schema (a type's `metadata_schema`), JsonSchemaForm renders the keys it
// describes and key/value rows hold everything else. The schema is a hint, not a
// contract (utils/metadataDoc.ts). Without one, rows only.
//
// Nesting is per key: an object or array row gets a JSON textarea. No tree
// editor; the JSON tab already handles nesting.
//
// The model is an object, not a JSON string, so invalid JSON is caught before Save.

interface Props {
  modelValue: Record<string, any> | null
  /** JSON Schema describing the expected fields. Omit for free-form key/value. */
  schema?: any
  /** Read-only: renders the viewer used by the detail pages. */
  disabled?: boolean
  /** Show the field count in the toolbar. MetadataCard turns it off (its header has one). */
  showCount?: boolean
}

const props = withDefaults(defineProps<Props>(), { schema: null, disabled: false, showCount: true })
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any> | null): void
}>()

const toast = useToast()

type RowType = 'text' | 'number' | 'boolean' | 'date' | 'json'

interface Row {
  key: string
  type: RowType
  value: any
  /** Textarea buffer for `json` rows, so an in-progress edit isn't reparsed. */
  text?: string
  /** Parse error for `json` rows. Blocks commit() while set. */
  error?: string
}

const activeTab = ref<'form' | 'json'>('form')
const jsonText = ref('')
const jsonError = ref('')
const rows = ref<Row[]>([])

let suppressNextWatch = false

const doc = computed<Record<string, any>>(() => props.modelValue ?? {})
const isEmpty = computed(() => Object.keys(doc.value).length === 0)

// A property-less schema counts as absent, or the user could not add a field.
const hasSchema = computed(() => {
  const s = props.schema
  return !!(s && s.type === 'object' && s.properties && Object.keys(s.properties).length > 0)
})

// The keys the schema form owns; the rows own everything else.
const described = computed(() => describedKeys(props.schema))

function isContainer(v: any): boolean {
  return typeof v === 'object' && v !== null
}

function inferType(v: any): RowType {
  if (isContainer(v)) return 'json'
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'number') return 'number'
  // Only an ISO calendar date round-trips through <input type="date"> unchanged.
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return 'date'
  return 'text'
}

function rowsFromDoc(d: Record<string, any>): Row[] {
  return undescribedEntries(d, described.value).map(([key, value]) => {
    const type = inferType(value)
    return {
      key,
      type,
      value: type === 'json' ? value : (value ?? ''),
      text: type === 'json' ? JSON.stringify(value, null, 2) : undefined,
      error: '',
    }
  })
}

// The schema is watched too: the Thing form swaps it when the type changes, and
// the rows must give up or take over keys.
watch(
  [() => props.modelValue, () => props.schema],
  () => {
    if (suppressNextWatch) { suppressNextWatch = false; return }
    rows.value = rowsFromDoc(doc.value)
    if (activeTab.value !== 'json') refreshJsonFromDoc()
  },
  { immediate: true, deep: true },
)

function refreshJsonFromDoc() {
  jsonText.value = isEmpty.value ? '' : JSON.stringify(doc.value, null, 2)
}

// Rows to document, keeping the schema-described keys. Merge rules: mergeRows.
function emitFromRows() {
  suppressNextWatch = true
  emit('update:modelValue', mergeRows(doc.value, described.value, rows.value))
}

// A row named like a schema field collides with it too, and the row wins.
const duplicateKeys = computed(() => {
  const seen = new Set<string>(described.value)
  const dupes = new Set<string>()
  for (const r of rows.value) {
    const k = r.key.trim()
    if (!k) continue
    if (seen.has(k)) dupes.add(k)
    seen.add(k)
  }
  return dupes
})

const rowErrors = computed(() => rows.value.filter(r => r.error).length)

function addRow() {
  rows.value.push({ key: '', type: 'text', value: '', error: '' })
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
  emitFromRows()
}

function setRowKey(i: number, key: string) {
  rows.value[i].key = key
  emitFromRows()
}

// An emptied number input reports NaN, which would serialise to null; store ''.
function numberOrBlank(n: number): number | string {
  return Number.isNaN(n) ? '' : n
}

function setRowValue(i: number, value: any) {
  rows.value[i].value = value
  emitFromRows()
}

// Emits only when the text parses, so the last good value stays in the model;
// commit() refuses while any row is in error.
function setRowJson(i: number, raw: string) {
  const r = rows.value[i]
  r.text = raw
  if (!raw.trim()) {
    r.value = {}
    r.error = ''
    emitFromRows()
    return
  }
  try {
    const parsed = JSON.parse(raw)
    if (!isContainer(parsed)) {
      r.error = 'Expected an object or array — use another field type for a single value.'
      return
    }
    r.value = parsed
    r.error = ''
    emitFromRows()
  } catch (err: any) {
    r.error = err.message
  }
}

// Re-casts the current value, so text to number on "42" keeps 42.
function setRowType(i: number, type: RowType) {
  const r = rows.value[i]
  const old = r.value
  const wasContainer = isContainer(old)
  r.type = type
  r.error = ''

  if (type === 'json') {
    r.value = wasContainer ? old : {}
    r.text = JSON.stringify(r.value, null, 2)
    emitFromRows()
    return
  }

  r.text = undefined
  // Leaving `json`: an object has no scalar cast, so start empty.
  if (wasContainer) {
    r.value = type === 'boolean' ? false : ''
  } else if (type === 'number') {
    const n = typeof old === 'number' ? old : parseFloat(String(old))
    r.value = Number.isNaN(n) ? '' : n
  } else if (type === 'boolean') {
    r.value = old === true || old === 'true'
  } else if (type === 'date') {
    r.value = /^\d{4}-\d{2}-\d{2}$/.test(String(old)) ? String(old) : ''
  } else {
    r.value = old === null || old === undefined ? '' : String(old)
  }
  emitFromRows()
}

function onSchemaFormUpdate(next: Record<string, any>) {
  suppressNextWatch = true
  emit('update:modelValue', Object.keys(next).length ? next : null)
}

function onJsonBlur() {
  const text = jsonText.value.trim()
  if (!text) {
    jsonError.value = ''
    suppressNextWatch = true
    emit('update:modelValue', null)
    return
  }
  try {
    const parsed = JSON.parse(text)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      jsonError.value = 'Metadata must be a JSON object.'
      return
    }
    jsonError.value = ''
    suppressNextWatch = true
    emit('update:modelValue', Object.keys(parsed).length ? parsed : null)
  } catch (err: any) {
    jsonError.value = err.message
  }
}

// As in MetadataSchemaCard.switchTab: leaving JSON with a parse error is
// refused, rather than discarding what the user typed.
function switchTab(tab: 'form' | 'json') {
  if (tab === activeTab.value) return
  if (activeTab.value === 'json') {
    onJsonBlur()
    if (jsonError.value) {
      toast.error('JSON has errors — fix them before switching views')
      return
    }
    rows.value = rowsFromDoc(doc.value)
  } else {
    refreshJsonFromDoc()
  }
  activeTab.value = tab
}

// Read-only: schema-described fields first, labelled with their schema title.
const viewEntries = computed(() => {
  const entries = Object.entries(doc.value)
  if (!hasSchema.value) return entries.map(([k, v]) => ({ key: k, label: k, value: v }))
  const order = Object.keys(props.schema.properties)
  const rank = (k: string) => {
    const i = order.indexOf(k)
    return i === -1 ? order.length : i
  }
  return entries
    .sort((a, b) => rank(a[0]) - rank(b[0]))
    .map(([k, v]) => ({
      key: k,
      label: props.schema.properties[k]?.title || k,
      value: v,
    }))
})

// Booleans show as `true` / `false`, not Yes/No: firmware and rule-router read
// these values, so show what is stored.
function displayValue(v: any): string {
  if (v === null || v === undefined || v === '') return '—'
  if (typeof v === 'boolean') return v ? 'true' : 'false'
  if (isContainer(v)) return JSON.stringify(v, null, 2)
  return String(v)
}

// The read-only filter appears only above a threshold, so short documents keep
// a clean header.
const viewFilter = ref('')
const FILTER_THRESHOLD = 10

const showViewFilter = computed(() => Object.keys(doc.value).length > FILTER_THRESHOLD)

const filteredViewEntries = computed(() => {
  const q = viewFilter.value.toLowerCase().trim()
  if (!q) return viewEntries.value
  return viewEntries.value.filter(e =>
    e.key.toLowerCase().includes(q) ||
    e.label.toLowerCase().includes(q) ||
    displayValue(e.value).toLowerCase().includes(q),
  )
})

// The parent calls this before submitting, so a JSON tab left mid-edit is
// committed or the save is refused.
function commit(): boolean {
  if (activeTab.value === 'json') {
    onJsonBlur()
    if (jsonError.value) {
      toast.error('Metadata is not valid JSON')
      return false
    }
    return true
  }
  if (rowErrors.value > 0) {
    toast.error('A metadata field contains invalid JSON')
    return false
  }
  return true
}

defineExpose({ commit })
</script>

<template>
  <div>
    <div class="flex items-center justify-between gap-2 mb-4">
      <div role="tablist" class="tabs tabs-bordered">
        <a
          role="tab"
          class="tab"
          :class="{ 'tab-active': activeTab === 'form' }"
          @click="switchTab('form')"
        >{{ disabled ? 'Fields' : 'Form' }}</a>
        <a
          role="tab"
          class="tab"
          :class="{ 'tab-active': activeTab === 'json' }"
          @click="switchTab('json')"
        >JSON</a>
      </div>

      <span v-if="!isEmpty && showCount" class="text-xs text-base-content/50 shrink-0">
        {{ Object.keys(doc).length }} field{{ Object.keys(doc).length === 1 ? '' : 's' }}
      </span>
    </div>

    <!-- ---------- Form / Fields tab ---------- -->
    <template v-if="activeTab === 'form'">
      <!-- Read-only viewer (detail pages). -->
      <template v-if="disabled">
        <div v-if="isEmpty" class="text-sm text-base-content/60 italic">
          No metadata recorded.
        </div>
        <template v-else>
          <input
            v-if="showViewFilter"
            v-model="viewFilter"
            type="text"
            class="input input-bordered input-sm w-full mb-2"
            placeholder="Filter fields…"
          />

          <div
            v-if="filteredViewEntries.length === 0"
            class="text-sm text-base-content/60 italic"
          >
            No field matches “{{ viewFilter }}”.
          </div>

          <!-- Capped from `sm` up so a long document does not push the rest of
               the page down. Unbounded on mobile: a nested scroller traps the
               swipe, and MetadataCard's collapse already solves it there. -->
          <dl v-else class="divide-y divide-base-300 sm:max-h-[500px] sm:overflow-y-auto custom-scrollbar">
            <div
              v-for="e in filteredViewEntries"
              :key="e.key"
              class="py-2 flex flex-col sm:flex-row sm:gap-4 sm:items-baseline"
            >
              <dt class="text-xs font-medium text-base-content/60 sm:w-1/3 shrink-0 font-mono break-all">
                {{ e.label }}
              </dt>
              <dd class="text-sm min-w-0 flex-1">
                <pre
                  v-if="typeof e.value === 'object' && e.value !== null"
                  class="font-mono text-xs bg-base-200 rounded p-2 overflow-x-auto"
                >{{ displayValue(e.value) }}</pre>
                <span v-else class="break-words">{{ displayValue(e.value) }}</span>
              </dd>
            </div>
          </dl>
        </template>
      </template>

      <template v-else>
        <template v-if="hasSchema">
          <JsonSchemaForm
            :schema="schema"
            :model-value="doc"
            @update:model-value="onSchemaFormUpdate"
          />

          <div class="divider text-xs text-base-content/50 my-4">Other fields</div>

          <div v-if="rows.length === 0" class="text-xs text-base-content/60 italic mb-3">
            Anything this type's schema does not list can be added here.
          </div>
        </template>

        <div v-else-if="rows.length === 0" class="text-sm text-base-content/60 italic mb-3">
          No metadata yet. Add a field to record something about this record —
          a service date, an asset tag, a warranty reference.
        </div>

        <!-- Capped from `sm` up, as above. On mobile the header's Save can scroll
             away, hence MetadataCard's bottom action bar. -->
        <div v-if="rows.length" class="space-y-2 mb-3 sm:max-h-[500px] sm:overflow-y-auto custom-scrollbar sm:pr-1">
          <!-- One line on a wide screen, two on a narrow one (four controls do not
               fit). The wrappers are `sm:contents`, so above `sm` the controls
               share one flex line; `sm:order-last` puts delete back on the right. -->
          <div
            v-for="(r, i) in rows"
            :key="i"
            class="flex flex-col gap-2 rounded-lg bg-base-200/40 p-2
                   sm:flex-row sm:items-start sm:rounded-none sm:bg-transparent sm:p-0"
          >
            <div class="flex items-center gap-2 sm:contents">
              <input
                type="text"
                class="input input-bordered input-sm font-mono flex-1 min-w-0"
                :class="{ 'input-error': duplicateKeys.has(r.key.trim()) }"
                placeholder="key"
                :value="r.key"
                @input="setRowKey(i, ($event.target as HTMLInputElement).value)"
              />

              <button
                type="button"
                class="btn btn-ghost btn-square btn-sm shrink-0 sm:order-last"
                title="Remove field"
                aria-label="Remove field"
                @click="removeRow(i)"
              >
                ✕
              </button>
            </div>

            <div class="flex flex-wrap items-start gap-2 sm:contents">
              <select
                class="select select-bordered select-sm w-24 shrink-0 sm:w-28"
                :value="r.type"
                @change="setRowType(i, ($event.target as HTMLSelectElement).value as RowType)"
              >
                <option value="text">Text</option>
                <option value="number">Number</option>
                <option value="boolean">Boolean</option>
                <option value="date">Date</option>
                <option value="json">Object</option>
              </select>

              <!-- An Object row's textarea gets its own line on mobile; beside the
                   select it is too narrow to read. -->
              <div
                class="form-control flex-1 min-w-0"
                :class="{ 'basis-full sm:basis-auto': r.type === 'json' }"
              >
                <!-- The literal stored value, which is what a subscriber reads. -->
                <label v-if="r.type === 'boolean'" class="flex items-center gap-2 h-8">
                  <input
                    type="checkbox"
                    class="toggle toggle-primary"
                    :checked="!!r.value"
                    @change="setRowValue(i, ($event.target as HTMLInputElement).checked)"
                  />
                  <code class="text-xs text-base-content/60">{{ r.value ? 'true' : 'false' }}</code>
                </label>
                <input
                  v-else-if="r.type === 'date'"
                  type="date"
                  class="input input-bordered input-sm w-full"
                  :value="r.value"
                  @input="setRowValue(i, ($event.target as HTMLInputElement).value)"
                />
                <input
                  v-else-if="r.type === 'number'"
                  type="number"
                  step="any"
                  class="input input-bordered input-sm font-mono w-full"
                  :value="r.value"
                  @input="setRowValue(i, numberOrBlank(($event.target as HTMLInputElement).valueAsNumber))"
                />
                <template v-else-if="r.type === 'json'">
                  <textarea
                    class="textarea textarea-bordered textarea-sm font-mono text-xs w-full"
                    :class="{ 'textarea-error': !!r.error }"
                    rows="4"
                    placeholder='{"nested": "value"}'
                    :value="r.text"
                    @input="setRowJson(i, ($event.target as HTMLTextAreaElement).value)"
                  ></textarea>
                  <span v-if="r.error" class="text-[10px] text-error mt-0.5">{{ r.error }}</span>
                </template>
                <input
                  v-else
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="value"
                  :value="r.value"
                  @input="setRowValue(i, ($event.target as HTMLInputElement).value)"
                />
              </div>
            </div>
          </div>
        </div>

        <div v-if="duplicateKeys.size" class="text-xs text-error mb-2">
          Duplicate key{{ duplicateKeys.size === 1 ? '' : 's' }}:
          <code>{{ [...duplicateKeys].join(', ') }}</code> — the last row wins, and a
          row wins over a form field of the same name.
        </div>

        <button type="button" class="btn btn-sm w-full sm:w-auto" @click="addRow">
          + Add Field
        </button>
      </template>
    </template>

    <!-- ---------- JSON tab ---------- -->
    <template v-else>
      <div v-if="disabled">
        <div v-if="isEmpty" class="text-sm text-base-content/60 italic">
          No metadata recorded.
        </div>
        <div v-else class="bg-base-200 rounded-lg p-4 border border-base-300 overflow-hidden">
          <!-- Horizontal scroll at every width; the vertical cap is `sm` up only. -->
          <div class="overflow-x-auto sm:max-h-[500px] sm:overflow-y-auto custom-scrollbar">
            <JsonViewer :data="modelValue" class="text-sm leading-relaxed" />
          </div>
        </div>
      </div>
      <div v-else class="form-control">
        <textarea
          v-model="jsonText"
          class="textarea textarea-bordered font-mono text-xs"
          rows="12"
          placeholder='{"last_service": "2026-03-14"}'
          @blur="onJsonBlur"
        ></textarea>
        <label v-if="jsonError" class="label">
          <span class="label-text-alt text-error">{{ jsonError }}</span>
        </label>
      </div>
    </template>
  </div>
</template>
