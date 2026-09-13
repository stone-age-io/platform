<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import SchemaFieldEditor from './SchemaFieldEditor.vue'
import {
  type Field,
  emptyField,
  duplicateNames,
  isFormCompatible,
  schemaToFields,
  fieldsToSchema,
} from './schemaFields'

// SchemaBuilder — a guided form for building JSON Schemas describing an
// `object`. Supports nested objects and arrays (including array-of-object) to
// any depth. Schemas that use features outside this scope ($ref,
// anyOf/oneOf/allOf) show a notice so the user knows to switch to the raw JSON
// view instead of silently losing data.

interface Props {
  modelValue: Record<string, any>
}
const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

const compatible = computed(() => isFormCompatible(props.modelValue))
const fields = ref<Field[]>([])
let suppressNextWatch = false

const dupes = computed(() => duplicateNames(fields.value))

watch(
  () => props.modelValue,
  (v) => {
    if (suppressNextWatch) { suppressNextWatch = false; return }
    if (isFormCompatible(v)) fields.value = schemaToFields(v)
  },
  { immediate: true, deep: true },
)

function emitFromFields() {
  suppressNextWatch = true
  emit('update:modelValue', fieldsToSchema(fields.value))
}

function addField() {
  fields.value.push(emptyField())
  emitFromFields()
}

function updateField(i: number, next: Field) {
  fields.value[i] = next
  emitFromFields()
}

function removeField(i: number) {
  fields.value.splice(i, 1)
  emitFromFields()
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="!compatible" class="alert alert-warning text-sm">
      <span>
        This schema uses structures the form doesn't handle (<code>$ref</code>,
        <code>anyOf</code>/<code>oneOf</code>/<code>allOf</code>).
        Switch to <strong>JSON view</strong> to edit it without losing anything.
      </span>
    </div>

    <template v-else>
      <div v-if="fields.length === 0" class="text-sm text-base-content/60 italic">
        No properties yet. Add one to describe a field of this message.
      </div>

      <SchemaFieldEditor
        v-for="(f, i) in fields"
        :key="i"
        :model-value="f"
        :duplicate="dupes.has(f.name.trim())"
        @update:model-value="updateField(i, $event)"
        @remove="removeField(i)"
      />

      <div v-if="dupes.size" class="text-xs text-error">
        Duplicate propert{{ dupes.size === 1 ? 'y' : 'ies' }}:
        <code>{{ [...dupes].join(', ') }}</code> — only the last is saved.
      </div>

      <button type="button" class="btn btn-sm" @click="addField">
        + Add Property
      </button>
    </template>
  </div>
</template>
