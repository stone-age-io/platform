<script setup lang="ts">
import { computed } from 'vue'

// Minimal JSON Schema form: top-level primitive properties of an object schema.
// Nested objects and arrays fall back to a JSON textarea.

interface Props {
  schema: any
  modelValue: Record<string, any>
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), { disabled: false })
const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, any>): void
}>()

interface Field {
  name: string
  type: string
  format?: string
  required: boolean
  description?: string
  title?: string
  enumValues?: any[]
  minimum?: number
  maximum?: number
}

const fields = computed<Field[]>(() => {
  const s = props.schema
  if (!s || s.type !== 'object' || !s.properties) return []
  const required = new Set<string>(Array.isArray(s.required) ? s.required : [])
  return Object.entries(s.properties).map(([name, raw]: [string, any]) => ({
    name,
    type: raw.type || 'string',
    format: raw.format,
    required: required.has(name),
    description: raw.description,
    title: raw.title,
    enumValues: raw.enum,
    minimum: typeof raw.minimum === 'number' ? raw.minimum : undefined,
    maximum: typeof raw.maximum === 'number' ? raw.maximum : undefined,
  }))
})

function setField(name: string, value: any) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

function inputType(f: Field): string {
  if (f.type === 'boolean') return 'checkbox'
  if (f.type === 'integer' || f.type === 'number') return 'number'
  if (f.format === 'date-time') return 'datetime-local'
  if (f.format === 'date') return 'date'
  if (f.format === 'time') return 'time'
  return 'text'
}

function isPrimitive(f: Field): boolean {
  return ['string', 'integer', 'number', 'boolean'].includes(f.type)
}

// Every control returns a string, so the schema type decides what is stored.
// Firmware and rule-router read this document off the bus, where `"3"` for an
// integer is a type mismatch. The enum <select> goes through this too.
function castOnInput(f: Field, raw: string): any {
  if (f.type === 'integer') {
    const n = parseInt(raw, 10)
    return Number.isNaN(n) ? '' : n
  }
  if (f.type === 'number') {
    const n = parseFloat(raw)
    return Number.isNaN(n) ? '' : n
  }
  return raw
}

// The default step of 1 would make 20.5 invalid and block the form's submit.
function stepFor(f: Field): string {
  return f.type === 'integer' ? '1' : 'any'
}

function rangeHint(f: Field): string {
  const lo = f.minimum, hi = f.maximum
  if (lo !== undefined && hi !== undefined) return `Expected ${lo} to ${hi}`
  if (lo !== undefined) return `Expected at least ${lo}`
  if (hi !== undefined) return `Expected at most ${hi}`
  return ''
}
</script>

<template>
  <div v-if="fields.length === 0" class="text-xs opacity-60 italic">
    Schema has no top-level properties to render.
  </div>
  <div v-else class="space-y-3">
    <div v-for="f in fields" :key="f.name" class="form-control">
      <label class="label py-1">
        <span class="label-text text-xs">
          <code class="font-mono">{{ f.name }}</code>
          <span v-if="f.required" class="text-error ml-1">*</span>
          <span v-if="f.title" class="text-base-content/60 ml-2">{{ f.title }}</span>
        </span>
      </label>

      <select
        v-if="f.enumValues && f.enumValues.length"
        class="select select-bordered select-sm"
        :value="modelValue[f.name] ?? ''"
        :disabled="disabled"
        @change="setField(f.name, castOnInput(f, ($event.target as HTMLSelectElement).value))"
      >
        <option value="">— —</option>
        <option v-for="opt in f.enumValues" :key="String(opt)" :value="opt">{{ opt }}</option>
      </select>

      <!-- Shows the literal true/false, since a NATS subscriber reads the value. -->
      <label v-else-if="f.type === 'boolean'" class="flex items-center gap-2">
        <input
          type="checkbox"
          class="toggle toggle-primary"
          :checked="!!modelValue[f.name]"
          :disabled="disabled"
          @change="setField(f.name, ($event.target as HTMLInputElement).checked)"
        />
        <code class="text-xs text-base-content/60">{{ modelValue[f.name] ? 'true' : 'false' }}</code>
      </label>

      <!-- Deliberately no `required`, `min` or `max`: browser validation would
           block the save, and the schema is a hint (utils/metadataDoc.ts). They
           show as the asterisk and the range text instead. -->
      <input
        v-else-if="isPrimitive(f)"
        :type="inputType(f)"
        class="input input-bordered input-sm font-mono"
        :step="inputType(f) === 'number' ? stepFor(f) : undefined"
        :value="modelValue[f.name] ?? ''"
        :disabled="disabled"
        @input="setField(f.name, castOnInput(f, ($event.target as HTMLInputElement).value))"
      />

      <textarea
        v-else
        class="textarea textarea-bordered font-mono text-xs"
        rows="3"
        :value="typeof modelValue[f.name] === 'string' ? modelValue[f.name] : JSON.stringify(modelValue[f.name] ?? null, null, 2)"
        :disabled="disabled"
        @blur="(e) => {
          try { setField(f.name, JSON.parse((e.target as HTMLTextAreaElement).value)) } catch { /* keep raw string */ setField(f.name, (e.target as HTMLTextAreaElement).value) }
        }"
      ></textarea>

      <div v-if="rangeHint(f)" class="text-[10px] text-base-content/60 mt-0.5">{{ rangeHint(f) }}</div>
      <div v-if="f.description" class="text-[10px] text-base-content/60 mt-0.5">{{ f.description }}</div>
    </div>
  </div>
</template>
