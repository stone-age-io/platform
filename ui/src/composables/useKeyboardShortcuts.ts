import { onMounted, onUnmounted } from 'vue'

type ShortcutHandler = (event: KeyboardEvent) => void

export interface ShortcutDefinition {
  key: string
  description: string
  handler: ShortcutHandler
}

export function useKeyboardShortcuts(shortcuts: ShortcutDefinition[]) {
  
  function handleKeyDown(event: KeyboardEvent) {
    // Never fire while the user is typing.
    const target = event.target as HTMLElement
    if (target.tagName === 'INPUT' || 
        target.tagName === 'TEXTAREA' || 
        target.isContentEditable) {
      return
    }

    for (const s of shortcuts) {
      const keyMatch = event.key.toLowerCase() === s.key.toLowerCase()

      // No Ctrl/Alt/Meta. Shift only for keys that need it to be typed ('?').
      const isShiftException = /^[?<>!@#$%^&*()_+{}|:"~]$/.test(s.key)
      const noModifiers = !event.ctrlKey && !event.altKey && !event.metaKey && (!event.shiftKey || isShiftException)

      if (keyMatch && noModifiers) {
        event.preventDefault()
        s.handler(event)
        return
      }
      
      // Escape fires whatever modifiers are held.
      if (s.key.toLowerCase() === 'escape' && event.key === 'Escape') {
        event.preventDefault()
        s.handler(event)
        return
      }
    }
  }
  
  onMounted(() => window.addEventListener('keydown', handleKeyDown))
  onUnmounted(() => window.removeEventListener('keydown', handleKeyDown))
  
  return {
    shortcuts
  }
}
