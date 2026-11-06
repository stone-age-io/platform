<template>
  <Teleport to="body">
    <div v-if="modelValue" class="confirm-overlay" @click.self="cancel">
      <!--
        tabindex="-1" lets focus land on the container on open, not on a button:
        nothing is pre-selected, so Enter cannot confirm, and the first Tab
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
          <!-- Decorative: the title and message already convey the variant. -->
          <component :is="icon" class="confirm-icon" :size="28" aria-hidden="true" />
          <h3 :id="titleId" class="confirm-title">{{ title }}</h3>
        </div>

        <div class="confirm-body">
          <p :id="messageId" class="confirm-message">{{ message }}</p>
          <p v-if="details" class="confirm-details">{{ details }}</p>

          <!--
            The typed gate shows the string (selectable) so the reader sees which
            record this is. Deliberately not a <form>, so Enter has no submit path.
          -->
          <div v-if="requireText" class="confirm-gate">
            <label :for="gateId" class="confirm-gate-label">
              Type <code class="confirm-gate-token">{{ requireText }}</code> to confirm
            </label>
            <input
              :id="gateId"
              ref="gateEl"
              v-model="typed"
              type="text"
              class="confirm-gate-input"
              autocomplete="off"
              autocapitalize="off"
              autocorrect="off"
              spellcheck="false"
            />
          </div>
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
            :disabled="!gateOpen"
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
import { Info, TriangleAlert, Zap } from '@lucide/vue'

interface Props {
  modelValue: boolean
  title: string
  message: string
  details?: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'info'
  /**
   * Keeps confirm disabled until the user types this string exactly. Only for
   * deletes that re-creating the record cannot undo (Thing, Location, Nebula
   * host/network, NATS user, organization); unearned friction teaches people
   * to type past it.
   *
   * Callers pass `code`, falling back to the name, because `code` is optional
   * and a gate that vanishes on some records is worse than none. The fallback
   * is spelled out at each call site, not inferred here.
   */
  requireText?: string
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
    case 'warning': return Zap
    case 'info': return Info
    default: return TriangleAlert
  }
})

const variantClass = computed(() => `variant-${props.variant}`)

const uid = useId()
const titleId = `confirm-title-${uid}`
const messageId = `confirm-message-${uid}`
const gateId = `confirm-gate-${uid}`

const dialogEl = ref<HTMLElement | null>(null)
const gateEl = ref<HTMLInputElement | null>(null)

const typed = ref('')

// Trimmed (paste artefacts) but not case-folded. Always true with no gate.
const gateOpen = computed(() => !props.requireText || typed.value.trim() === props.requireText)

// Restored on close, so a keyboard user is not dropped to the top of the page.
let previouslyFocused: HTMLElement | null = null

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

// No visibility filtering: nothing in here is hidden, and `offsetParent` is null
// inside position:fixed in some engines and for every element under jsdom, so
// filtering on it would empty the list and disable the trap.
function focusableElements(): HTMLElement[] {
  if (!dialogEl.value) return []
  return Array.from(dialogEl.value.querySelectorAll<HTMLElement>(FOCUSABLE))
}

// Escape cancels. Tab is trapped, because aria-modal claims the page is inert.
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
      // One instance in App.vue serves the whole console, so a cancelled gate
      // must not stay satisfied for the next record.
      typed.value = ''
      await nextTick()
      // Focus the gate input if there is one (Enter does nothing there),
      // otherwise the container, never a button.
      if (props.requireText && gateEl.value) {
        gateEl.value.focus()
        return
      }
      dialogEl.value?.focus()
      return
    }
    restoreFocus()
  },
)

function restoreFocus() {
  const target = previouslyFocused
  previouslyFocused = null
  // The confirmed action may have removed the opener.
  if (target && target.isConnected) target.focus()
}

onBeforeUnmount(restoreFocus)

function confirm() {
  // Rechecked here: the disabled attribute is only a rendering detail.
  if (!gateOpen.value) return
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
  flex-shrink: 0;
}

.variant-danger .confirm-icon { color: oklch(var(--er)); }
.variant-warning .confirm-icon { color: oklch(var(--wa)); }
.variant-info .confirm-icon { color: oklch(var(--in)); }

.confirm-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: oklch(var(--bc));
}

.confirm-body {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
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

/* A shut gate drops the variant colour (a dimmed red still reads as armed).
   :hover is overridden too, or the variant rules relight it. */
.btn-confirm:disabled,
.btn-confirm:disabled:hover {
  opacity: 0.4;
  cursor: not-allowed;
  background: oklch(var(--bc) / 0.25);
}

.confirm-gate {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.confirm-gate-label {
  font-size: 13px;
  color: oklch(var(--bc) / 0.7);
}

.confirm-gate-token {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  padding: 1px 5px;
  border-radius: 4px;
  background: oklch(var(--bc) / 0.12);
  color: oklch(var(--bc));
  user-select: all;
}

.confirm-gate-input {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 14px;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid oklch(var(--bc) / 0.25);
  background: oklch(var(--b1));
  color: oklch(var(--bc));
}

.confirm-gate-input:focus-visible {
  outline: 2px solid oklch(var(--bc));
  outline-offset: 1px;
}

/* Buttons get a visible ring; the container takes focus on open but gets none,
   since it is not actionable. */
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
