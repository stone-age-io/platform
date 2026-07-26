<!-- ui/src/components/common/MetadataSchemaCard.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import SchemaBuilder from '@/components/things/SchemaBuilder.vue'

// MetadataSchemaCard — authors a type's `metadata_schema`: the JSON Schema
// describing what is tracked about each record of that type. Used identically by
// the thing type and location type forms, which is why it is a component rather
// than a copy in each.
//
// Reuses SchemaBuilder + a JSON tab, the same pair MessageSchemaFormView uses,
// because this IS a JSON Schema document — the only difference is where it is
// stored and what reads it (MetadataEditor on the Thing/Location form).
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

const doc = ref<Record<string, any>>({})
const activeTab = ref<'form' | 'json'>('form')
const jsonText = ref('')
const jsonError = ref('')

let suppressNextWatch = false

const fieldCount = computed(() => Object.keys(doc.value?.properties || {}).length)

// Seed the local document from the prop. Only on a genuine external change —
// our own emits are suppressed, or every keystroke in the builder would round
// trip through the parent and reset the cursor.
watch(
  () => props.modelValue,
  (v) => {
    if (suppressNextWatch) { suppressNextWatch = false; return }
    doc.value = v ? { ...v } : {}
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
    doc.value = {}
    return
  }
  try {
    const parsed = JSON.parse(text)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      jsonError.value = 'A schema must be a JSON object.'
      return
    }
    jsonError.value = ''
    doc.value = parsed
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
  </BaseCard>
</template>
