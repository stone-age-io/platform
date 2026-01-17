import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUIStore = defineStore('ui', () => {
  // State
  const theme = ref<'light' | 'dark'>('dark')
  const sidebarOpen = ref(true) // For mobile drawer
  const sidebarCompact = ref(false) // For desktop compact mode
  
  // Actions
  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    document.documentElement.setAttribute('data-theme', theme.value)
    localStorage.setItem('theme', theme.value)
  }
  
  function toggleSidebar() {
    sidebarOpen.value = !sidebarOpen.value
  }

  function toggleCompact() {
    sidebarCompact.value = !sidebarCompact.value
    localStorage.setItem('sidebar_compact', String(sidebarCompact.value))
  }
  
  function initializeTheme() {
    const savedTheme = localStorage.getItem('theme') as 'light' | 'dark' | null
    if (savedTheme) {
      theme.value = savedTheme
    }
    document.documentElement.setAttribute('data-theme', theme.value)

    const savedCompact = localStorage.getItem('sidebar_compact')
    if (savedCompact) {
      sidebarCompact.value = savedCompact === 'true'
    }
  }
  
  return {
    theme,
    sidebarOpen,
    sidebarCompact,
    toggleTheme,
    toggleSidebar,
    toggleCompact,
    initializeTheme,
  }
})
