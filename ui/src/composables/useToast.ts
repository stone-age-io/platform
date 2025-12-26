import { ref } from 'vue'

/**
 * Toast notification interface
 */
interface Toast {
  id: number
  message: string
  type: 'success' | 'error' | 'info' | 'warning'
  duration: number
}

// Global state - shared across all instances of useToast
const toasts = ref<Toast[]>([])
let toastId = 0

/**
 * Toast Composable
 * 
 * Simple toast notification system.
 * Uses a global array so toasts persist across component unmounts.
 * 
 * Usage:
 *   const toast = useToast()
 *   toast.success('Saved!')
 *   toast.error('Failed to save')
 */
export function useToast() {
  /**
   * Show a toast notification
   */
  function show(
    message: string, 
    type: Toast['type'] = 'info', 
    duration = 3000
  ) {
    const id = toastId++
    toasts.value.push({ id, message, type, duration })
    
    // Auto-remove after duration
    setTimeout(() => {
      remove(id)
    }, duration)
  }
  
  /**
   * Remove a toast by ID
   */
  function remove(id: number) {
    const index = toasts.value.findIndex(t => t.id === id)
    if (index > -1) {
      toasts.value.splice(index, 1)
    }
  }
  
  /**
   * Clear all toasts
   */
  function clear() {
    toasts.value = []
  }
  
  // Convenience methods
  const success = (msg: string, duration?: number) => show(msg, 'success', duration)
  const error = (msg: string, duration?: number) => show(msg, 'error', duration)
  const info = (msg: string, duration?: number) => show(msg, 'info', duration)
  const warning = (msg: string, duration?: number) => show(msg, 'warning', duration)
  
  return {
    toasts,
    show,
    remove,
    clear,
    success,
    error,
    info,
    warning,
  }
}
