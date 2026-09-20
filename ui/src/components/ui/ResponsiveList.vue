<!-- ui/src/components/ui/ResponsiveList.vue -->
<script setup lang="ts" generic="T extends { id: string }">

import { computed } from 'vue'

import { NULL_DISPLAY } from '@/utils/tableColumns'

export interface Column<T = any> {
  key: string
  label: string
  format?: (value: any, item: T) => string
  class?: string
  mobileLabel?: string
  /**
   * Give this field the whole width of the mobile card instead of half of it.
   *
   * The card lays its fields out two to a row, which on a 393px phone (iPhone
   * 14 Pro, and the narrowest common size) leaves each value between 105px and
   * 129px depending on how long its label is -- about thirteen characters. That
   * is plenty for what most of these columns hold: a date, a count, an enum, a
   * badge, an IP, a CIDR, a code. It is not close to enough for the ones that
   * hold a NAME: 'Kansas City Distribution Center' showed 62% of itself and cut
   * off mid-word.
   *
   * So the rule is about the VALUE, not the column's importance: a field whose
   * value is another record's name, an email address or a NATS subject has no
   * length bound and takes the full row. Everything else pairs up.
   *
   * Do not reach for this to make a field stand out. Every wide field is a row
   * the short fields no longer share, and a card with all of them wide is just
   * the one-column layout, which was measured and rejected: forcing one column
   * on the KV bucket list cost 36px a card to widen five values that already
   * fit.
   */
  cardWide?: boolean
  /**
   * Desktop column width, e.g. '8rem' or '20%'. Optional -- see widthOf().
   *
   * The table is `table-fixed`, so a column WITHOUT a width does not shrink to
   * its content: the un-widthed columns split whatever is left over, equally.
   * Declare a width on every column whose content has a known size and leave the
   * free-text one -- the description, the subject, the email -- to absorb the
   * slack. Get that backwards and the slack lands on the column with the least
   * to say: Thing Types gave its Operations badge 746px of a 1570px table while
   * Code, capped at 7rem next to it, wrapped every code longer than fourteen
   * characters.
   *
   * Widths in use, so a new column has an answer rather than a guess:
   *   7rem   a single badge, a count, a short enum ('Active', 'Owner')
   *   8rem   a date in `PP` form ('Aug 31, 2026')
   *   11rem  a `code` value -- fits the ~18 characters real codes reach
   *   16rem  a person -- a name or an email address
   *   9rem   the Actions column, set on the <th> below rather than here
   * Two or more badges in one cell (StatusBadge + ExpiryBadge) need the room to
   * wrap and are deliberately left un-widthed.
   *
   * Always leave exactly one absorber, and be deliberate about which. If EVERY
   * column is widthed the browser still has to place the excess, and where it
   * puts it is not where you would guess: a PERCENTAGE column does not grow, so
   * the fixed columns soak it up instead. Widthing the five short columns on the
   * KV bucket list moved them 226px -> 238px, i.e. nowhere, while Created and
   * Actions quietly doubled.
   *
   * A record's DESCRIPTION is not a column. It goes under the name in the
   * identity cell, clamped to one line, with the full text on `title` -- which
   * is what ten of the twelve lists carrying one already did. The two that
   * spent a column on it cost more than the column: it competed for width with
   * Code and Operations, which have known sizes and no slack to give, and it
   * rode a `class: 'hidden md:table-cell'` that hid the description outright
   * below 768px while every other list showed it on a phone. Under the name it
   * sits beside the thing it describes, it is the absorber the table wants
   * anyway, and it still answers the question it was put on screen to answer --
   * why this row matched the search.
   *
   * So the free-text column IS the identity column on nearly every list, and
   * there you say `width: 'auto'` on it. That is not the same as omitting the
   * width: omitting it means 28% for column 0, per widthOf() below. With 'auto'
   * every other column gets exactly what it declares and the name takes the
   * rest.
   */
  width?: string
  /**
   * The API field to sort this column by -- its presence is what makes the
   * column sortable. It is a field NAME and not the column key on purpose: the
   * key is a display path (`expand.type.name`) while the server wants
   * `type.name`, and the two are not mechanically convertible for every column.
   *
   * A leading `-` means "sort this one descending first", which is what you want
   * for dates. That is PocketBase's own sort syntax rather than a second
   * convention, and it means the value here can be handed to the API verbatim.
   *
   * Only name a field the collection actually has. An unknown sort field is a
   * 400 raised before any API rule is evaluated, it fails for superusers too, and
   * the browser shows only "Something went wrong while processing your request."
   * -- so it never looks like a sorting bug. `scripts/check-sort-fields.sh`
   * checks every value in this file against a real server.
   */
  sortable?: string
}

interface Props {
  items: T[]
  columns: Column<T>[]
  loading?: boolean
  clickable?: boolean
  /**
   * The active sort, in PocketBase's own syntax (`name`, `-created`). Empty
   * means unsorted. This component renders the control and reports the change;
   * the view owns the reload, because only the view knows the rest of the query.
   */
  sort?: string
}

const props = withDefaults(defineProps<Props>(), {
  clickable: true
})

const emit = defineEmits<{
  'row-click': [item: T]
  'update:sort': [value: string]
}>()

function get(obj: any, path: string): any {
  return path.split('.').reduce((acc, part) => acc?.[part], obj)
}

// A cell's text, and whether what came back was nothing.
//
// The placeholder is NULL_DISPLAY rather than a local literal so the list
// tables and the dashboard table widgets agree on what "no value" looks like --
// this used to print "-" here and "—" there, in the same theme, three clicks
// apart. Blank cells also render at a third of the contrast: in a table with a
// sparse column, a full-strength dash reads as data and is the thing your eye
// lands on, which is exactly backwards.
//
// Two functions rather than one returning a pair, because a template cannot
// destructure a call. Both are cheap and neither touches the DOM.
function formatted(col: Column<T>, item: T): unknown {
  const raw = get(item, col.key)
  return col.format ? col.format(raw, item) : raw
}

function cellBlank(col: Column<T>, item: T): boolean {
  const out = formatted(col, item)
  return out === null || out === undefined || out === ''
}

function cellText(col: Column<T>, item: T): string {
  const out = formatted(col, item)
  return cellBlank(col, item) ? NULL_DISPLAY : String(out)
}

// The desktop table is `table-fixed`: a column's width comes from its <th>, NOT
// from the widest cell under it. Under the default `auto` layout a single long
// description set the width of the whole column -- so the same table had a
// different shape on every page of results, and every column to its right got
// squeezed until short values like a date wrapped onto two lines.
//
// The width lives on the <th> rather than in a <colgroup> deliberately: a column
// hidden at a breakpoint (`class: 'hidden xl:table-cell'`) takes its <th> out of
// the layout and the remaining columns re-share the space, where a <col> would
// go on reserving width for a column nobody can see.
//
// The cost of fixed layout is that content which does not fit has to wrap rather
// than push the column open, which is what the `break-words` wrapper on each
// cell is for.
function widthOf(col: Column<T>, index: number): string | undefined {
  // Column 0 is the identity column everywhere in this app -- the mobile card
  // below already treats it that way -- so it gets the larger default share.
  return col.width ?? (index === 0 ? '28%' : undefined)
}

// Sorting is server-side for every PocketBase-backed view, for the same reason
// search is: those views hold ONE PAGE, so ordering it in the browser sorts the
// twenty rows it happens to have rather than the set the reader asked about, and
// says nothing about the difference.
//
// The two JetStream lists are the exception, and not a relaxation of that rule
// but the same rule reaching a different answer: they hold the WHOLE set, since
// listStreams() and listKvBuckets() return everything in the account and the
// paging is a slice taken afterwards. `utils/clientSort` orders the full set and
// the slice comes after. This component neither knows nor cares which kind it is
// talking to -- it renders the control and reports the change, and the view owns
// what happens next.
const sortableColumns = computed(() => props.columns.filter(c => c.sortable))

/** The field name without its direction prefix. */
function bare(value?: string): string {
  return value ? value.replace(/^-/, '') : ''
}

const sortDesc = computed(() => props.sort?.startsWith('-') ?? false)

function isSorted(col: Column<T>): boolean {
  return !!col.sortable && !!props.sort && bare(col.sortable) === bare(props.sort)
}

function sortGlyph(col: Column<T>): string {
  if (!isSorted(col)) return '↕'
  return sortDesc.value ? '↓' : '↑'
}

// Two states, not three. A third "unsorted" state in the cycle means three
// clicks to get back where you started and an order nobody asked for in between.
// The first click uses the column's declared direction, which is why dates open
// newest-first.
function toggleSort(col: Column<T>) {
  if (!col.sortable) return
  if (!isSorted(col)) {
    emit('update:sort', col.sortable)
    return
  }
  emit('update:sort', sortDesc.value ? bare(col.sortable) : '-' + bare(col.sortable))
}

function selectSort(event: Event) {
  const field = (event.target as HTMLSelectElement).value
  const col = props.columns.find(c => bare(c.sortable) === field)
  emit('update:sort', col?.sortable ?? '')
}

function flipSort() {
  emit('update:sort', sortDesc.value ? bare(props.sort) : '-' + bare(props.sort))
}

function handleClick(item: T) {
  if (props.clickable) {
    emit('row-click', item)
  }
}
</script>

<template>
  <!--
    tabular-nums on the root, so it reaches the desktop table and the mobile
    cards from one place (font-variant-numeric inherits). Every column in this
    app that a reader scans DOWN is numeric -- dates, byte counts, stream
    sequences, revisions, RTT -- and proportional digits make those ragged,
    which is most of why a dense table looks unsettled. It only changes digit
    advance widths, so slotted text is unaffected.
  -->
  <div class="w-full tabular-nums">
    <!-- 1. DESKTOP VIEW: Table -->
    <div class="hidden lg:block overflow-x-auto">
      <table class="table table-sm w-full table-fixed">
        <thead>
          <tr class="border-b border-base-300">
            <th
              v-for="(col, i) in columns"
              :key="col.key"
              :class="col.class"
              :style="{ width: widthOf(col, i) }"
              :aria-sort="!col.sortable ? undefined : isSorted(col) ? (sortDesc ? 'descending' : 'ascending') : 'none'"
              class="text-[11px] uppercase tracking-wider"
            >
              <button
                v-if="col.sortable"
                type="button"
                class="inline-flex items-center gap-1 uppercase tracking-wider hover:text-base-content"
                :class="isSorted(col) ? 'text-base-content' : 'text-base-content/60'"
                @click="toggleSort(col)"
              >
                {{ col.label }}
                <span class="text-[10px]" :class="isSorted(col) ? '' : 'opacity-50'">{{ sortGlyph(col) }}</span>
              </button>
              <span v-else class="text-base-content/60">{{ col.label }}</span>
            </th>
            <!-- Actions never holds more than two buttons (Edit/Delete/View, and
                 View only renders when the other two do not), so a fixed 9rem is
                 enough and hands the leftover width back to the content. -->
            <th v-if="$slots.actions" class="w-36 text-right text-[11px] uppercase tracking-wider text-base-content/60">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr 
            v-for="item in items" 
            :key="item.id" 
            :class="{ 'hover cursor-pointer': clickable }"
            class="border-b border-base-200/50 last:border-0"
            @click="handleClick(item)"
          >
            <td v-for="col in columns" :key="col.key" :class="col.class" class="py-3">
              <div class="min-w-0 break-words">
                <slot :name="`cell-${col.key}`" :item="item" :value="get(item, col.key)">
                  <span class="text-sm" :class="{ 'text-base-content/40': cellBlank(col, item) }">
                    {{ cellText(col, item) }}
                  </span>
                </slot>
              </div>
            </td>
            <td v-if="$slots.actions" @click.stop class="py-3">
              <div class="flex justify-end gap-2">
                <slot name="actions" :item="item" />
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    
    <!-- 2. MOBILE VIEW: High-Density Cards -->
    <div class="lg:hidden space-y-2">
      <div v-if="sortableColumns.length" class="flex items-center gap-2 pb-1">
        <span class="text-[10px] uppercase font-bold text-base-content/50 tracking-tight shrink-0">Sort</span>
        <select
          class="select select-xs select-bordered flex-1"
          aria-label="Sort by"
          :value="bare(sort)"
          @change="selectSort"
        >
          <option value="">Default</option>
          <option v-for="col in sortableColumns" :key="col.key" :value="bare(col.sortable)">
            {{ col.label }}
          </option>
        </select>
        <button
          v-if="bare(sort)"
          type="button"
          class="btn btn-xs shrink-0"
          :aria-label="sortDesc ? 'Sort ascending' : 'Sort descending'"
          @click="flipSort"
        >
          {{ sortDesc ? '↓' : '↑' }}
        </button>
      </div>

      <div 
        v-for="item in items" 
        :key="item.id"
        :class="[
          'card bg-base-100 border border-base-300 shadow-sm transition-all duration-200',
          { 'cursor-pointer active:scale-[0.98] hover:border-primary/40': clickable }
        ]"
        @click="handleClick(item)"
      >
        <!--
          gap-1.5 overrides daisyUI's own `.card-body { gap: .5rem }`, which is
          easy to miss: it applies on top of any margin written here, so the old
          `mb-1.5` on the identity block and `mt-2` on the action bar were each
          adding to 8px that was already there rather than setting it.
        -->
        <div class="card-body p-3 gap-1.5">
          
          <!-- IDENTITY HEADER (First Column), with the row's actions beside it.
               They used to sit in a full-width bar of their own under the
               metadata grid, which cost 44px of every card -- a border, two
               paddings, a card-body gap and a 24px button -- to say "Edit".
               Up here the row is already at least as tall as the button, so
               the actions are free: measured over four Things at 390px, the
               card went from 194px to 150px on that move alone, and taking the
               button out entirely from there saves nothing at all. That last
               part is worth knowing, because three list views point their Edit
               button at exactly where tapping the card already goes, and the
               tempting fix is to delete it. Don't: on a card there is no hover
               and no cursor, so that button is the only thing announcing the
               row is actionable, and it now costs nothing to keep.

               min-w-0 on the text side is load-bearing -- without it a flex
               child refuses to shrink below its content and the buttons get
               pushed off the card instead. -->
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0 flex-1">
              <!-- A `card-` slot wins, then the desktop `cell-` slot, then the
                   raw value. The middle step is the one that matters: a view
                   that renders a column through a cell slot alone -- a status
                   badge, a `Deactivated` marker, a value that is not a record
                   field at all -- used to drop straight to the raw value on
                   mobile, which printed `true`, `-`, or nothing for the very
                   columns a card has room for. Falling back to the desktop
                   rendering makes the phone show what the table shows unless a
                   view deliberately says otherwise. -->
              <slot :name="`card-${columns[0].key}`" :item="item" :value="get(item, columns[0].key)">
                <slot :name="`cell-${columns[0].key}`" :item="item" :value="get(item, columns[0].key)">
                  <div class="text-sm font-bold text-primary truncate">
                    {{ columns[0].format ? columns[0].format(get(item, columns[0].key), item) : get(item, columns[0].key) || 'Unnamed' }}
                  </div>
                </slot>
              </slot>
            </div>

            <div v-if="$slots.actions" class="card-actions-touch flex items-center gap-1 shrink-0" @click.stop>
              <slot name="actions" :item="item" />
            </div>
          </div>

          <!-- METADATA GRID (Remaining Columns) -->
          <div class="grid grid-cols-2 gap-x-3 gap-y-0.5 border-t border-base-200/60 pt-2">
            <div 
              v-for="col in columns.slice(1)" 
              :key="col.key"
              :class="[col.class, col.cardWide ? 'col-span-2' : '']"
              class="flex items-center gap-1.5 overflow-hidden"
            >
              <!-- Fixed Label: Now handled by ResponsiveList only -->
              <span class="text-[10px] uppercase font-bold opacity-50 tracking-tight shrink-0">
                {{ col.mobileLabel || col.label }}:
              </span>

              <!-- Value Slot: Now only handles the value part -->
              <div class="flex-1 truncate">
                <slot :name="`card-${col.key}`" :item="item" :value="get(item, col.key)">
                  <slot :name="`cell-${col.key}`" :item="item" :value="get(item, col.key)">
                    <span class="text-xs font-medium" :class="cellBlank(col, item) ? 'text-base-content/40' : 'text-base-content/80'">
                      {{ cellText(col, item) }}
                    </span>
                  </slot>
                </slot>
              </div>
            </div>
          </div>
                  </div>
      </div>
    </div>
    
    <!-- EMPTY & LOADING STATES -->
    <div v-if="items.length === 0 && !loading" class="text-center py-12 bg-base-200/30 rounded-xl border-2 border-dashed border-base-300">
      <slot name="empty">
        <div class="flex flex-col items-center gap-2 opacity-40">
          <span class="text-4xl">📭</span>
          <span class="text-sm font-bold uppercase tracking-widest">No items found</span>
        </div>
      </slot>
    </div>

    <div v-if="loading" class="flex justify-center p-4">
      <span class="loading loading-dots loading-md opacity-30"></span>
    </div>
  </div>
</template>

<style scoped>
/*
  Touch targets. A row's action button is written `btn btn-xs` for the desktop
  table, where a mouse is pointing at it; in the mobile card that is a 39x24px
  target, and both platform guidelines want 44x44 (iOS) or 48x48 (Android).
  This block sizes it up for the card only -- the desktop table is a separate
  element above and never matches.

  It costs about 15px a card, measured over a mix of rows with and without a
  description: 110px average becomes 125px. That is real, and it was worth
  paying rather than taking either of the two cheaper answers, because both are
  worse than they look:

    - btn-sm (32px) costs 6px and meets NEITHER guideline. It buys the feeling
      of having addressed this and none of the substance.
    - Keeping the 24px button and growing only its HIT area with a transparent
      ::after costs nothing and was the tempting one. It is wrong here for a
      reason specific to this layout: an invisible target is free when it sits
      in dead space, and this one sits inside a LARGER competing target -- the
      whole card is tappable. Growing it invisibly means a tap 10px above the
      button, on what looks like the card, silently opens the edit form
      instead. It makes the primary action unreliable exactly where the
      secondary one is, and nothing on screen explains why.

  `:deep()` because the button is slotted, so it is compiled in the parent's
  scope and a plain scoped selector would not reach it.
*/
.card-actions-touch :deep(.btn) {
  height: 44px;
  min-height: 44px;
  min-width: 44px;
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.card-body {
  min-height: unset;
}
</style>
