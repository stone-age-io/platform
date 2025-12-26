import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * UI Store
 * 
 * Manages UI-specific state like theme and sidebar visibility.
 * Keeps UI state separate from business logic.
 */
export const useUIStore = defineStore('ui', () => {
  // ============================================================================
  // STATE
  // ============================================================================
  
  const theme = ref<'light' | 'dark'>('dark')
  const sidebarOpen = ref(true)
  
  // ============================================================================
  // ACTIONS
  // ============================================================================
  
  /**
   * Toggle between light and dark theme
   */
  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    document.documentElement.setAttribute('data-theme', theme.value)
    localStorage.setItem('theme', theme.value)
  }
  
  /**
   * Toggle sidebar visibility (mainly for mobile)
   */
  function toggleSidebar() {
    sidebarOpen.value = !sidebarOpen.value
  }
  
  /**
   * Initialize theme from localStorage
   * Called on app mount
   */
  function initializeTheme() {
    const saved = localStorage.getItem('theme') as 'light' | 'dark' | null
    if (saved) {
      theme.value = saved
    }
    document.documentElement.setAttribute('data-theme', theme.value)
  }
  
  // ============================================================================
  // RETURN PUBLIC API
  // ============================================================================
  
  return {
    theme,
    sidebarOpen,
    toggleTheme,
    toggleSidebar,
    initializeTheme,
  }
})
