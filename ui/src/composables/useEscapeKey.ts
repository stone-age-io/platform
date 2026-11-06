import { onScopeDispose, unref, watch, type Ref } from 'vue'

/**
 * Close-on-Escape for the app's modals.
 *
 * Modals here use the daisyUI `modal-open` class, not `showModal()`, so the
 * browser gives them no Escape handling.
 *
 * This listens on `window` in the bubble phase and skips an event that is
 * already `defaultPrevented`, so a handler closer to the event (ConfirmDialog,
 * RecordPicker) wins: closing a confirm does not also close the modal under it.
 * Nested modals form a stack and only the top one closes per press.
 *
 * Do not use this on a dialog that shows a one-time secret (generated password,
 * provisioned credentials): a stray keypress would lose it for good.
 */

type BooleanSource = Ref<boolean> | (() => boolean)

// Innermost open modal last. Module-level so it orders registrations across
// unrelated components.
const stack: Array<() => void> = []
let bound = false

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (event.defaultPrevented) return

  const top = stack[stack.length - 1]
  if (!top) return

  event.preventDefault()
  top()
}

function push(handler: () => void) {
  stack.push(handler)
  if (!bound) {
    window.addEventListener('keydown', onKeydown)
    bound = true
  }
}

function remove(handler: () => void) {
  const i = stack.lastIndexOf(handler)
  if (i !== -1) stack.splice(i, 1)
  if (stack.length === 0 && bound) {
    window.removeEventListener('keydown', onKeydown)
    bound = false
  }
}

/**
 * Call `onEscape` when Escape is pressed while `isOpen` is true. Pass
 * `() => true` from a component that is only mounted while open. Registration
 * follows the open state, not the component lifetime.
 */
export function useEscapeKey(isOpen: BooleanSource, onEscape: () => void) {
  const handler = () => onEscape()

  const stop = watch(
    () => (typeof isOpen === 'function' ? isOpen() : unref(isOpen)),
    (open) => {
      if (open) push(handler)
      else remove(handler)
    },
    // Sync flush: a deferred watcher would order the stack by component setup
    // order, not by the order the modals opened.
    { immediate: true, flush: 'sync' },
  )

  onScopeDispose(() => {
    stop()
    remove(handler)
  })
}
