<!-- ui/src/components/common/RecordTimestamps.vue -->
<script setup lang="ts">
/**
 * The Created / Last updated pair for a record's detail surface.
 *
 * It exists for the activity feed. The feed says "someone updated Thing X
 * twenty minutes ago"; the record itself has to be able to confirm that, and a
 * detail view showing only `created` cannot -- it leaves the reader comparing a
 * feed entry against nothing. Every collection the feed covers
 * (hooks/activity.go) carries both autodate fields, so every one of their
 * detail surfaces can show both.
 *
 * RELATIVE ON TOP, EXACT UNDERNEATH, which is the pairing AuditLogView already
 * uses for its When column and the one ActivityView uses. Deliberately not a
 * `title` tooltip for the exact value: correlating two screens is the whole job
 * here, and a tooltip does not exist on a touch device.
 *
 * TWO SIBLING <div>s, NOT a <dl>. Every caller already has a description list
 * and nesting one inside another is invalid HTML, so the caller supplies the
 * container and its grid. A form view that has no <dl> wraps this in one.
 *
 * NO "never edited" MARKER. PocketBase's autodate fields each call
 * types.NowDateTime() separately (core/field_autodate.go), so `created` and
 * `updated` on a record nobody has touched can differ by a millisecond and
 * `created === updated` would be a coin flip. The feed answers that question
 * properly: no `updated` entry means nobody updated it.
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
