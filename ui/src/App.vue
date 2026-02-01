<!-- ui/src/App.vue -->
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const toast = useToast()
const { state: confirmState, handleConfirm, handleCancel } = useConfirm()

onMounted(() => {
  // Initialize theme from localStorage
  uiStore.initializeTheme()
  
  // Restore session
  authStore.initializeFromAuth()
})

// 1. Handle Logout
// We watch isAuthenticated specifically to catch the logout event immediately
watch(() => authStore.isAuthenticated, (isAuth) => {
  if (!isAuth) {
    natsStore.disconnect()
  }
})

// 2. Handle Login & Context Switching
// We watch currentMembership because NATS requires the membership-linked identity.
// This fires when the page loads (after context is fetched) AND when switching Orgs.
watch(() => authStore.currentMembership, (newMembership) => {
  if (newMembership) {
    natsStore.tryAutoConnect()
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

  <!-- Global Confirm Dialog -->
  <ConfirmDialog
    v-model="confirmState.show"
    :title="confirmState.options.title || 'Confirm'"
    :message="confirmState.options.message"
    :details="confirmState.options.details"
    :confirm-text="confirmState.options.confirmText"
    :cancel-text="confirmState.options.cancelText"
    :variant="confirmState.options.variant"
    @confirm="handleConfirm"
    @cancel="handleCancel"
  />
</template>
