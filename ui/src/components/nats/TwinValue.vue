<!-- ui/src/components/nats/TwinValue.vue -->
<script setup lang="ts">
/**
 * Compact, typed rendering of one twin value. Same visual language as the KV
 * browser's flat list — a raw JSON.stringify of an object is unreadable in a
 * table row, and a bare `true` reads as a string when it isn't one.
 *
 * Full values live in the detail modal; this is the at-a-glance form.
 */
import { computed } from 'vue'

const props = defineProps<{
  value?: any
  /** No entry exists on this side. Distinct from a value that IS null. */
  missing?: boolean
}>()

const isObject = computed(() => props.value !== null && typeof props.value === 'object')

/** Objects and arrays get a shape summary rather than their serialised text. */
const objectSummary = computed(() => {
  const v = props.value
  if (Array.isArray(v)) return `[${v.length} item${v.length === 1 ? '' : 's'}]`
  const n = Object.keys(v ?? {}).length
  return `{${n} field${n === 1 ? '' : 's'}}`
})
</script>

<template>
  <span v-if="missing" class="opacity-30 select-none">—</span>

  <span
    v-else-if="typeof value === 'boolean'"
    class="badge badge-sm"
    :class="value ? 'badge-success' : 'badge-ghost'"
  >{{ value ? 'TRUE' : 'FALSE' }}</span>

  <span v-else-if="value === null" class="badge badge-sm badge-ghost">null</span>

  <span v-else-if="typeof value === 'number'" class="font-mono text-sm font-medium">{{ value }}</span>

  <span
    v-else-if="isObject"
    class="font-mono text-xs opacity-70"
    :title="JSON.stringify(value)"
  >{{ objectSummary }}</span>

  <span v-else class="text-sm truncate block">{{ value }}</span>
</template>
