<script setup lang="ts">
import { computed, ref, watch } from 'vue'

// SchemaBuilder — a guided form for building simple JSON Schemas describing
// an `object` with primitive top-level properties. Round-trips to / from the
// schema JSON via v-model. Schemas that use features outside this scope
// (nested objects, $ref, anyOf/oneOf/allOf, ...) show a notice so the user
// knows to switch to the raw JSON view instead of silently losing data.

interface Props {
  modelValue: Record<string, any>
}
const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

type FieldType = 'string' | 'integer' | 'number' | 'boolean' | 'object' | 'array'

interface Field {
  name: string
  type: FieldType
  required: boolean
  description: string
  format: string
  enumText: string
  minimum: string
  maximum: string
}

const TYPES: FieldType[] = ['string', 'integer', 'number', 'boolean', 'object', 'array']
const STRING_FORMATS = ['', 'date-time', 'date', 'time', 'email', 'uri', 'uuid']

function emptyField(): Field {
  return { name: '', type: 'string', required: false, description: '', format: '', enumText: '', minimum: '', maximum: '' }
}

// A schema is form-compatible when it's a simple top-level object and each
// property is a primitive declaration we can round-trip safely.
function isFormCompatible(schema: any): boolean {
  if (!schema || typeof schema !== 'object') return false
  if (schema.type !== 'object') return false
  if (schema.$ref || schema.anyOf || schema.oneOf || schema.allOf) return false
  const props = schema.properties
  if (props == null) return true
  if (typeof props !== 'object') return false
  return Object.values(props).every((raw: any) => {
    if (!raw || typeof raw !== 'object') return false
    if (raw.$ref || raw.anyOf || raw.oneOf || raw.allOf) return false
    if (!TYPES.includes(raw.type)) return false
    // Reject nested object properties (their own `properties`) and typed arrays
    // with complex item shapes — JSON view handles those.
    if (raw.type === 'object' && raw.properties) return false
    if (raw.type === 'array' && raw.items && typeof raw.items === 'object' && raw.items.properties) return false
    return true
  })
}

function schemaToFields(schema: any): Field[] {
  if (!schema?.properties) return []
  const required = new Set<string>(Array.isArray(schema.required) ? schema.required : [])
  return Object.entries(schema.properties).map(([name, raw]: [string, any]) => ({
    name,
    type: (TYPES.includes(raw.type) ? raw.type : 'string') as FieldType,
    required: required.has(name),
    description: raw.description || '',
    format: raw.format || '',
    enumText: Array.isArray(raw.enum) ? raw.enum.join(', ') : '',
    minimum: raw.minimum != null ? String(raw.minimum) : '',
    maximum: raw.maximum != null ? String(raw.maximum) : '',
  }))
}

function fieldsToSchema(list: Field[]): Record<string, any> {
  const properties: Record<string, any> = {}
  const required: string[] = []
  for (const f of list) {
    const name = f.name.trim()
    if (!name) continue
    const entry: Record<string, any> = { type: f.type }
    if (f.description) entry.description = f.description
    if (f.type === 'string' && f.format) entry.format = f.format
    const enumValues = f.enumText.split(',').map(s => s.trim()).filter(Boolean)
    if (enumValues.length > 0) {
      entry.enum = f.type === 'integer' || f.type === 'number'
        ? enumValues.map(v => Number(v)).filter(v => !Number.isNaN(v))
        : enumValues
    }
    if ((f.type === 'integer' || f.type === 'number') && f.minimum !== '') {
      const n = Number(f.minimum)
      if (!Number.isNaN(n)) entry.minimum = n
    }
    if ((f.type === 'integer' || f.type === 'number') && f.maximum !== '') {
      const n = Number(f.maximum)
      if (!Number.isNaN(n)) entry.maximum = n
    }
    properties[name] = entry
    if (f.required) required.push(name)
  }
  const out: Record<string, any> = { type: 'object', properties }
  if (required.length > 0) out.required = required
  return out
}

const compatible = computed(() => isFormCompatible(props.modelValue))
const fields = ref<Field[]>([])
let suppressNextWatch = false

// Sync from external schema → fields (only when form-compatible).
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
}

function removeField(i: number) {
  fields.value.splice(i, 1)
  emitFromFields()
}

function onFieldInput() {
  emitFromFields()
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="!compatible" class="alert alert-warning text-sm">
      <span>
        This schema uses structures the form doesn't handle (nested objects, <code>$ref</code>, or
        <code>anyOf</code>/<code>oneOf</code>/<code>allOf</code>). Switch to <strong>JSON view</strong> to edit it
        without losing anything.
      </span>
    </div>

    <template v-else>
      <div v-if="fields.length === 0" class="text-sm text-base-content/60 italic">
        No properties yet. Add one to describe a field of this message.
      </div>

      <div
        v-for="(f, i) in fields"
        :key="i"
        class="border border-base-300 rounded-lg p-3 space-y-3 bg-base-100"
      >
        <div class="flex flex-wrap items-end gap-2">
          <div class="form-control flex-1 min-w-[10rem]">
            <label class="label py-1"><span class="label-text text-xs">Name *</span></label>
            <input
              v-model="f.name"
              type="text"
              class="input input-bordered input-sm font-mono"
              placeholder="field_name"
              pattern="[a-zA-Z_][a-zA-Z0-9_]*"
              @input="onFieldInput"
            />
          </div>

          <div class="form-control w-32">
            <label class="label py-1"><span class="label-text text-xs">Type</span></label>
            <select v-model="f.type" class="select select-bordered select-sm" @change="onFieldInput">
              <option v-for="t in TYPES" :key="t" :value="t">{{ t }}</option>
            </select>
          </div>

          <label class="label cursor-pointer gap-2">
            <input v-model="f.required" type="checkbox" class="checkbox checkbox-sm" @change="onFieldInput" />
            <span class="label-text text-xs">required</span>
          </label>

          <button type="button" class="btn btn-ghost btn-sm text-error" @click="removeField(i)">
            Remove
          </button>
        </div>

        <div class="form-control">
          <input
            v-model="f.description"
            type="text"
            class="input input-bordered input-sm"
            placeholder="Description (optional)"
            @input="onFieldInput"
          />
        </div>

        <div v-if="f.type === 'string'" class="form-control w-48">
          <label class="label py-1"><span class="label-text text-xs">Format</span></label>
          <select v-model="f.format" class="select select-bordered select-sm" @change="onFieldInput">
            <option v-for="fmt in STRING_FORMATS" :key="fmt" :value="fmt">{{ fmt || '— none —' }}</option>
          </select>
        </div>

        <div v-if="f.type === 'integer' || f.type === 'number'" class="flex flex-wrap gap-2">
          <div class="form-control w-32">
            <label class="label py-1"><span class="label-text text-xs">Minimum</span></label>
            <input v-model="f.minimum" type="number" class="input input-bordered input-sm font-mono" @input="onFieldInput" />
          </div>
          <div class="form-control w-32">
            <label class="label py-1"><span class="label-text text-xs">Maximum</span></label>
            <input v-model="f.maximum" type="number" class="input input-bordered input-sm font-mono" @input="onFieldInput" />
          </div>
        </div>

        <div v-if="f.type !== 'boolean' && f.type !== 'object' && f.type !== 'array'" class="form-control">
          <label class="label py-1">
            <span class="label-text text-xs">Allowed values (optional, comma-separated)</span>
          </label>
          <input
            v-model="f.enumText"
            type="text"
            class="input input-bordered input-sm font-mono"
            placeholder="e.g. low, medium, high"
            @input="onFieldInput"
          />
        </div>
      </div>

      <button type="button" class="btn btn-sm" @click="addField">
        + Add Property
      </button>
    </template>
  </div>
</template>
