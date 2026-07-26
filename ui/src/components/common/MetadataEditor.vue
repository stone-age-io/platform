<!-- ui/src/components/common/MetadataEditor.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import JsonSchemaForm from './JsonSchemaForm.vue'
import { useToast } from '@/composables/useToast'

// MetadataEditor — edits a free-form `metadata` object with a Form / JSON
// toggle, following the contract MessageSchemaFormView already established for
// schema documents: the form is a convenience over the document, the JSON view
// is always the escape hatch, and anything the form cannot represent says so
// instead of silently dropping it.
//
// Two form modes, chosen by whether a schema was supplied:
//
//   schema given → JsonSchemaForm renders typed inputs. This is the
//     thing_types.metadata_schema path: an admin describes the fields tracked
//     for a class of device once, and a member fills in a form.
//
//   no schema → flat key/value rows. The generic fallback, and the only mode
//     available to a type that has no schema (which is every type today).
//
// The model is an OBJECT, not a JSON string. The form views used to hold
// metadata as text and JSON.parse it at submit, which is why an invalid blob
// was only caught after the user hit Save.

interface Props {
  modelValue: Record<string, any> | null
  /** JSON Schema describing the expected fields. Omit for free-form key/value. */
  schema?: any
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), { schema: null, disabled: false })
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any> | null): void
}>()

const toast = useToast()

type RowType = 'text' | 'number' | 'boolean' | 'date'

interface Row {
  key: string
  type: RowType
  value: any
}

const activeTab = ref<'form' | 'json'>('form')
const jsonText = ref('')
const jsonError = ref('')
const rows = ref<Row[]>([])

let suppressNextWatch = false

const doc = computed<Record<string, any>>(() => props.modelValue ?? {})

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

// The key/value form handles flat objects only. A nested object or array has no
// representation as a single input, so we say so and point at the JSON view
// rather than flattening (lossy) or hiding it (worse).
const flatCompatible = computed(() =>
  Object.values(doc.value).every(v => v === null || ['string', 'number', 'boolean'].includes(typeof v)),
)

function inferType(v: any): RowType {
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'number') return 'number'
  // An ISO calendar date round-trips through <input type="date"> unchanged, so
  // treating it as a date is safe; anything else stays text.
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return 'date'
  return 'text'
}

function rowsFromDoc(d: Record<string, any>): Row[] {
  return Object.entries(d).map(([key, value]) => ({
    key,
    type: inferType(value),
    value: value ?? '',
  }))
}

watch(
  () => props.modelValue,
  (v) => {
    if (suppressNextWatch) { suppressNextWatch = false; return }
    const d = v ?? {}
    if (flatCompatible.value) rows.value = rowsFromDoc(d)
    if (activeTab.value !== 'json') refreshJsonFromDoc()
  },
  { immediate: true, deep: true },
)

function refreshJsonFromDoc() {
  jsonText.value = Object.keys(doc.value).length ? JSON.stringify(doc.value, null, 2) : ''
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

function addRow() {
  rows.value.push({ key: '', type: 'text', value: '' })
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

// Changing a row's type re-casts the value it already holds, so switching
// text→number on "42" keeps 42 rather than blanking the field.
function setRowType(i: number, type: RowType) {
  const r = rows.value[i]
  const old = r.value
  r.type = type
  if (type === 'number') {
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

// The parent calls this before submitting so a JSON tab left mid-edit is
// committed (or the save is refused) rather than silently ignored.
function commit(): boolean {
  if (activeTab.value === 'json') {
    onJsonBlur()
    if (jsonError.value) {
      toast.error('Metadata is not valid JSON')
      return false
    }
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
      >Form</a>
      <a
        role="tab"
        class="tab"
        :class="{ 'tab-active': activeTab === 'json' }"
        @click="switchTab('json')"
      >JSON</a>
    </div>

    <!-- ---------- Form tab ---------- -->
    <template v-if="activeTab === 'form'">
      <!-- (b) Schema-driven: typed fields defined on the thing type. -->
      <template v-if="hasSchema">
        <JsonSchemaForm
          :schema="schema"
          :model-value="doc"
          :disabled="disabled"
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
        <div v-if="!flatCompatible" class="alert alert-warning text-sm">
          <span>
            This metadata contains nested objects or arrays, which the form can't show.
            Switch to <strong>JSON</strong> to edit it without losing anything.
          </span>
        </div>

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
                  :disabled="disabled"
                  @input="setRowKey(i, ($event.target as HTMLInputElement).value)"
                />
              </div>

              <select
                class="select select-bordered select-sm w-28 shrink-0"
                :value="r.type"
                :disabled="disabled"
                @change="setRowType(i, ($event.target as HTMLSelectElement).value as RowType)"
              >
                <option value="text">Text</option>
                <option value="number">Number</option>
                <option value="boolean">Yes/No</option>
                <option value="date">Date</option>
              </select>

              <div class="form-control flex-1 min-w-0">
                <input
                  v-if="r.type === 'boolean'"
                  type="checkbox"
                  class="toggle toggle-primary mt-1"
                  :checked="!!r.value"
                  :disabled="disabled"
                  @change="setRowValue(i, ($event.target as HTMLInputElement).checked)"
                />
                <input
                  v-else-if="r.type === 'date'"
                  type="date"
                  class="input input-bordered input-sm w-full"
                  :value="r.value"
                  :disabled="disabled"
                  @input="setRowValue(i, ($event.target as HTMLInputElement).value)"
                />
                <input
                  v-else-if="r.type === 'number'"
                  type="number"
                  step="any"
                  class="input input-bordered input-sm font-mono w-full"
                  :value="r.value"
                  :disabled="disabled"
                  @input="setRowValue(i, numberOrBlank(($event.target as HTMLInputElement).valueAsNumber))"
                />
                <input
                  v-else
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="value"
                  :value="r.value"
                  :disabled="disabled"
                  @input="setRowValue(i, ($event.target as HTMLInputElement).value)"
                />
              </div>

              <button
                type="button"
                class="btn btn-sm btn-ghost btn-square shrink-0"
                :disabled="disabled"
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

          <button type="button" class="btn btn-sm" :disabled="disabled" @click="addRow">
            + Add Field
          </button>
        </template>
      </template>
    </template>

    <!-- ---------- JSON tab ---------- -->
    <div v-else class="form-control">
      <textarea
        v-model="jsonText"
        class="textarea textarea-bordered font-mono text-xs"
        rows="12"
        placeholder='{"last_service": "2026-03-14"}'
        :disabled="disabled"
        @blur="onJsonBlur"
      ></textarea>
      <label v-if="jsonError" class="label">
        <span class="label-text-alt text-error">{{ jsonError }}</span>
      </label>
    </div>
  </div>
</template>
