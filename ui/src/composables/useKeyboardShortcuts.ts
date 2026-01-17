// src/composables/useKeyboardShortcuts.ts
import { onMounted, onUnmounted } from 'vue'

type ShortcutHandler = (event: KeyboardEvent) => void

export interface ShortcutDefinition {
  key: string
  description: string
  handler: ShortcutHandler
}

export function useKeyboardShortcuts(shortcuts: ShortcutDefinition[]) {
  
  function handleKeyDown(event: KeyboardEvent) {
    // 1. Safety Check: Never trigger if user is typing in an input
    const target = event.target as HTMLElement
    if (target.tagName === 'INPUT' || 
        target.tagName === 'TEXTAREA' || 
        target.isContentEditable) {
      return
    }

    // 2. Check defined shortcuts
    for (const s of shortcuts) {
      // Case insensitive match
      const keyMatch = event.key.toLowerCase() === s.key.toLowerCase()
      
      // Strict Modifier Check: 
      // We want to trigger if NO control/alt/meta modifiers are pressed.
      // We allow Shift ONLY if the defined key is a special character that usually requires it (like '?')
      const isShiftException = /^[?<>!@#$%^&*()_+{}|:"~]$/.test(s.key)
      const noModifiers = !event.ctrlKey && !event.altKey && !event.metaKey && (!event.shiftKey || isShiftException)

      if (keyMatch && noModifiers) {
        event.preventDefault()
        s.handler(event)
        return
      }
      
      // Special case for Escape
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
