<script setup lang="ts">
/**
 * The Created / Last updated pair for a record's detail surface, so a record
 * can confirm what the activity feed says about it.
 *
 * Relative time on top, exact underneath (as in AuditLogView and ActivityView),
 * not a tooltip: tooltips do not exist on touch devices.
 *
 * Two sibling <div>s, not a <dl>: callers already have one, so they supply it.
 *
 * No "never edited" marker: PocketBase sets `created` and `updated` separately,
 * so they can differ by a millisecond on an untouched record.
 */
import { formatDate, formatRelativeTime } from '@/utils/format'

defineOptions({ inheritAttrs: false })

defineProps<{
  created?: string
  updated?: string
}>()
</script>

<template>
  <div>
    <dt class="text-sm font-medium text-base-content/70">Created</dt>
    <dd v-if="created" class="mt-1">
      <div class="text-sm">{{ formatRelativeTime(created) }}</div>
      <div class="text-xs text-base-content/60">{{ formatDate(created, 'PP pp') }}</div>
    </dd>
    <dd v-else class="mt-1 text-sm text-base-content/40">&mdash;</dd>
  </div>
  <div>
    <dt class="text-sm font-medium text-base-content/70">Last updated</dt>
    <dd v-if="updated" class="mt-1">
      <div class="text-sm">{{ formatRelativeTime(updated) }}</div>
      <div class="text-xs text-base-content/60">{{ formatDate(updated, 'PP pp') }}</div>
    </dd>
    <dd v-else class="mt-1 text-sm text-base-content/40">&mdash;</dd>
  </div>
</template>
