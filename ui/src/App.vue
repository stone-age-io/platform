<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'

const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const toast = useToast()

onMounted(() => {
  // Initialize theme from localStorage
  uiStore.initializeTheme()
  
  // Restore session - the router guard will handle redirection 
  // based on the result of this initialization.
  authStore.initializeFromAuth()

  // Attempt NATS Auto-Connect if session was restored
  if (authStore.isAuthenticated) {
    natsStore.tryAutoConnect()
  }
})

// Global Auth Watcher
// Handles connecting on Login and disconnecting on Logout
watch(() => authStore.isAuthenticated, (isAuth) => {
  if (isAuth) {
    natsStore.tryAutoConnect()
  } else {
    // If logged out, kill the NATS connection
    natsStore.disconnect()
  }
})
</script>

<template>
  <router-view />
  
  <!-- Toast Container -->
  <div class="toast toast-end z-[9999]">
    <div
      v-for="t in toast.toasts.value"
      :key="t.id"
      :class="['alert shadow-lg', {
          'alert-success': t.type === 'success',
          'alert-error': t.type === 'error',
          'alert-info': t.type === 'info',
          'alert-warning': t.type === 'warning',
        }]"
    >
      <span>{{ t.message }}</span>
      <button class="btn btn-sm btn-ghost" @click="toast.remove(t.id)">✕</button>
    </div>
  </div>
</template>
