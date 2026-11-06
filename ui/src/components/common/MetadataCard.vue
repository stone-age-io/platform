<script setup lang="ts">
import { CopyIcon, Pencil, Plus } from '@lucide/vue'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataEditor from './MetadataEditor.vue'

// The metadata block on a detail page, with quick edit. The read view is the
// same MetadataEditor with `disabled`, so the two cannot drift apart.
//
// The save sends only `metadata`: the member branch of things.updateRule needs
// `nats_user` and `nebula_host` unchanged, and an absent field counts as
// unchanged. Sending the whole record would turn a member's edit into a 404.

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

// Collapsing is for the one-column mobile layout, where metadata sits between
// Basic Information and everything below. So it starts collapsed only below
// `lg` and only for larger documents. Read once at setup, not a computed: after
// that the state is the reader's, and a resize should not snap it. Not persisted.
// Kept here, not in BaseCard, until a second card wants it.
const AUTO_EXPAND_MAX = 4
const WIDE = '(min-width: 1024px)' // Tailwind `lg`, the detail grid's breakpoint
const expanded = ref(window.matchMedia(WIDE).matches || fieldCount.value <= AUTO_EXPAND_MAX)

function startEdit() {
  expanded.value = true
  // Deep clone so Cancel discards: the editor mutates `json` rows in place.
  draft.value = props.modelValue ? JSON.parse(JSON.stringify(props.modelValue)) : null
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  draft.value = null
}

async function save() {
  // Commit a JSON tab or row left mid-edit; refuses on a parse error.
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
        <!-- The title is the disclosure control, with the field count so a
             collapsed card is never a blind click. flex-1 and 44px min height
             for a touch target; the hover padding is cancelled by negative
             margins so the title aligns with other cards. -->
        <button
          type="button"
          class="flex flex-1 items-center gap-2 min-w-0 text-left -mx-2 px-2 rounded-lg
                 min-h-[44px] sm:min-h-0 sm:py-1 sm:-my-1
                 hover:bg-base-200 focus-visible:outline-none focus-visible:ring-2
                 focus-visible:ring-primary/50 transition-colors"
          :aria-expanded="expanded"
          aria-controls="metadata-body"
          @click="expanded = !expanded"
        >
          <!-- The rotation is on this span, not the <svg>: `rotate-90` on the svg
               silently does nothing. Do not drop the wrapper. -->
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
          <h2 class="card-title">Metadata</h2>
          <span class="text-xs text-base-content/50 shrink-0">
            {{ isEmpty ? 'empty' : `${fieldCount} field${fieldCount === 1 ? '' : 's'}` }}
          </span>
        </button>

        <!-- btn-sm on mobile: a 24px button is too small for a thumb. -->
        <div class="flex items-center gap-1 shrink-0">
          <button
            v-if="!editing && !isEmpty"
            class="btn btn-sm sm:btn-xs btn-ghost gap-1 px-2 sm:px-3 opacity-70 hover:opacity-100"
            title="Copy raw JSON"
            @click="copyMetadata"
          >
            <!-- Icon only on mobile, to leave the header row room. -->
            <CopyIcon :size="16" class="inline-block shrink-0" /><span class="hidden sm:inline">Copy</span>
          </button>

          <button
            v-if="!editing && canEdit"
            class="btn btn-sm sm:btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100"
            :title="isEmpty ? 'Add metadata' : 'Edit metadata'"
            @click="startEdit"
          >
            <component :is="isEmpty ? Plus : Pencil" :size="14" /> {{ isEmpty ? 'Add' : 'Edit' }}
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

    <!-- v-if, not v-show: do not render a collapsed document. -->
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

      <!-- Mobile-only second copy of the edit actions: below `sm` the field list
           is unbounded, so the header pair scrolls away. Not a sticky header:
           DaisyUI's `.card` sets `overflow: hidden`, which kills `position: sticky`. -->
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
