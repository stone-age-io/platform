<!-- ui/src/components/ui/ResponsiveList.vue -->
<script setup lang="ts" generic="T extends { id: string }">

import { computed } from 'vue'

export interface Column<T = any> {
  key: string
  label: string
  format?: (value: any, item: T) => string
  class?: string
  mobileLabel?: string
  /** Desktop column width, e.g. '8rem' or '20%'. Optional -- see widthOf(). */
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

// Sorting is server-side for every view that uses it, for the same reason search
// is: sorting the twenty rows already on screen answers a different question than
// the one the reader asked, and does it silently.
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
  <div class="w-full">
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
                  <span class="text-sm">
                    {{ col.format ? col.format(get(item, col.key), item) : get(item, col.key) || '-' }}
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
        <span class="text-[10px] uppercase font-bold opacity-50 tracking-tight shrink-0">Sort</span>
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
        <div class="card-body p-3">
          
          <!-- IDENTITY HEADER (First Column) -->
          <div class="mb-1.5">
            <!-- A `card-` slot wins, then the desktop `cell-` slot, then the raw
                 value. The middle step is the one that matters: a view that
                 renders a column through a cell slot alone -- a status badge, a
                 `Deactivated` marker, a value that is not a record field at all --
                 used to drop straight to the raw value on mobile, which printed
                 `true`, `-`, or nothing for the very columns a card has room for.
                 Falling back to the desktop rendering makes the phone show what
                 the table shows unless a view deliberately says otherwise. -->
            <slot :name="`card-${columns[0].key}`" :item="item" :value="get(item, columns[0].key)">
              <slot :name="`cell-${columns[0].key}`" :item="item" :value="get(item, columns[0].key)">
                <div class="text-sm font-bold text-primary truncate">
                  {{ columns[0].format ? columns[0].format(get(item, columns[0].key), item) : get(item, columns[0].key) || 'Unnamed' }}
                </div>
              </slot>
            </slot>
          </div>

          <!-- METADATA GRID (Remaining Columns) -->
          <div class="grid grid-cols-2 gap-x-3 gap-y-1.5 border-t border-base-200/60 pt-2">
            <div 
              v-for="col in columns.slice(1)" 
              :key="col.key"
              :class="col.class"
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
                    <span class="text-xs font-medium text-base-content/80">
                      {{ col.format ? col.format(get(item, col.key), item) : get(item, col.key) || '-' }}
                    </span>
                  </slot>
                </slot>
              </div>
            </div>
          </div>
          
          <!-- SLIM ACTION BAR -->
          <div v-if="$slots.actions" class="flex justify-end items-center gap-1 mt-2 pt-2 border-t border-base-200/60" @click.stop>
            <slot name="actions" :item="item" />
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
.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.card-body {
  min-height: unset;
}
</style>
