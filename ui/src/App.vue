<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useToast } from '@/composables/useToast'

const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUIStore()
const toast = useToast()

/**
 * Initialize app on mount
 */
onMounted(() => {
  // Initialize theme from localStorage
  uiStore.initializeTheme()
  
  // Initialize auth from PocketBase authStore
  authStore.initializeFromAuth()
  
  // Redirect to login if not authenticated
  if (!authStore.isAuthenticated) {
    router.push('/login')
  }
})
</script>

<template>
  <router-view />
  
  <!-- Toast Container (global) -->
  <div class="toast toast-end">
    <div
      v-for="t in toast.toasts.value"
      :key="t.id"
      :class="[
        'alert',
        {
          'alert-success': t.type === 'success',
          'alert-error': t.type === 'error',
          'alert-info': t.type === 'info',
          'alert-warning': t.type === 'warning',
        }
      ]"
    >
      <span>{{ t.message }}</span>
      <button class="btn btn-sm btn-ghost" @click="toast.remove(t.id)">✕</button>
    </div>
  </div>
</template>
