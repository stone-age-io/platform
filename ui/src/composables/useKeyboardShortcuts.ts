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

    // 2. Global Help Shortcut (?)
    if (event.key === '?' && !event.ctrlKey && !event.altKey && !event.metaKey) {
      window.dispatchEvent(new CustomEvent('show-shortcuts-help'))
      return
    }
    
    // 3. Check defined shortcuts
    for (const s of shortcuts) {
      // Case insensitive match
      const keyMatch = event.key.toLowerCase() === s.key.toLowerCase()
      
      // Strict Modifier Check: 
      // We only want to trigger if NO modifiers are pressed.
      // This prevents 's' logic from firing when user hits 'Ctrl+S'
      const noModifiers = !event.ctrlKey && !event.altKey && !event.metaKey && !event.shiftKey

      // Exception: 'Escape' usually works regardless of modifiers, but let's keep it simple
      // Exception: If the key itself is uppercase (like 'S'), shift might be implied, 
      // but usually event.key handles that. We'll stick to strict no-modifiers for safety.
      
      if (keyMatch && noModifiers) {
        event.preventDefault()
        s.handler(event)
        return
      }
      
      // Special case for Escape (often doesn't care about modifiers)
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
