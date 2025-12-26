<script setup lang="ts" generic="T extends { id: string }">
/**
 * Responsive List Component
 * 
 * Shows table on desktop, cards on mobile.
 * Provides consistent UX across all list views.
 * 
 * Usage:
 *   <ResponsiveList 
 *     :items="things" 
 *     :columns="columns"
 *     @row-click="handleRowClick"
 *   >
 *     <template #actions="{ item }">
 *       <button @click="edit(item)">Edit</button>
 *     </template>
 *   </ResponsiveList>
 */

export interface Column<T = any> {
  key: string
  label: string
  format?: (value: any, item: T) => string
  class?: string
  mobileLabel?: string // Label to show in mobile card view
}

interface Props {
  items: T[]
  columns: Column<T>[]
  loading?: boolean
  clickable?: boolean // Make rows/cards clickable
}

const props = withDefaults(defineProps<Props>(), {
  clickable: true // Default to clickable
})

const emit = defineEmits<{
  'row-click': [item: T]
}>()

/**
 * Get nested property from object using dot notation
 * Example: get(item, 'expand.type.name')
 */
function get(obj: any, path: string): any {
  return path.split('.').reduce((acc, part) => acc?.[part], obj)
}

/**
 * Handle row/card click
 */
function handleClick(item: T) {
  if (props.clickable) {
    emit('row-click', item)
  }
}
</script>

<template>
  <div>
    <!-- Desktop Table View (lg and up) -->
    <div class="hidden lg:block overflow-x-auto">
      <table class="table">
        <thead>
          <tr>
            <th v-for="col in columns" :key="col.key" :class="col.class">
              {{ col.label }}
            </th>
            <th v-if="$slots.actions" class="text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr 
            v-for="item in items" 
            :key="item.id" 
            :class="{ 'hover cursor-pointer': clickable }"
            @click="handleClick(item)"
          >
            <td v-for="col in columns" :key="col.key" :class="col.class">
              <slot :name="`cell-${col.key}`" :item="item" :value="get(item, col.key)">
                {{ col.format ? col.format(get(item, col.key), item) : get(item, col.key) || '-' }}
              </slot>
            </td>
            <td v-if="$slots.actions" @click.stop>
              <div class="flex justify-end gap-2">
                <slot name="actions" :item="item" />
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    
    <!-- Mobile Card View (below lg) -->
    <div class="lg:hidden space-y-4">
      <div 
        v-for="item in items" 
        :key="item.id"
        :class="[
          'card bg-base-100 border border-base-300',
          { 'cursor-pointer hover:shadow-md transition-shadow': clickable }
        ]"
        @click="handleClick(item)"
      >
        <div class="card-body p-4">
          <!-- Card content from columns -->
          <div class="space-y-3">
            <div v-for="col in columns" :key="col.key">
              <slot :name="`card-${col.key}`" :item="item" :value="get(item, col.key)">
                <div class="flex flex-col">
                  <span class="text-xs font-medium text-base-content/70">
                    {{ col.mobileLabel || col.label }}
                  </span>
                  <span class="text-sm mt-1">
                    {{ col.format ? col.format(get(item, col.key), item) : get(item, col.key) || '-' }}
                  </span>
                </div>
              </slot>
            </div>
          </div>
          
          <!-- Actions in mobile card -->
          <div v-if="$slots.actions" class="flex gap-2 mt-4 pt-4 border-t border-base-300" @click.stop>
            <slot name="actions" :item="item" />
          </div>
        </div>
      </div>
    </div>
    
    <!-- Empty State -->
    <div v-if="items.length === 0 && !loading" class="text-center py-12 text-base-content/50">
      <slot name="empty">
        No items found
      </slot>
    </div>
  </div>
</template>
