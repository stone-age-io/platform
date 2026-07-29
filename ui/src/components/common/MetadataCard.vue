<!-- ui/src/components/common/MetadataCard.vue -->
<script setup lang="ts">
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataEditor from './MetadataEditor.vue'

// MetadataCard — the metadata block on a detail page, with quick edit.
//
// Locked, it is a MetadataEditor with `disabled` set, so the read view and the
// edit view are the same component and cannot drift apart in layout or in which
// fields they show. Unlocked, the same editor becomes writable in place.
//
// The save sends ONLY `metadata`. That is not just economy: things.updateRule
// admits a member through a branch requiring `nats_user:changed = false` and
// `nebula_host:changed = false`, and a field absent from the body counts as
// unchanged — so a partial update is exactly what keeps a member's edit legal.
// Sending the whole record back is what would turn an inventory edit into a 404.

interface Props {
  collection: 'things' | 'locations'
  recordId: string
  modelValue: Record<string, any> | null
  /** metadata_schema from the record's type, if it has one. */
  schema?: any
  /** Whether this caller may edit inventory (authStore.can.manageInventory). */
  canEdit?: boolean
}

const props = withDefaults(defineProps<Props>(), { schema: null, canEdit: false })
const emit = defineEmits<{
  (e: 'saved', metadata: Record<string, any> | null): void
}>()

const toast = useToast()

const editing = ref(false)
const saving = ref(false)
const draft = ref<Record<string, any> | null>(null)
const editor = ref<InstanceType<typeof MetadataEditor> | null>(null)

const isEmpty = computed(() => !props.modelValue || Object.keys(props.modelValue).length === 0)
const fieldCount = computed(() => Object.keys(props.modelValue || {}).length)

// This is the app's first collapsible card, so the pattern lives here rather
// than in BaseCard. Generalising on the first instance is how you get an
// abstraction shaped by one caller; if a second card wants this, that is when it
// moves.
//
// The reason for collapsing is mobile, not desktop. On a wide screen the grid's
// other column usually sets the row height anyway (the location page's map card
// is min-h-[550px], taller than this card's 500px cap, so collapsing frees
// nothing there). At `grid-cols-1` there is no other column: metadata sits
// directly between Basic Information and everything below it, Live State
// included, and 500px of it is 500px of scroll.
//
// Which is why the initial state is a function of size rather than a constant:
// collapsing earns its click on the 11-field record it was written for, and on a
// two-field one it is a click to reveal two lines. Read once at setup, not a
// computed — after that the state belongs to the reader. (Deliberately not
// persisted: remembering it per record is the version of this that needs a
// store, and nobody has asked for it.)
const AUTO_EXPAND_MAX = 4
const expanded = ref(fieldCount.value <= AUTO_EXPAND_MAX)

function startEdit() {
  // Editing a collapsed card would otherwise show nothing. One click, not two.
  expanded.value = true
  // Deep clone so Cancel genuinely discards — the editor mutates nested values
  // in place for `json` rows.
  draft.value = props.modelValue ? JSON.parse(JSON.stringify(props.modelValue)) : null
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  draft.value = null
}

async function save() {
  // Commit a JSON tab or nested-object row left mid-edit; refuses on a parse
  // error rather than saving a stale value.
  if (editor.value && !editor.value.commit()) return

  saving.value = true
  try {
    await pb.collection(props.collection).update(props.recordId, {
      metadata: draft.value,
    })
    toast.success('Metadata saved')
    emit('saved', draft.value)
    editing.value = false
    draft.value = null
  } catch (err: any) {
    toast.error(err.response?.message || err.message || 'Failed to save metadata')
  } finally {
    saving.value = false
  }
}

async function copyMetadata() {
  if (isEmpty.value) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(props.modelValue, null, 2))
    toast.success('Metadata copied to clipboard')
  } catch {
    toast.error('Failed to copy')
  }
}
</script>

<template>
  <BaseCard>
    <template #header>
      <div class="flex justify-between items-center mb-2 gap-2">
        <!-- The title is the disclosure control. The field count travels with it
             so a collapsed card is never a blind click — you can see whether
             there is anything in there without opening it.
             `flex-1` so the target is all the header width the actions don't
             claim, rather than hugging the text: a ~150px hit area next to two
             small buttons is a bad aim on touch and reads as a heading, not a
             control, on desktop. Hence also the hover tint and 44px minimum. -->
        <button
          type="button"
          class="flex flex-1 items-center gap-2 min-w-0 text-left -mx-2 px-2 rounded-lg
                 min-h-[44px] sm:min-h-0 sm:py-1
                 hover:bg-base-200 focus-visible:outline-none focus-visible:ring-2
                 focus-visible:ring-primary/50 transition-colors"
          :aria-expanded="expanded"
          aria-controls="metadata-body"
          @click="expanded = !expanded"
        >
          <!-- An SVG chevron rather than a `▶` glyph, whose weight and baseline
               drift with the platform font.

               The rotation is on this wrapping span and NOT on the <svg>, where
               it silently did nothing: with `rotate-90` on the svg the computed
               transform stayed the identity matrix and a 45° test rotation left
               its box 14px wide, while the same class on a span rotated it (16.5px
               → 23px at 45°). Don't "simplify" this by dropping the wrapper — the
               failure is invisible in code review and the chevron just stops
               moving. -->
          <span
            class="flex shrink-0 opacity-50 transition-transform duration-150"
            :class="{ 'rotate-90': expanded }"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path
                fill-rule="evenodd"
                d="M7.21 5.23a.75.75 0 011.06-.02l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 11-1.03-1.09L11.16 10 7.23 6.29a.75.75 0 01-.02-1.06z"
                clip-rule="evenodd"
              />
            </svg>
          </span>
          <h3 class="card-title text-base">Metadata</h3>
          <span class="text-xs text-base-content/50 shrink-0">
            {{ isEmpty ? 'empty' : `${fieldCount} field${fieldCount === 1 ? '' : 's'}` }}
          </span>
        </button>

        <!-- btn-sm on mobile, btn-xs from `sm`: a 24px Save is too small to hit
             with a thumb, and these are the controls you reach for most. -->
        <div class="flex items-center gap-1 shrink-0">
          <button
            v-if="!editing && !isEmpty"
            class="btn btn-sm sm:btn-xs btn-ghost gap-1 px-2 sm:px-3 opacity-70 hover:opacity-100"
            title="Copy raw JSON"
            @click="copyMetadata"
          >
            <!-- Icon only on mobile. Growing these to btn-sm for thumbs cost the
                 disclosure button 45px of the header row, and of the three labels
                 up here "Copy" is the one a 📋 already says. -->
            📋<span class="hidden sm:inline">Copy</span>
          </button>

          <button
            v-if="!editing && canEdit"
            class="btn btn-sm sm:btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100"
            :title="isEmpty ? 'Add metadata' : 'Edit metadata'"
            @click="startEdit"
          >
            {{ isEmpty ? '＋ Add' : '✎ Edit' }}
          </button>

          <template v-if="editing">
            <button class="btn btn-sm sm:btn-xs btn-ghost" :disabled="saving" @click="cancelEdit">
              Cancel
            </button>
            <button class="btn btn-sm sm:btn-xs btn-primary" :disabled="saving" @click="save">
              <span v-if="saving" class="loading loading-spinner loading-xs"></span>
              <span v-else>Save</span>
            </button>
          </template>
        </div>
      </div>
    </template>

    <!-- v-if, not v-show: a collapsed card should not be paying to render a
         document nobody is looking at. -->
    <div v-if="expanded" id="metadata-body">
      <MetadataEditor
        v-if="editing"
        ref="editor"
        v-model="draft"
        :schema="schema"
        :show-count="false"
      />
      <MetadataEditor
        v-else
        :model-value="modelValue"
        :schema="schema"
        :show-count="false"
        disabled
      />

      <!-- Mobile-only second copy of the edit actions. The editor's field list is
           unbounded below `sm` (see MetadataEditor), so the header pair can be
           scrolled off the top by a long document — and this is where the thumb
           already is when the last field has been filled in. A sticky header
           would be the other answer, but DaisyUI's `.card` sets
           `overflow: hidden`, which makes it the (non-scrolling) containing
           block and kills `position: sticky` outright. -->
      <div v-if="editing" class="flex justify-end gap-2 mt-4 pt-3 border-t border-base-300 sm:hidden">
        <button class="btn btn-sm btn-ghost" :disabled="saving" @click="cancelEdit">
          Cancel
        </button>
        <button class="btn btn-sm btn-primary" :disabled="saving" @click="save">
          <span v-if="saving" class="loading loading-spinner loading-xs"></span>
          <span v-else>Save</span>
        </button>
      </div>
    </div>
  </BaseCard>
</template>
