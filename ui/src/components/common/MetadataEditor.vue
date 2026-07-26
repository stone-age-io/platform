<!-- ui/src/components/common/MetadataEditor.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import JsonSchemaForm from './JsonSchemaForm.vue'
import JsonViewer from './JsonViewer.vue'
import { useToast } from '@/composables/useToast'

// MetadataEditor — edits (or displays) a free-form `metadata` object with a
// Form / JSON toggle, following the contract MessageSchemaFormView already
// established for schema documents: the form is a convenience over the
// document, the JSON view is always the escape hatch, and anything the form
// cannot represent says so instead of silently dropping it.
//
// Two form modes, chosen by whether a schema was supplied:
//
//   schema given → JsonSchemaForm renders typed inputs. This is the
//     thing_types / location_types `metadata_schema` path: an admin describes
//     the fields tracked for a class of record once, and a member fills in a
//     form.
//
//   no schema → key/value rows. The generic fallback, and the only mode
//     available to a type with no schema.
//
// NESTING is handled per key, not by a recursive editor. A row whose value is
// an object or array gets a small JSON textarea — exactly what JsonSchemaForm
// already does for its own non-primitive properties. A tree editor was the
// obvious alternative and is not worth it: the JSON tab already edits nesting
// perfectly, and the real complaint was never "I can't author a nested object",
// it was that ONE nested key used to disable the row editor for every flat key
// in the document. Per-row typing fixes that; a tree would fix it at ten times
// the size.
//
// With `disabled`, the same component is the read-only viewer used on the
// detail pages, so the locked and unlocked states of a record's metadata cannot
// drift apart in layout or in what they choose to show.
//
// The model is an OBJECT, not a JSON string. The form views used to hold
// metadata as text and JSON.parse it at submit, which is why an invalid blob
// was only caught after the user hit Save.

interface Props {
  modelValue: Record<string, any> | null
  /** JSON Schema describing the expected fields. Omit for free-form key/value. */
  schema?: any
  /** Read-only: renders the viewer used by the detail pages. */
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), { schema: null, disabled: false })
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

// Does this document have a schema to render typed fields from? An empty or
// property-less schema is treated as absent — JsonSchemaForm would render
// nothing and the user would have no way to add a field.
const hasSchema = computed(() => {
  const s = props.schema
  return !!(s && s.type === 'object' && s.properties && Object.keys(s.properties).length > 0)
})

// Keys present on the record but absent from the schema. JsonSchemaForm spreads
// the existing model on every edit so these survive a save — but they are
// invisible in the form, and silently carrying data the user cannot see is the
// same failure the SchemaBuilder notice exists to prevent.
const extraKeys = computed(() => {
  if (!hasSchema.value) return []
  const known = new Set(Object.keys(props.schema.properties))
  return Object.keys(doc.value).filter(k => !known.has(k))
})

function isContainer(v: any): boolean {
  return typeof v === 'object' && v !== null
}

function inferType(v: any): RowType {
  if (isContainer(v)) return 'json'
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'number') return 'number'
  // An ISO calendar date round-trips through <input type="date"> unchanged, so
  // treating it as a date is safe; anything else stays text.
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return 'date'
  return 'text'
}

function rowsFromDoc(d: Record<string, any>): Row[] {
  return Object.entries(d).map(([key, value]) => {
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

watch(
  () => props.modelValue,
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

// Rows → document. Blank keys are skipped (an empty row is a row in progress,
// not an instruction to write ""), and on a duplicate key the last row wins —
// which matches what the JSON view would produce from the same text.
function emitFromRows() {
  const next: Record<string, any> = {}
  for (const r of rows.value) {
    const k = r.key.trim()
    if (!k) continue
    next[k] = r.value
  }
  suppressNextWatch = true
  emit('update:modelValue', Object.keys(next).length ? next : null)
}

const duplicateKeys = computed(() => {
  const seen = new Set<string>()
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

// An emptied number input reports NaN, which is not JSON — store '' so the key
// round-trips as an empty value rather than serialising to null behind the user.
function numberOrBlank(n: number): number | string {
  return Number.isNaN(n) ? '' : n
}

function setRowValue(i: number, value: any) {
  rows.value[i].value = value
  emitFromRows()
}

// `json` rows keep their own text buffer so a half-typed object isn't parsed on
// every keystroke. Parse on input, but only emit when it succeeds — the last
// good value stays in the model, and commit() refuses to save while any row is
// still in error.
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

// Changing a row's type re-casts the value it already holds, so switching
// text→number on "42" keeps 42 rather than blanking the field.
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
  // Leaving `json`: an object has no sensible scalar cast (String() yields
  // "[object Object]"), so start the new type empty rather than write nonsense.
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

// Same semantics as MessageSchemaFormView.switchTab: leaving JSON with a parse
// error is refused, because the alternative is discarding what the user typed.
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

// Read-only rendering. Ordered schema-first so a described field keeps a stable
// position, with its schema title as the label when it has one.
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

function displayValue(v: any): string {
  if (v === null || v === undefined || v === '') return '—'
  if (typeof v === 'boolean') return v ? 'Yes' : 'No'
  if (isContainer(v)) return JSON.stringify(v, null, 2)
  return String(v)
}

// The parent calls this before submitting so a JSON tab left mid-edit is
// committed (or the save is refused) rather than silently ignored.
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
    <div role="tablist" class="tabs tabs-bordered mb-4">
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

    <!-- ---------- Form / Fields tab ---------- -->
    <template v-if="activeTab === 'form'">
      <!-- Read-only viewer (detail pages). Same tab pair as the editor, so the
           locked and unlocked states of a record look like each other. -->
      <template v-if="disabled">
        <div v-if="isEmpty" class="text-sm text-base-content/60 italic">
          No metadata recorded.
        </div>
        <dl v-else class="divide-y divide-base-300">
          <div
            v-for="e in viewEntries"
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

      <!-- (b) Schema-driven: typed fields defined on the type. -->
      <template v-else-if="hasSchema">
        <JsonSchemaForm
          :schema="schema"
          :model-value="doc"
          @update:model-value="onSchemaFormUpdate"
        />

        <div v-if="extraKeys.length" class="alert alert-info text-xs mt-4">
          <span>
            {{ extraKeys.length }} field{{ extraKeys.length === 1 ? '' : 's' }} on this record
            {{ extraKeys.length === 1 ? 'is' : 'are' }} not described by this type's schema
            (<template v-for="(k, i) in extraKeys" :key="k"
              ><code>{{ k }}</code><span v-if="i < extraKeys.length - 1">, </span
            ></template>).
            {{ extraKeys.length === 1 ? 'It is' : 'They are' }} kept on save — switch to
            <strong>JSON</strong> to see or edit {{ extraKeys.length === 1 ? 'it' : 'them' }}.
          </span>
        </div>
      </template>

      <!-- (a) Free-form key/value rows. -->
      <template v-else>
        <div v-if="rows.length === 0" class="text-sm text-base-content/60 italic mb-3">
          No metadata yet. Add a field to record something about this record —
          a service date, an asset tag, a warranty reference.
        </div>

        <div v-else class="space-y-2 mb-3">
          <div v-for="(r, i) in rows" :key="i" class="flex gap-2 items-start">
            <div class="form-control flex-1 min-w-0">
              <input
                type="text"
                class="input input-bordered input-sm font-mono w-full"
                :class="{ 'input-error': duplicateKeys.has(r.key.trim()) }"
                placeholder="key"
                :value="r.key"
                @input="setRowKey(i, ($event.target as HTMLInputElement).value)"
              />
            </div>

            <select
              class="select select-bordered select-sm w-28 shrink-0"
              :value="r.type"
              @change="setRowType(i, ($event.target as HTMLSelectElement).value as RowType)"
            >
              <option value="text">Text</option>
              <option value="number">Number</option>
              <option value="boolean">Yes/No</option>
              <option value="date">Date</option>
              <option value="json">Object</option>
            </select>

            <div class="form-control flex-1 min-w-0">
              <input
                v-if="r.type === 'boolean'"
                type="checkbox"
                class="toggle toggle-primary mt-1"
                :checked="!!r.value"
                @change="setRowValue(i, ($event.target as HTMLInputElement).checked)"
              />
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
              <!-- Nested object / array: a JSON textarea for this key alone. -->
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

            <button
              type="button"
              class="btn btn-sm btn-ghost btn-square shrink-0"
              title="Remove field"
              @click="removeRow(i)"
            >
              ✕
            </button>
          </div>
        </div>

        <div v-if="duplicateKeys.size" class="text-xs text-error mb-2">
          Duplicate key{{ duplicateKeys.size === 1 ? '' : 's' }}:
          <code>{{ [...duplicateKeys].join(', ') }}</code> — the last row wins.
        </div>

        <button type="button" class="btn btn-sm" @click="addRow">
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
          <div class="max-h-[500px] overflow-y-auto overflow-x-auto custom-scrollbar">
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
