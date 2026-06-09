<script setup lang="ts">
import { computed } from 'vue'
import { formatRelativeTime } from '@/utils/format'
import type { LeafStatusState, LeafHeartbeat } from '@/utils/leafStatus'

const props = defineProps<{
  status: LeafStatusState
  hb?: LeafHeartbeat
}>()

const dotClass = computed(() => ({
  online: 'bg-success',
  offline: 'bg-error',
  unknown: 'bg-base-content/30',
}[props.status]))

const label = computed(() => ({
  online: 'Online',
  offline: 'Offline',
  unknown: 'Unknown',
}[props.status]))

const title = computed(() => {
  if (props.status === 'unknown') return 'No live status — connect to NATS'
  if (props.hb?.ts) return `Last heartbeat ${formatRelativeTime(props.hb.ts)}`
  return label.value
})
</script>

<template>
  <span class="inline-flex items-center gap-1.5" :title="title">
    <span class="w-2 h-2 rounded-full shrink-0" :class="dotClass"></span>
    <span class="text-xs text-base-content/70">{{ label }}</span>
  </span>
</template>
