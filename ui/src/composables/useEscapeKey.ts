import { onScopeDispose, unref, watch, type Ref } from 'vue'

/**
 * Close-on-Escape for the app's modals.
 *
 * Every modal in this console is written as
 * `<dialog class="modal" :class="{ 'modal-open': show }">` -- the daisyUI CSS
 * form, not `dialogEl.showModal()`. That is a deliberate trade (the open state
 * stays a plain ref that `v-if`, Teleport and the parent's logic can all see),
 * but it costs the three things the browser gives a *modal* dialog for free:
 * the top layer, the focus trap, and Escape. Backdrop click was wired up by
 * hand in each one; Escape never was, so for twenty-odd dialogs the key did
 * nothing at all -- while ConfirmDialog and RecordPicker, which handle their
 * own, did close. Inconsistent is worse than absent: the user learns the key
 * works and then it silently doesn't.
 *
 * Two details carry the weight:
 *
 * `defaultPrevented` is the deference check. This listens on `window` in the
 * bubble phase, so it runs AFTER any handler bound closer to the event --
 * ConfirmDialog's dialog-level trap, RecordPicker's combobox keydown -- and
 * both of those call `preventDefault()` on the Escape they consume. Bailing
 * when the event is already handled means a confirm opened on top of a modal
 * cancels the confirm and leaves the modal standing, and closing a
 * RecordPicker dropdown does not also close the form it sits in. It costs
 * nothing and covers anything added later that handles Escape properly.
 *
 * The stack is for modals of our own that nest (a quick-add form inside a
 * form view, say). Only the top registration fires, so Escape unwinds one
 * layer per press rather than collapsing the lot.
 *
 * Do NOT wire this into a dialog that displays a one-time secret -- the
 * generated-password and provisioned-credentials modals say "it cannot be
 * recovered later" in their own copy, and a stray keypress that loses it is a
 * support call, not a dismissal. Those four are listed at the call sites.
 */

type BooleanSource = Ref<boolean> | (() => boolean)

// Innermost open modal last. Module-level on purpose: the whole point is to
// order registrations that live in unrelated components.
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
 * Call `onEscape` when Escape is pressed while `isOpen` is true.
 *
 * `isOpen` may be a ref or a getter; pass `() => true` from a component that is
 * itself only mounted while open. Registration follows the open state rather
 * than the component lifetime, so a view holding four modals registers four
 * times and only the open one is ever on the stack.
 */
export function useEscapeKey(isOpen: BooleanSource, onEscape: () => void) {
  const handler = () => onEscape()

  const stop = watch(
    () => (typeof isOpen === 'function' ? isOpen() : unref(isOpen)),
    (open) => {
      if (open) push(handler)
      else remove(handler)
    },
    // Sync flush, not the default pre-flush. The stack is an ORDERING
    // structure, and a deferred watcher would order it by the registering
    // components' setup order rather than by the order the modals actually
    // opened -- so two dialogs opening in one tick could unwind backwards.
    // Sync also means the handler is live the instant the ref flips, with no
    // tick in which the modal is on screen and Escape does nothing.
    { immediate: true, flush: 'sync' },
  )

  onScopeDispose(() => {
    stop()
    remove(handler)
  })
}
