<script setup lang="ts" generic="T extends { id: string }">

import { computed } from 'vue'
import { Inbox } from '@lucide/vue'

import { NULL_DISPLAY } from '@/utils/tableColumns'

export interface Column<T = any> {
  key: string
  label: string
  format?: (value: any, item: T) => string
  class?: string
  mobileLabel?: string
  /**
   * Give this field the whole width of the mobile card instead of half (about
   * thirteen characters on a phone). Use it for values with no length bound: a
   * record name, an email address, a NATS subject. Also free for the half-width
   * field directly before a wide one, since that field already owns its row.
   * Not for emphasis: each wide field costs card height.
   */
  cardWide?: boolean
  /**
   * Desktop column width, e.g. '8rem' or '20%'. See widthOf().
   *
   * The table is `table-fixed`, so un-widthed columns split the leftover space
   * equally. Width every column whose content has a known size and leave exactly
   * one free-text column to absorb the slack; if every column is widthed, the
   * fixed ones absorb it unpredictably.
   *
   * Widths in use:
   *   7rem   a single badge, a count, a short enum ('Active', 'Owner')
   *   8rem   a date in `PP` form ('Aug 31, 2026')
   *   11rem  a `code` value (~18 characters)
   *   16rem  a person: a name or an email address
   *   9rem   the Actions column, set on the <th> below
   * Cells with two or more badges are left un-widthed so they can wrap.
   *
   * A description is not a column: it goes under the name in the identity cell,
   * clamped to one line with the full text on `title`. That makes the identity
   * column the absorber, so give it `width: 'auto'`. Omitting the width is not
   * the same: column 0 then defaults to 28%.
   */
  width?: string
  /**
   * The API field to sort by; setting it makes the column sortable. A field name,
   * not the column key: the key is a display path (`expand.type.name`), the
   * server wants `type.name`. A leading `-` sorts descending first (PocketBase
   * syntax, passed to the API verbatim).
   *
   * Only name a field the collection has: an unknown one is a 400 with a generic
   * message, for superusers too. `scripts/check-sort-fields.sh` checks every
   * `sortable:` value against a real server.
   */
  sortable?: string
}

interface Props {
  items: T[]
  columns: Column<T>[]
  loading?: boolean
  clickable?: boolean
  /**
   * The active sort in PocketBase syntax (`name`, `-created`); empty is unsorted.
   * This component only reports changes; the view owns the reload.
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

// NULL_DISPLAY keeps "no value" the same here and in the dashboard table
// widgets. Blank cells are also dimmed, so a dash does not read as data.
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

// `table-fixed`, so widths come from the <th>, not the widest cell, and the
// table keeps its shape from page to page. Overflowing content wraps (the
// `break-words` wrapper). Widths go on the <th>, not a <colgroup>, so a column
// hidden at a breakpoint stops reserving space.
function widthOf(col: Column<T>, index: number): string | undefined {
  // Column 0 is always the identity column, so it gets a larger default share.
  return col.width ?? (index === 0 ? '28%' : undefined)
}

// Sorting is server-side for PocketBase-backed views, which hold one page, so a
// browser sort would order only that page. The JetStream lists hold the whole
// set and sort it with `utils/clientSort` before slicing. Either way this
// component only reports the change.
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

// Toggles between two directions, with no unsorted third state. The first click
// uses the column's declared direction (dates open newest-first).
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
  <!-- tabular-nums on the root reaches table and cards; scanned columns are numeric. -->
  <div class="w-full tabular-nums">
    <!-- Desktop: table -->
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
            <!-- Actions holds at most two buttons, so a fixed 9rem is enough. -->
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
    
    <!-- Mobile: cards -->
    <div class="lg:hidden space-y-2">
      <!-- Touch-only sort row, sized up to 44px by `.sort-touch` below. Not `h-11`:
           `select-xs` and `btn-xs` also set `min-height`, which would win. -->
      <div v-if="sortableColumns.length" class="sort-touch flex items-center gap-2 pb-1">
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
        <!-- gap-1.5 overrides daisyUI's `.card-body` gap, which adds to any margin. -->
        <div class="card-body p-3 gap-1.5">
          
          <!-- Identity (first column) with the row's actions beside it, which costs
               no card height. Keep an Edit button even where it goes where the
               card tap goes: on touch it is the only sign the row is actionable.
               min-w-0 lets the text shrink, or the buttons are pushed off the card. -->
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0 flex-1">
              <!-- A `card-` slot wins, then the desktop `cell-` slot, then the raw
                   value, so the card shows what the table shows by default. -->
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

          <!-- Remaining columns -->
          <div class="card-fields-touch grid grid-cols-2 gap-x-3 gap-y-0.5 border-t border-base-200/60 pt-2">
            <div 
              v-for="col in columns.slice(1)" 
              :key="col.key"
              :class="[col.class, col.cardWide ? 'col-span-2' : '']"
              class="flex items-center gap-1.5 overflow-hidden"
            >
              <span class="text-[10px] uppercase font-bold opacity-50 tracking-tight shrink-0">
                {{ col.mobileLabel || col.label }}:
              </span>

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
    
    <div v-if="items.length === 0 && !loading" class="text-center py-12 bg-base-200/30 rounded-box border-2 border-dashed border-base-300">
      <slot name="empty">
        <div class="flex flex-col items-center gap-2 opacity-40">
          <Inbox :size="36" :stroke-width="1.5" />
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
  Touch targets: slotted `btn-xs` buttons in a mobile card (actions and field
  cells) grow to 44px. Not btn-sm (32px, below both platform guidelines), and
  not an invisible enlarged hit area: the whole card is tappable, so a hidden
  target would steal taps meant for the card.

  `:deep()` because slotted content is compiled in the parent's scope.
*/
.card-actions-touch :deep(.btn),
.card-fields-touch :deep(.btn) {
  height: 44px;
  min-height: 44px;
  min-width: 44px;
}

/* The sort row: same 44px rule. Own elements, so no `:deep()`. */
.sort-touch .select,
.sort-touch .btn {
  height: 44px;
  min-height: 44px;
}
.sort-touch .btn {
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
