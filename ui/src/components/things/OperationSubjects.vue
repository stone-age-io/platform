<!-- ui/src/components/things/OperationSubjects.vue -->
<script setup lang="ts">
import { computed } from 'vue'
import {
  resolveOperationSubjects,
  hasUnresolved,
  type OperationLike,
  type ThingContext,
} from '@/utils/subjectResolver'

// The full subject of every operation a Thing Type declares, resolved for one
// context. One component for the Thing page and the type form, so the two cannot
// disagree about what a device speaks on.
//
// The capability is read from the DEVICE's side: `publish` is a subject the
// device sends on, `subscribe` one it listens on. That is the side a technician
// standing at the device is on, and the side the operations are written from.
//
// This is a description, not a permission. Nothing derives the device's NATS
// role from it, so a subject listed here can still be refused by the bus.

const props = withDefaults(defineProps<{
  prefix?: string | null
  operations: OperationLike[]
  context: ThingContext
  /**
   * Resolving for one real Thing. A leftover `{variable}` then means a value
   * was missing, which is worth saying; on the type form it is the template.
   */
  forThing?: boolean
}>(), { prefix: '', forThing: false })

const rows = computed(() => resolveOperationSubjects(props.prefix, props.operations, props.context))

const incomplete = computed(() => props.forThing && rows.value.some(r => hasUnresolved(r.subject)))

const BADGE: Record<string, string> = {
  publish: 'badge-primary',
  request: 'badge-secondary',
  subscribe: 'badge-accent',
  reply: 'badge-neutral',
}
</script>

<template>
  <div>
    <ul v-if="rows.length" class="space-y-2">
      <li
        v-for="r in rows"
        :key="r.id"
        class="bg-base-200 rounded-lg p-3 border border-base-300"
      >
        <div class="flex items-center gap-2 mb-1 min-w-0">
          <span class="badge badge-sm badge-outline shrink-0" :class="BADGE[r.capability] || ''">
            {{ r.capability }}
          </span>
          <span class="text-xs text-base-content/60 truncate">{{ r.name }}</span>
        </div>
        <code class="font-mono text-sm break-all select-all">{{ r.subject }}</code>
      </li>
    </ul>
    <p v-else class="text-sm text-base-content/60 italic">
      No operations declared, so no subjects to show.
    </p>

    <p v-if="incomplete" class="text-xs text-warning mt-2">
      A <code>{variable}</code> is left in a subject because this Thing has no
      value for it — usually a missing location, or an organization without a code.
    </p>
  </div>
</template>
