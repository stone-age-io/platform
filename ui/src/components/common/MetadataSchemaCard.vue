<!-- ui/src/components/common/MetadataSchemaCard.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import SchemaBuilder from '@/components/things/SchemaBuilder.vue'
import { inferSchema } from '@/utils/inferSchema'

// MetadataSchemaCard — authors a type's `metadata_schema`: the JSON Schema
// describing what is tracked about each record of that type. Used identically by
// the thing type and location type forms, which is why it is a component rather
// than a copy in each.
//
// Reuses SchemaBuilder + a JSON tab because this IS a JSON Schema document —
// the only difference from any other is where it is stored and what reads it
// (MetadataEditor on the Thing/Location form).
//
// "Infer from sample" came from MessageSchemaFormView, which was removed with
// the message_schemas collection. It was never specific to that collection, and
// pasting a sample beats hand-authoring a schema wherever one is written, so it
// moved here rather than being deleted with its old home.
//
// Emits a normalised value: `{type: 'object', ...}` when there is at least one
// property, or null when there are none. A builder with no properties yields
// `{type: 'object', properties: {}}`, which is not a schema — storing null
// instead is what makes the record form fall back to free-form key/value rows.

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

// The builder only recognises a node that says `type: 'object'`, so an empty or
// reset document has to carry it — otherwise a blank schema is reported as one
// using $ref/anyOf/deep nesting, which is that check's catch-all branch.
const EMPTY_DOC = (): Record<string, any> => ({ type: 'object', properties: {} })

const doc = ref<Record<string, any>>(EMPTY_DOC())
const activeTab = ref<'form' | 'json'>('form')
const jsonText = ref('')
const jsonError = ref('')

// Infer-from-sample modal
const showInferModal = ref(false)
const sampleText = ref('')
const sampleError = ref('')

let suppressNextWatch = false

const fieldCount = computed(() => Object.keys(doc.value?.properties || {}).length)

// Seed the local document from the prop. Only on a genuine external change —
// our own emits are suppressed, or every keystroke in the builder would round
// trip through the parent and reset the cursor.
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
    // Fill in an omitted top-level type. A schema that names its own wins the
    // spread, so this only affects one pasted without it.
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

  // REPLACES the document rather than merging into it. A merge would leave
  // properties from the previous schema that the sample says nothing about,
  // which is a schema nobody authored; the builder is right there for adding
  // fields back.
  doc.value = { type: 'object', ...inferSchema(parsed) }
  refreshJson()
  showInferModal.value = false
  toast.success('Fields inferred from sample — review before saving')
}

// Called by the parent before submit, so a JSON tab left mid-edit is committed
// (or the save refused) rather than silently ignored.
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
