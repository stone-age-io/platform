import { ref } from 'vue'

/**
 * Confirm dialog options
 */
interface ConfirmOptions {
  title?: string
  message: string
  details?: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'info'
}

/**
 * Internal state for the global confirm dialog
 */
interface ConfirmState {
  show: boolean
  options: ConfirmOptions
  resolve: ((value: boolean) => void) | null
}

// Global state - shared across all instances
const state = ref<ConfirmState>({
  show: false,
  options: { message: '' },
  resolve: null
})

/**
 * Confirm Composable
 *
 * Promise-based confirmation dialog that can be used like the native confirm()
 * but displays a beautiful styled dialog instead of the browser's ugly one.
 *
 * Usage:
 *   const confirm = useConfirm()
 *
 *   // Simple usage (like native confirm)
 *   if (await confirm('Delete this item?')) {
 *     // user clicked confirm
 *   }
 *
 *   // With options
 *   const confirmed = await confirm({
 *     title: 'Delete Thing',
 *     message: 'Are you sure you want to delete "Sensor 1"?',
 *     details: 'This action cannot be undone.',
 *     confirmText: 'Delete',
 *     variant: 'danger'
 *   })
 */
export function useConfirm() {
  /**
   * Show confirmation dialog and return promise
   * Resolves to true if confirmed, false if cancelled
   */
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
          variant: options.variant || 'danger'
        },
        resolve
      }
    })
  }

  /**
   * Handle user confirmation
   * Called by GlobalConfirmDialog component
   */
  function handleConfirm() {
    if (state.value.resolve) {
      state.value.resolve(true)
    }
    state.value.show = false
    state.value.resolve = null
  }

  /**
   * Handle user cancellation
   * Called by GlobalConfirmDialog component
   */
  function handleCancel() {
    if (state.value.resolve) {
      state.value.resolve(false)
    }
    state.value.show = false
    state.value.resolve = null
  }

  return {
    // For views to call
    confirm,
    // For GlobalConfirmDialog to use
    state,
    handleConfirm,
    handleCancel
  }
}
