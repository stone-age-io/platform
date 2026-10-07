<!-- ui/src/components/things/OperationSubjects.vue -->
<script setup lang="ts">
import { computed, ref } from 'vue'
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

// A LONG list collapses to PREVIEW rows and gains a filter; anything up to LONG
// renders whole with neither. The gap between the two is deliberate: collapsing
// eleven rows to eight hides three, while collapsing nine to eight would put a
// button where one row would have fitted. Collapsed rather than scrolled inside
// a fixed height: a scroll box inside a scrolling page traps the wheel, hides
// rows from the browser's find, and cuts a row in half at an arbitrary line.
// The demo types declare one to six operations, so most pages never see either.
const LONG = 10
const PREVIEW = 8

const isLong = computed(() => rows.value.length > LONG)
const query = ref('')
const expanded = ref(false)

// Matches the name as well as the subject, since the name is not on screen
// (it is the row's tooltip) but is what a type's author called the operation.
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(r =>
    r.subject.toLowerCase().includes(q) ||
    r.name.toLowerCase().includes(q) ||
    r.capability.includes(q),
  )
})

// A filter shows every match: the reader has already narrowed the list, and a
// second "show all" behind it would only hide what they asked for.
const visible = computed(() =>
  !isLong.value || expanded.value || query.value.trim()
    ? filtered.value
    : filtered.value.slice(0, PREVIEW),
)
const hiddenCount = computed(() => filtered.value.length - visible.value.length)

// A subject cut into its tokens, each keeping the dot after it, so the
// template can offer a line break after every dot and nowhere else.
// `break-all` broke anywhere -- `heartbea|t` -- in a column that is
// rarely wide enough for a whole subject. A single token too long for the
// line still breaks, via break-words, rather than overflowing the card.
function pieces(r: { prefix: string; suffix: string }) {
  const cut = (s: string, more: boolean) =>
    s.split('.').map((t, i, all) => (i < all.length - 1 || more ? `${t}.` : t))
  return [
    ...cut(r.prefix, !!r.suffix).map(text => ({ text, shared: true })),
    ...(r.suffix ? cut(r.suffix, false).map(text => ({ text, shared: false })) : []),
  ]
}

const BADGE: Record<string, string> = {
  publish: 'badge-primary',
  request: 'badge-secondary',
  subscribe: 'badge-accent',
  reply: 'badge-neutral',
}
</script>

<template>
  <div>
    <template v-if="rows.length">
      <input
        v-if="isLong"
        v-model="query"
        type="search"
        class="input input-bordered input-sm w-full mb-3"
        :placeholder="`Filter ${rows.length} subjects`"
        aria-label="Filter subjects"
        @keydown.enter.prevent
      />
      <!-- Enter is swallowed because on the Thing Type form this input sits
           inside the form, where Enter would submit it and save the type. -->

      <!--
        One line per operation, in one bordered list. Each used to be its own
        box with the operation name on a line above the subject, about 70px a
        row; a type with twenty operations was a card taller than the page.

        The name moved to the row's tooltip: what a reader copies is the
        subject, and the suffix usually repeats the name anyway.

        The type's prefix is the same on every row, so it is drawn quietly and
        the suffix -- the part that tells rows apart -- plainly. Every piece
        stays in the one <code>, so select-all still copies the WHOLE subject;
        showing the suffix alone would hand a technician half a subject to
        paste. The <wbr> after each dot adds no character to the copy.
      -->
      <ul
        v-if="visible.length"
        class="rounded-lg border border-base-300 bg-base-200 divide-y divide-base-300"
      >
        <li
          v-for="r in visible"
          :key="r.id"
          :title="r.name"
          class="flex items-baseline gap-2 px-3 py-2 min-w-0"
        >
          <!-- Fixed width, so the subjects line up whatever the capability. -->
          <span
            class="badge badge-sm badge-outline shrink-0 w-[4.75rem] justify-center"
            :class="BADGE[r.capability] || ''"
          >
            {{ r.capability }}
          </span>
          <code class="font-mono text-sm break-words select-all min-w-0">
            <template v-for="(p, i) in pieces(r)" :key="i"><span :class="{ 'text-base-content/50': p.shared }">{{ p.text }}</span><wbr /></template>
          </code>
        </li>
      </ul>
      <p v-else class="text-sm text-base-content/60 italic">
        No subject matches “{{ query.trim() }}”.
      </p>

      <button
        v-if="hiddenCount > 0"
        type="button"
        class="btn btn-ghost btn-sm w-full mt-2"
        @click="expanded = true"
      >
        Show all {{ rows.length }}
      </button>
      <button
        v-else-if="isLong && expanded && !query.trim()"
        type="button"
        class="btn btn-ghost btn-sm w-full mt-2"
        @click="expanded = false"
      >
        Show fewer
      </button>
    </template>
    <p v-else class="text-sm text-base-content/60 italic">
      No operations declared, so no subjects to show.
    </p>

    <p v-if="incomplete" class="text-xs text-warning mt-2">
      A <code>{variable}</code> is left in a subject because this Thing has no
      value for it — usually a missing location, or an organization without a code.
    </p>
  </div>
</template>
