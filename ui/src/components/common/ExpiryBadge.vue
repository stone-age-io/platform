<!-- ui/src/components/common/ExpiryBadge.vue -->
<script setup lang="ts">
/**
 * Renders nothing unless a credential is expired or close to it.
 *
 * Silence is the common case and the point: a badge that always shows something
 * is a badge nobody reads. See utils/expiry.ts for the threshold and why this
 * lives in the list views rather than in a notification system.
 */
import { computed } from 'vue'
import { expiryState, expiryLabel } from '@/utils/expiry'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  /** ISO / PocketBase timestamp. Absent means "no expiry" and renders nothing. */
  value?: string | null
  size?: 'sm' | 'md'
}>()

const state = computed(() => expiryState(props.value))
const label = computed(() => expiryLabel(props.value))
const title = computed(() => (props.value ? formatDate(props.value) : ''))
</script>

<template>
  <span
    v-if="state === 'expired' || state === 'expiring'"
    class="badge badge-outline gap-1"
    :class="[
      state === 'expired' ? 'badge-error' : 'badge-warning',
      size === 'sm' ? 'badge-sm' : '',
    ]"
    :title="title"
  >
    {{ label }}
  </span>
</template>
