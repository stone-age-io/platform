<!-- ui/src/components/common/RecordPicker.vue -->
<script setup lang="ts">
/**
 * A filterable picker for a relation field: one component, two surfaces.
 *
 * WHY NOT A NATIVE <select>. Native selects win for enums -- free type-ahead,
 * free keyboard nav, a real OS picker on mobile -- and they stay in use for
 * every fixed set defined in code. They lose for relations, which grow with
 * tenant data, and they lose hardest for locations: the tree needs indentation,
 * and native type-ahead matches from the START of the option text, so an option
 * rendered as "   L Room 12" cannot be reached by typing "Room". Indentation and
 * type-ahead are mutually exclusive in a native select. That is the whole reason
 * this exists.
 *
 * WHY TWO SURFACES. Desktop wants light: a popover leaves the form's scroll
 * position untouched, where an inline panel pushes 300px of fields down and
 * snaps them back on select. Mobile wants the opposite: a popover in a
 * keyboard-shrunk viewport leaves ~120px of usable list, so it becomes a
 * full-screen sheet. Same idiom as LocationMapDrawer.
 *
 * It needs no Teleport, unlike the sidebar's org switcher: daisyUI's .card is
 * position:relative with no overflow clipping, so an absolute panel anchors to
 * the card and scrolls with it for free. The sidebar teleports because a compact
 * 64px rail has to escape itself -- that is a sidebar problem, not a popover one.
 */
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import type { PickerOption } from '@/types/picker'

const props = withDefaults(defineProps<{
  /** A string, or an array of them when `multiple`. */
  modelValue: string | string[]
  options: PickerOption[]
  /** Noun for the header and the filter placeholder, e.g. "Location". */
  title?: string
  placeholder?: string
  /**
   * Select many. The panel stays open and the filter survives each pick, since
   * choosing several things from one search is the whole point; selections show
   * as removable chips under the trigger. Replaces `<select multiple>`, whose
   * "hold Ctrl/Cmd" affordance is undiscoverable and unusable on a touchscreen.
   */
  multiple?: boolean
  /** Offer an explicit "none" row that clears the field. Ignored when `multiple` -- the chips remove themselves. */
  clearable?: boolean
  clearLabel?: string
  /** Shown in place of the list when there is nothing to choose from. */
  emptyText?: string
  disabled?: boolean
}>(), {
  title: 'Option',
  placeholder: 'Select...',
  multiple: false,
  clearable: false,
  clearLabel: 'None',
  emptyText: 'Nothing to choose from yet.',
  disabled: false,
})

const emit = defineEmits<{ (e: 'update:modelValue', value: string | string[]): void }>()

const isMobile = useMediaQuery('(max-width: 767px)')

const open = ref(false)
const query = ref('')
const activeIndex = ref(0)

const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const searchRef = ref<HTMLInputElement | null>(null)
const listRef = ref<HTMLElement | null>(null)

const filtering = computed(() => query.value.trim() !== '')

const selectedIds = computed<string[]>(() => {
  if (Array.isArray(props.modelValue)) return props.modelValue
  return props.modelValue ? [props.modelValue] : []
})

function isSelected(id: string): boolean {
  return props.multiple ? selectedIds.value.includes(id) : props.modelValue === id
}

/** Single-select only: the one option backing the current value, if resolvable. */
const selected = computed(() =>
  props.multiple ? undefined : props.options.find(o => o.id === props.modelValue)
)

/** Chips, in the order the options are listed rather than the order picked. */
const selectedOptions = computed(() =>
  props.multiple ? props.options.filter(o => selectedIds.value.includes(o.id)) : []
)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter(o =>
    o.label.toLowerCase().includes(q) || (o.sublabel || '').toLowerCase().includes(q)
  )
})

/**
 * The clear row is part of the list so the keyboard can reach it. It is hidden
 * while filtering so an empty result reads as "no matches" rather than "one
 * match named None".
 */
const rows = computed<PickerOption[]>(() => {
  if (props.clearable && !props.multiple && !filtering.value) {
    return [{ id: '', label: props.clearLabel }, ...filtered.value]
  }
  return filtered.value
})

/**
 * A hierarchical option carries its ancestors two ways -- indentation while
 * browsing, path while filtering -- and shows exactly one at a time. A flat
 * option's sublabel is a different fact (an IP, a code) and always shows.
 */
function showSublabel(opt: PickerOption): boolean {
  if (!opt.sublabel) return false
  return filtering.value || !opt.depth
}

function indentFor(opt: PickerOption): string {
  if (filtering.value || !opt.depth) return '0'
  return `${opt.depth}rem`
}

function choose(opt: PickerOption) {
  if (opt.disabled) return

  if (props.multiple) {
    // Stay open and keep the filter: picking four operations out of one search
    // is the reason this mode exists.
    const next = selectedIds.value.includes(opt.id)
      ? selectedIds.value.filter(id => id !== opt.id)
      : [...selectedIds.value, opt.id]
    emit('update:modelValue', next)
    return
  }

  emit('update:modelValue', opt.id)
  close()
  triggerRef.value?.focus()
}

function remove(id: string) {
  if (props.disabled) return
  emit('update:modelValue', selectedIds.value.filter(x => x !== id))
}

function close() {
  open.value = false
  query.value = ''
}

async function openPanel() {
  open.value = true
  query.value = ''

  const current = rows.value.findIndex(o => isSelected(o.id))
  activeIndex.value = current >= 0 ? current : firstEnabled()

  await nextTick()
  searchRef.value?.focus()
  if (!isMobile.value) panelRef.value?.scrollIntoView({ block: 'nearest' })
  scrollActiveIntoView()
}

function toggle() {
  if (props.disabled) return
  if (open.value) close()
  else openPanel()
}

function firstEnabled(): number {
  const i = rows.value.findIndex(o => !o.disabled)
  return i >= 0 ? i : 0
}

/** Clamps at both ends rather than wrapping -- a list that jumps from the last
 *  row back to the first reads as a glitch on a long location tree. */
function move(step: number) {
  const list = rows.value
  let i = activeIndex.value
  for (let n = 0; n < list.length; n++) {
    i += step
    if (i < 0 || i >= list.length) return
    if (!list[i].disabled) {
      activeIndex.value = i
      scrollActiveIntoView()
      return
    }
  }
}

function scrollActiveIntoView() {
  nextTick(() => {
    const row = listRef.value?.children[activeIndex.value] as HTMLElement | undefined
    row?.scrollIntoView({ block: 'nearest' })
  })
}

function onKeydown(e: KeyboardEvent) {
  if (props.disabled) return

  if (!open.value) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      openPanel()
    }
    return
  }

  switch (e.key) {
    case 'Escape':
      e.preventDefault()
      close()
      triggerRef.value?.focus()
      break
    case 'ArrowDown':
      e.preventDefault()
      move(1)
      break
    case 'ArrowUp':
      e.preventDefault()
      move(-1)
      break
    case 'Enter': {
      // Also stops the Enter from submitting the surrounding form.
      e.preventDefault()
      const opt = rows.value[activeIndex.value]
      if (opt) choose(opt)
      break
    }
    case 'Tab':
      close()
      break
  }
}

// A new filter invalidates the highlight.
watch(query, () => {
  activeIndex.value = firstEnabled()
})

function onClickOutside(e: Event) {
  if (!open.value) return
  const target = e.target as Node
  if (triggerRef.value?.contains(target)) return
  if (panelRef.value?.contains(target)) return
  close()
}

onMounted(() => window.addEventListener('click', onClickOutside))
onUnmounted(() => window.removeEventListener('click', onClickOutside))
</script>

<template>
  <div @keydown="onKeydown">
    <div class="relative">
    <!-- Trigger: styled as .select so it sits flush with the native selects
         still used for enums elsewhere on the same form. -->
    <button
      ref="triggerRef"
      type="button"
      class="select select-bordered w-full items-center text-left overflow-hidden"
      :class="{ 'select-disabled': disabled }"
      :disabled="disabled"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="toggle"
    >
      <span v-if="multiple" class="truncate block w-full" :class="{ 'text-base-content/50': selectedIds.length === 0 }">
        {{ selectedIds.length === 0 ? placeholder : `${selectedIds.length} selected` }}
      </span>
      <span v-else-if="selected" class="truncate block w-full">
        {{ selected.label }}
        <span v-if="selected.sublabel" class="text-base-content/50">· {{ selected.sublabel }}</span>
      </span>
      <!-- A value we hold but cannot resolve. Showing the placeholder here would
           read as "unset" and quietly clear the field on the next save. -->
      <span v-else-if="modelValue" class="truncate block w-full font-mono text-xs text-base-content/60">
        {{ modelValue }}
      </span>
      <span v-else class="truncate block w-full text-base-content/50">{{ placeholder }}</span>
    </button>

    <div
      v-if="open"
      ref="panelRef"
      class="flex flex-col bg-base-100 border-base-300 shadow-xl"
      :class="isMobile
        ? 'fixed inset-0 z-[9999]'
        : 'absolute left-0 right-0 top-full mt-1 z-30 rounded-box border max-h-80'"
    >
      <!-- The sheet needs a way out that is not "tap the backdrop"; there is none. -->
      <div
        v-if="isMobile"
        class="flex items-center justify-between gap-2 px-4 h-14 flex-shrink-0 border-b border-base-200"
      >
        <span class="font-semibold truncate">{{ title }}</span>
        <button type="button" class="btn btn-sm btn-ghost btn-circle" aria-label="Close" @click="close">
          ✕
        </button>
      </div>

      <div v-if="options.length > 0" class="p-2 flex-shrink-0 border-b border-base-200">
        <input
          ref="searchRef"
          v-model="query"
          type="text"
          class="input input-sm input-bordered w-full"
          :placeholder="`Filter ${title.toLowerCase()}...`"
        />
      </div>

      <div ref="listRef" class="flex-1 overflow-y-auto scrollbar-thin" role="listbox">
        <button
          v-for="(opt, i) in rows"
          :key="opt.id || '__clear__'"
          type="button"
          role="option"
          :aria-selected="isSelected(opt.id)"
          :disabled="opt.disabled"
          class="w-full text-left px-3 py-2 flex items-center gap-2 border-b border-base-200/50 disabled:opacity-40 disabled:cursor-not-allowed"
          :class="i === activeIndex ? 'bg-base-200' : 'hover:bg-base-200/60'"
          @click="choose(opt)"
          @mousemove="activeIndex = i"
        >
          <span
            class="w-2 h-2 flex-shrink-0"
            :class="[
              multiple ? 'rounded-sm' : 'rounded-full',
              isSelected(opt.id) ? 'bg-primary' : 'bg-transparent border border-base-content/20',
            ]"
          ></span>
          <span class="min-w-0 flex-1" :style="{ marginLeft: indentFor(opt) }">
            <span class="block truncate text-sm">{{ opt.label }}</span>
            <span v-if="showSublabel(opt)" class="block truncate text-xs text-base-content/60">
              {{ opt.sublabel }}
            </span>
          </span>
        </button>

        <div v-if="options.length === 0" class="px-4 py-6 text-center text-sm text-base-content/60">
          {{ emptyText }}
        </div>
        <div v-else-if="rows.length === 0" class="px-4 py-6 text-center text-sm text-base-content/60">
          No matches for "{{ query.trim() }}"
        </div>
      </div>

      <div v-if="$slots.footer" class="p-1 flex-shrink-0 border-t border-base-300">
        <slot name="footer" :close="close" />
      </div>
    </div>
    </div>

    <!-- Chips sit outside the trigger because a button cannot contain buttons,
         and each needs its own remove control. -->
    <div v-if="multiple && selectedOptions.length > 0" class="flex flex-wrap gap-1 mt-2">
      <span v-for="opt in selectedOptions" :key="opt.id" class="badge badge-neutral gap-1 max-w-full">
        <span class="truncate">{{ opt.label }}</span>
        <button
          v-if="!disabled"
          type="button"
          class="opacity-60 hover:opacity-100 flex-shrink-0"
          :aria-label="`Remove ${opt.label}`"
          @click="remove(opt.id)"
        >
          ✕
        </button>
      </span>
    </div>
  </div>
</template>
