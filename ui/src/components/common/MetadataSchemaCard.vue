<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import SchemaBuilder from '@/components/things/SchemaBuilder.vue'
import { inferSchema } from '@/utils/inferSchema'
import { useEscapeKey } from '@/composables/useEscapeKey'

// Authors a thing or location type's `metadata_schema`, the JSON Schema that
// MetadataEditor renders on the record form.
//
// Emits `{type: 'object', ...}` when there is at least one property, else null.
// Null, not an empty schema, is what makes the record form fall back to
// free-form key/value rows.

interface Props {
  modelValue: Record<string, any> | null
  /** What the described records are, for the help text: "device", "place". */
  noun?: string
}

const props = withDefaults(defineProps<Props>(), { noun: 'record' })
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any> | null): void
}>()

const toast = useToast()

// The builder only recognises a node with `type: 'object'`; without it a blank
// schema is reported as unsupported.
const EMPTY_DOC = (): Record<string, any> => ({ type: 'object', properties: {} })

const doc = ref<Record<string, any>>(EMPTY_DOC())
const activeTab = ref<'form' | 'json'>('form')
const jsonText = ref('')
const jsonError = ref('')

const showInferModal = ref(false)
const sampleText = ref('')
const sampleError = ref('')

let suppressNextWatch = false

const fieldCount = computed(() => Object.keys(doc.value?.properties || {}).length)

// Our own emits are suppressed, or every builder keystroke would round-trip
// through the parent and reset the cursor.
watch(
  () => props.modelValue,
  (v) => {
    if (suppressNextWatch) { suppressNextWatch = false; return }
    doc.value = v ? { type: 'object', ...v } : EMPTY_DOC()
    if (activeTab.value !== 'json') refreshJson()
  },
  { immediate: true, deep: true },
)

watch(doc, () => {
  if (activeTab.value !== 'json') refreshJson()
  emitNormalised()
}, { deep: true })

function refreshJson() {
  jsonText.value = fieldCount.value ? JSON.stringify(doc.value, null, 2) : ''
}

function emitNormalised() {
  suppressNextWatch = true
  emit('update:modelValue', fieldCount.value ? { type: 'object', ...doc.value } : null)
}

function onJsonBlur() {
  const text = jsonText.value.trim()
  if (!text) {
    jsonError.value = ''
    doc.value = EMPTY_DOC()
    return
  }
  try {
    const parsed = JSON.parse(text)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      jsonError.value = 'A schema must be a JSON object.'
      return
    }
    jsonError.value = ''
    // Fills in an omitted top-level type; a schema's own `type` wins the spread.
    doc.value = { type: 'object', ...parsed }
  } catch (err: any) {
    jsonError.value = err.message
  }
}

function switchTab(tab: 'form' | 'json') {
  if (tab === activeTab.value) return
  if (activeTab.value === 'json') {
    onJsonBlur()
    if (jsonError.value) {
      toast.error('JSON has errors — fix them before switching views')
      return
    }
  } else {
    refreshJson()
  }
  activeTab.value = tab
}

function openInferModal() {
  sampleText.value = ''
  sampleError.value = ''
  showInferModal.value = true
}

function applyInferred() {
  let parsed: any
  try {
    parsed = JSON.parse(sampleText.value)
  } catch (err: any) {
    sampleError.value = err.message
    return
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    sampleError.value = 'Paste a JSON object — one example record, not an array or a bare value.'
    return
  }

  // Replaces rather than merges: a merge would keep old properties the sample
  // says nothing about.
  doc.value = { type: 'object', ...inferSchema(parsed) }
  refreshJson()
  showInferModal.value = false
  toast.success('Fields inferred from sample — review before saving')
}

// The parent calls this before submit, so a JSON tab left mid-edit is committed
// or the save is refused.
function commit(): boolean {
  if (activeTab.value === 'json') {
    onJsonBlur()
    if (jsonError.value) {
      toast.error('Metadata schema is not valid JSON')
      return false
    }
  }
  return true
}

defineExpose({ commit })

// See useEscapeKey for why dialogs need this.
useEscapeKey(showInferModal, () => { showInferModal.value = false })
</script>

<template>
  <BaseCard title="Inventory Fields">
    <p class="text-sm text-base-content/70 mb-4">
      Describes what is tracked about each {{ noun }} of this type — a service date,
      an asset tag, a warranty reference. Members filling one in get these as typed
      inputs instead of a JSON blob. Leave empty to let them add free-form fields
      instead.
      <span class="block mt-1 text-base-content/50">
        Rendering only: existing records are never invalidated by a change here, and
        fields not listed are kept.
      </span>
    </p>

    <div class="flex items-end justify-between gap-2 mb-4">
      <div role="tablist" class="tabs tabs-bordered">
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
      <button type="button" class="btn btn-xs btn-ghost" @click="openInferModal">
        Infer from sample
      </button>
    </div>

    <SchemaBuilder v-if="activeTab === 'form'" v-model="doc" />

    <div v-else class="form-control">
      <textarea
        v-model="jsonText"
        class="textarea textarea-bordered font-mono text-xs"
        rows="12"
        placeholder='{"type":"object","properties":{"last_service":{"type":"string","format":"date"}}}'
        @blur="onJsonBlur"
      ></textarea>
      <label v-if="jsonError" class="label">
        <span class="label-text-alt text-error">{{ jsonError }}</span>
      </label>
    </div>

    <dialog class="modal" :class="{ 'modal-open': showInferModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg mb-2">Infer from sample</h3>
        <p class="text-sm text-base-content/70 mb-3">
          Paste one example {{ noun }} as JSON. Every key becomes a field, and every
          key present is marked required — a single sample cannot show which are
          optional, so review the result before saving.
        </p>
        <textarea
          v-model="sampleText"
          class="textarea textarea-bordered font-mono text-xs w-full"
          rows="10"
          placeholder='{"asset_tag":"NW-0142","last_service":"2026-03-14","warranty_months":36}'
        ></textarea>
        <p v-if="sampleError" class="text-error text-xs mt-1">{{ sampleError }}</p>
        <div class="modal-action">
          <button type="button" class="btn btn-ghost" @click="showInferModal = false">Cancel</button>
          <button type="button" class="btn btn-primary" :disabled="!sampleText.trim()" @click="applyInferred">
            Replace fields
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showInferModal = false">
        <button>close</button>
      </form>
    </dialog>
  </BaseCard>
</template>
