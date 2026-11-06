import { ref } from 'vue'

interface ConfirmOptions {
  title?: string
  message: string
  details?: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'info'
  /**
   * The user must type this exactly to enable confirm: a record's code, or its
   * name if it has none. Only for irreversible deletes (see ConfirmDialog).
   */
  requireText?: string
}

interface ConfirmState {
  show: boolean
  options: ConfirmOptions
  resolve: ((value: boolean) => void) | null
}

// Module-level: one dialog, rendered in App.vue, shared by every caller.
const state = ref<ConfirmState>({
  show: false,
  options: { message: '' },
  resolve: null
})

/**
 * Promise-based replacement for the native confirm(), using the styled dialog.
 */
export function useConfirm() {
  /** Resolves true if confirmed, false if cancelled. */
  function confirm(messageOrOptions: string | ConfirmOptions): Promise<boolean> {
    const options: ConfirmOptions = typeof messageOrOptions === 'string'
      ? { message: messageOrOptions }
      : messageOrOptions

    return new Promise((resolve) => {
      state.value = {
        show: true,
        options: {
          title: options.title || 'Confirm',
          message: options.message,
          details: options.details,
          confirmText: options.confirmText || 'Confirm',
          cancelText: options.cancelText || 'Cancel',
          variant: options.variant || 'danger',
          requireText: options.requireText
        },
        resolve
      }
    })
  }

  function handleConfirm() {
    if (state.value.resolve) {
      state.value.resolve(true)
    }
    state.value.show = false
    state.value.resolve = null
  }

  function handleCancel() {
    if (state.value.resolve) {
      state.value.resolve(false)
    }
    state.value.show = false
    state.value.resolve = null
  }

  return {
    confirm,
    // state and the handlers are for App.vue's ConfirmDialog
    state,
    handleConfirm,
    handleCancel
  }
}
