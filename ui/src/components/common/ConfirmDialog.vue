<template>
  <Teleport to="body">
    <div v-if="modelValue" class="confirm-overlay" @click.self="cancel">
      <!--
        role="dialog" + aria-modal tell a screen reader this is a modal and that
        the rest of the page is inert; aria-labelledby/describedby are what it
        reads out on arrival. Without them this was an anonymous <div> standing
        between the user and every destructive action in the product.

        tabindex="-1" makes the container focusable so focus can land HERE on
        open rather than on a button. That is deliberate for a confirm: nothing
        is pre-selected, so Enter does not delete anything, and the first Tab
        reaches Cancel because it comes first in the DOM.
      -->
      <div
        ref="dialogEl"
        class="confirm-dialog"
        :class="variantClass"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        :aria-describedby="messageId"
        tabindex="-1"
        @keydown="onKeydown"
      >
        <div class="confirm-header">
          <!-- Decorative: the variant is already conveyed by the title and the
               message, so announcing an emoji would just add noise. -->
          <div class="confirm-icon" aria-hidden="true">{{ icon }}</div>
          <h3 :id="titleId" class="confirm-title">{{ title }}</h3>
        </div>

        <div class="confirm-body">
          <p :id="messageId" class="confirm-message">{{ message }}</p>
          <p v-if="details" class="confirm-details">{{ details }}</p>
        </div>

        <div class="confirm-actions">
          <button
            class="btn-secondary"
            type="button"
            @click="cancel"
          >
            {{ cancelText }}
          </button>
          <button
            class="btn-confirm"
            :class="variantClass"
            type="button"
            @click="confirm"
          >
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'

interface Props {
  modelValue: boolean
  title: string
  message: string
  details?: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'info'
}

const props = withDefaults(defineProps<Props>(), {
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  variant: 'danger'
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: []
  cancel: []
}>()

const icon = computed(() => {
  switch (props.variant) {
    case 'danger': return '⚠️'
    case 'warning': return '⚡'
    case 'info': return 'ℹ️'
    default: return '⚠️'
  }
})

const variantClass = computed(() => `variant-${props.variant}`)

// Unique per instance, so two dialogs on a page cannot both claim the same
// aria-labelledby target.
const uid = useId()
const titleId = `confirm-title-${uid}`
const messageId = `confirm-message-${uid}`

const dialogEl = ref<HTMLElement | null>(null)

// What had focus before the dialog opened, so it can be given back. Without
// this, dismissing a confirm drops focus to the top of the document and a
// keyboard user has to tab back to wherever they were — which on a list view
// means tabbing past every row.
let previouslyFocused: HTMLElement | null = null

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

// No visibility filtering, deliberately. Everything inside this dialog is
// rendered by v-if and is on screen whenever the dialog exists, so there is
// nothing hidden to skip — and the usual test for it, `offsetParent !== null`,
// is wrong in two ways that matter: it reports null for anything inside a
// position:fixed subtree in some engines, and it is null for EVERY element
// under jsdom, which has no layout. Filtering on it silently emptied the list
// and turned the trap into a no-op.
function focusableElements(): HTMLElement[] {
  if (!dialogEl.value) return []
  return Array.from(dialogEl.value.querySelectorAll<HTMLElement>(FOCUSABLE))
}

// Escape cancels, and Tab is trapped inside the dialog.
//
// The trap is the part that makes aria-modal honest: it claims the rest of the
// page is inert, and without trapping Tab focus walks straight out of the
// dialog into a page the user cannot see, still able to activate whatever it
// lands on.
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    cancel()
    return
  }
  if (event.key !== 'Tab') return

  const focusable = focusableElements()
  if (focusable.length === 0) {
    event.preventDefault()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement

  if (event.shiftKey && (active === first || active === dialogEl.value)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(
  () => props.modelValue,
  async (open) => {
    if (open) {
      previouslyFocused = document.activeElement as HTMLElement | null
      await nextTick()
      // The container, not a button: see the template comment. Nothing is
      // pre-selected, so Enter cannot confirm a destructive action by accident.
      dialogEl.value?.focus()
      return
    }
    restoreFocus()
  },
)

function restoreFocus() {
  const target = previouslyFocused
  previouslyFocused = null
  // Only if it is still in the document — the confirmed action may well have
  // removed the row whose button opened this.
  if (target && target.isConnected) target.focus()
}

onBeforeUnmount(restoreFocus)

function confirm() {
  emit('confirm')
  emit('update:modelValue', false)
}

function cancel() {
  emit('cancel')
  emit('update:modelValue', false)
}
</script>

<style scoped>
.confirm-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10100;  /* Above all other modals (z-index: 9999) so confirms always appear on top */
  backdrop-filter: blur(2px);
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.confirm-dialog {
  background: oklch(var(--b1));
  border: 2px solid;
  border-radius: 8px;
  width: 90%;
  max-width: 480px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* Variant border colors */
.confirm-dialog.variant-danger { border-color: oklch(var(--er)); }
.confirm-dialog.variant-warning { border-color: oklch(var(--wa)); }
.confirm-dialog.variant-info { border-color: oklch(var(--in)); }

.confirm-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px;
  border-bottom: 1px solid oklch(var(--b3));
}

.confirm-icon {
  font-size: 32px;
  line-height: 1;
}

.confirm-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: oklch(var(--bc));
}

.confirm-body {
  padding: 24px;
}

.confirm-message {
  margin: 0 0 12px 0;
  font-size: 15px;
  line-height: 1.5;
  color: oklch(var(--bc));
}

.confirm-details {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: oklch(var(--bc) / 0.6);
  padding: 12px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 4px;
}

.confirm-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding: 16px 24px;
  border-top: 1px solid oklch(var(--b3));
}

.btn-secondary,
.btn-confirm {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.1);
  color: oklch(var(--bc));
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
}

.btn-confirm {
  color: white;
}

.btn-confirm.variant-danger { background: oklch(var(--er)); }
.btn-confirm.variant-danger:hover { background: oklch(var(--er) / 0.8); }

.btn-confirm.variant-warning { background: oklch(var(--wa)); }
.btn-confirm.variant-warning:hover { background: oklch(var(--wa) / 0.8); }

.btn-confirm.variant-info { background: oklch(var(--in)); }
.btn-confirm.variant-info:hover { background: oklch(var(--in) / 0.8); }

/* A keyboard user must be able to see which button they are about to press,
   and this dialog is where that matters most. The container takes focus on
   open and gets no ring, because it is not actionable. */
.btn-secondary:focus-visible,
.btn-confirm:focus-visible {
  outline: 2px solid oklch(var(--bc));
  outline-offset: 2px;
}

.confirm-dialog:focus {
  outline: none;
}

@media (prefers-reduced-motion: reduce) {
  .confirm-overlay,
  .confirm-dialog {
    animation: none;
  }

  .btn-secondary,
  .btn-confirm {
    transition: none;
  }
}

/* Mobile adjustments */
@media (max-width: 600px) {
  .confirm-dialog {
    width: 95%;
    max-width: none;
  }
  
  .confirm-header {
    padding: 16px 20px;
  }
  
  .confirm-body {
    padding: 20px;
  }
  
  .confirm-actions {
    flex-direction: column-reverse;
    padding: 12px 20px;
  }
  
  .btn-secondary,
  .btn-confirm {
    width: 100%;
  }
}
</style>
