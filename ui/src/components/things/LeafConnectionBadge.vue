<script setup lang="ts">
import { computed } from 'vue'
import { formatRelativeTime } from '@/utils/format'
import type { LeafConnection, LeafConnectionState } from '@/composables/useLeafConnections'

const props = defineProps<{
  state: LeafConnectionState
  conn?: LeafConnection
}>()

// `none` is deliberately NOT an error colour. The console cannot tell a gateway
// that should have a leaf from a temperature probe that never will -- there is no
// marker field, on purpose -- so a red dot here would assert a fault on every
// device in the inventory. What it can say for certain is what NATS reports, and
// "no leaf node attached" is a fact either way: an alarm on a gateway's page, a
// shrug on a probe's.
const dotClass = computed(() => ({
  attached: 'bg-success',
  none: 'bg-base-content/30',
  unknown: 'bg-base-content/20',
}[props.state]))

const label = computed(() => ({
  attached: 'Attached',
  none: 'No leaf node attached',
  unknown: 'Unknown',
}[props.state]))

const detail = computed(() => {
  if (props.state === 'unknown') return 'Connect to NATS to read this'
  if (props.state === 'none') return 'The hub reports no leaf connection under this code'
  if (props.conn?.start) return `since ${formatRelativeTime(props.conn.start)}`
  return ''
})
</script>

<template>
  <span class="inline-flex items-center gap-1.5" :title="detail">
    <span class="w-2 h-2 rounded-full shrink-0" :class="dotClass"></span>
    <span class="text-sm">{{ label }}</span>
    <span v-if="detail && state === 'attached'" class="text-xs text-base-content/60">{{ detail }}</span>
  </span>
</template>
