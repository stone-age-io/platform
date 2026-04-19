<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Field, FieldType } from './schemaFields'
import { TYPES, ITEM_TYPES, STRING_FORMATS, emptyField } from './schemaFields'

interface Props {
  modelValue: Field
  depth?: number
  maxDepth?: number
}

const props = withDefaults(defineProps<Props>(), { depth: 0, maxDepth: 4 })

const emit = defineEmits<{
  (e: 'update:modelValue', value: Field): void
  (e: 'remove'): void
}>()

const canNest = computed(() => props.depth < props.maxDepth)
const collapsed = ref(false)

const summary = computed(() => {
  const f = props.modelValue
  if (f.type === 'object') return `{ ${f.children.length} prop${f.children.length === 1 ? '' : 's'} }`
  if (f.type === 'array') {
    const inner = f.itemType === 'object'
      ? `{${f.itemChildren.length}}`
      : f.itemType
    return `[ ${inner} ]`
  }
  if (f.format) return `${f.type} · ${f.format}`
  return f.type
})

function patch(p: Partial<Field>) {
  emit('update:modelValue', { ...props.modelValue, ...p })
}

function addChild() {
  patch({ children: [...props.modelValue.children, emptyField()] })
}
function updateChild(i: number, next: Field) {
  const list = props.modelValue.children.slice()
  list[i] = next
  patch({ children: list })
}
function removeChild(i: number) {
  const list = props.modelValue.children.slice()
  list.splice(i, 1)
  patch({ children: list })
}

function addItemChild() {
  patch({ itemChildren: [...props.modelValue.itemChildren, emptyField()] })
}
function updateItemChild(i: number, next: Field) {
  const list = props.modelValue.itemChildren.slice()
  list[i] = next
  patch({ itemChildren: list })
}
function removeItemChild(i: number) {
  const list = props.modelValue.itemChildren.slice()
  list.splice(i, 1)
  patch({ itemChildren: list })
}
</script>

<template>
  <div class="border border-base-300 rounded-lg p-3 space-y-3 bg-base-100">
    <div class="flex flex-wrap items-end gap-2">
      <button
        type="button"
        class="btn btn-ghost btn-xs btn-square self-end mb-1"
        :title="collapsed ? 'Expand' : 'Collapse'"
        @click="collapsed = !collapsed"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-3 h-3 transition-transform"
          :class="{ 'rotate-90': !collapsed }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
        </svg>
      </button>

      <div class="form-control flex-1 min-w-[10rem]">
        <label class="label py-1"><span class="label-text text-xs">Name *</span></label>
        <input
          :value="modelValue.name"
          type="text"
          class="input input-bordered input-sm font-mono"
          placeholder="field_name"
          pattern="[a-zA-Z_][a-zA-Z0-9_]*"
          @input="patch({ name: ($event.target as HTMLInputElement).value })"
        />
      </div>

      <div class="form-control w-32">
        <label class="label py-1"><span class="label-text text-xs">Type</span></label>
        <select
          :value="modelValue.type"
          class="select select-bordered select-sm"
          @change="patch({ type: ($event.target as HTMLSelectElement).value as FieldType })"
        >
          <option v-for="t in TYPES" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>

      <label class="label cursor-pointer gap-2">
        <input
          type="checkbox"
          class="checkbox checkbox-sm"
          :checked="modelValue.required"
          @change="patch({ required: ($event.target as HTMLInputElement).checked })"
        />
        <span class="label-text text-xs">required</span>
      </label>

      <button type="button" class="btn btn-ghost btn-sm text-error" @click="emit('remove')">
        Remove
      </button>
    </div>

    <div v-if="collapsed" class="text-xs text-base-content/60 font-mono pl-8">
      {{ summary }}
    </div>

    <template v-if="!collapsed">
    <div class="form-control">
      <input
        :value="modelValue.description"
        type="text"
        class="input input-bordered input-sm"
        placeholder="Description (optional)"
        @input="patch({ description: ($event.target as HTMLInputElement).value })"
      />
    </div>

    <div v-if="modelValue.type === 'string'" class="form-control w-48">
      <label class="label py-1"><span class="label-text text-xs">Format</span></label>
      <select
        :value="modelValue.format"
        class="select select-bordered select-sm"
        @change="patch({ format: ($event.target as HTMLSelectElement).value })"
      >
        <option v-for="fmt in STRING_FORMATS" :key="fmt" :value="fmt">{{ fmt || '— none —' }}</option>
      </select>
    </div>

    <div v-if="modelValue.type === 'integer' || modelValue.type === 'number'" class="flex flex-wrap gap-2">
      <div class="form-control w-32">
        <label class="label py-1"><span class="label-text text-xs">Minimum</span></label>
        <input
          :value="modelValue.minimum"
          type="number"
          class="input input-bordered input-sm font-mono"
          @input="patch({ minimum: ($event.target as HTMLInputElement).value })"
        />
      </div>
      <div class="form-control w-32">
        <label class="label py-1"><span class="label-text text-xs">Maximum</span></label>
        <input
          :value="modelValue.maximum"
          type="number"
          class="input input-bordered input-sm font-mono"
          @input="patch({ maximum: ($event.target as HTMLInputElement).value })"
        />
      </div>
    </div>

    <div
      v-if="modelValue.type === 'string' || modelValue.type === 'integer' || modelValue.type === 'number'"
      class="form-control"
    >
      <label class="label py-1">
        <span class="label-text text-xs">Allowed values (optional, comma-separated)</span>
      </label>
      <input
        :value="modelValue.enumText"
        type="text"
        class="input input-bordered input-sm font-mono"
        placeholder="e.g. low, medium, high"
        @input="patch({ enumText: ($event.target as HTMLInputElement).value })"
      />
    </div>

    <!-- Nested: object -->
    <div v-if="modelValue.type === 'object'" class="pl-3 border-l-2 border-base-300 space-y-2">
      <div class="text-xs font-semibold uppercase tracking-wide opacity-60">Properties</div>
      <div v-if="modelValue.children.length === 0" class="text-xs text-base-content/60 italic">
        No properties yet.
      </div>
      <SchemaFieldEditor
        v-for="(child, i) in modelValue.children"
        :key="i"
        :model-value="child"
        :depth="depth + 1"
        :max-depth="maxDepth"
        @update:model-value="updateChild(i, $event)"
        @remove="removeChild(i)"
      />
      <button
        v-if="canNest"
        type="button"
        class="btn btn-xs"
        @click="addChild"
      >
        + Add Property
      </button>
      <div v-else class="text-xs text-warning">
        Max nesting depth reached. Use the JSON view to go deeper.
      </div>
    </div>

    <!-- Nested: array -->
    <div v-if="modelValue.type === 'array'" class="pl-3 border-l-2 border-base-300 space-y-2">
      <div class="flex flex-wrap items-end gap-2">
        <div class="form-control w-32">
          <label class="label py-1"><span class="label-text text-xs">Item Type</span></label>
          <select
            :value="modelValue.itemType"
            class="select select-bordered select-sm"
            @change="patch({ itemType: ($event.target as HTMLSelectElement).value as FieldType })"
          >
            <option v-for="t in ITEM_TYPES" :key="t" :value="t">{{ t }}</option>
          </select>
        </div>
        <div v-if="modelValue.itemType === 'string'" class="form-control w-48">
          <label class="label py-1"><span class="label-text text-xs">Item Format</span></label>
          <select
            :value="modelValue.itemFormat"
            class="select select-bordered select-sm"
            @change="patch({ itemFormat: ($event.target as HTMLSelectElement).value })"
          >
            <option v-for="fmt in STRING_FORMATS" :key="fmt" :value="fmt">{{ fmt || '— none —' }}</option>
          </select>
        </div>
      </div>

      <template v-if="modelValue.itemType === 'object'">
        <div class="text-xs font-semibold uppercase tracking-wide opacity-60">Item Properties</div>
        <div v-if="modelValue.itemChildren.length === 0" class="text-xs text-base-content/60 italic">
          No properties yet.
        </div>
        <SchemaFieldEditor
          v-for="(child, i) in modelValue.itemChildren"
          :key="i"
          :model-value="child"
          :depth="depth + 1"
          :max-depth="maxDepth"
          @update:model-value="updateItemChild(i, $event)"
          @remove="removeItemChild(i)"
        />
        <button
          v-if="canNest"
          type="button"
          class="btn btn-xs"
          @click="addItemChild"
        >
          + Add Property
        </button>
        <div v-else class="text-xs text-warning">
          Max nesting depth reached. Use the JSON view to go deeper.
        </div>
      </template>
    </div>
    </template>
  </div>
</template>
