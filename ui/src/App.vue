<!-- ui/src/App.vue -->
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const TOAST_ICON_PATHS: Record<'success' | 'error' | 'warning' | 'info', string> = {
  success: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
  error:   'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
  warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
  info:    'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
}

const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const toast = useToast()
const { state: confirmState, handleConfirm, handleCancel } = useConfirm()

onMounted(() => {
  // Initialize theme from localStorage.
  // Auth hydration runs pre-mount in main.ts so the router guard sees a
  // restored session on the very first navigation.
  uiStore.initializeTheme()
})

// 1. Handle Logout
// Intentionally NOT { immediate: true }: this watcher only reacts to the
// authenticated -> unauthenticated transition (logout). Firing on initial
// false would call a redundant disconnect on every logged-out tab open.
watch(() => authStore.isAuthenticated, async (isAuth) => {
  if (!isAuth) {
    try {
      await natsStore.disconnect()
    } catch (err) {
      console.error('Failed to disconnect NATS on logout:', err)
    }
  }
})

// 2. Handle Login & Org Switching
// Driven off currentOrgId (not currentMembership) so that identity edits on the
// settings page don't trigger a reconnect through reactivity — those are handled
// explicitly by the settings view to avoid racing two reconnect paths.
//
// Org change (including logout) always drops the existing connection: the
// NATS user lives on the membership record, so the previous org's creds are
// wrong by definition once the org changes. Reconnect only if auto-connect
// is enabled — otherwise the user reconnects manually with the new identity.
//
// { immediate: true } so that a session restored from localStorage pre-mount
// (new-tab scenario) still triggers tryAutoConnect on the initial orgId.
watch(() => authStore.currentOrgId, async (orgId) => {
  if (natsStore.status !== 'disconnected') {
    await natsStore.disconnect()
  }
  if (orgId) {
    await natsStore.tryAutoConnect()
  }
}, { immediate: true })
</script>

<template>
  <router-view />

  <!-- Toast Container: top-center on mobile, bottom-right on desktop -->
  <div class="toast toast-top toast-center md:toast-top md:toast-end z-[9999] w-full max-w-[calc(100vw-1rem)] md:w-auto md:max-w-sm">
    <TransitionGroup name="toast" tag="div" class="flex flex-col gap-2 w-full">
      <div
        v-for="t in toast.toasts.value"
        :key="t.id"
        :class="['alert shadow-lg w-full md:min-w-[20rem]', {
            'alert-success': t.type === 'success',
            'alert-error': t.type === 'error',
            'alert-info': t.type === 'info',
            'alert-warning': t.type === 'warning',
          }]"
        role="alert"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="TOAST_ICON_PATHS[t.type] ?? TOAST_ICON_PATHS.info" />
        </svg>

        <span class="flex-1 text-sm break-words">{{ t.message }}</span>

        <button
          class="btn btn-circle btn-ghost btn-xs"
          aria-label="Dismiss"
          @click="toast.remove(t.id)"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
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

<style>
.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateY(-0.75rem) scale(0.96);
}
.toast-leave-to {
  opacity: 0;
  transform: scale(0.95);
}
.toast-leave-active {
  position: absolute;
  right: 0;
  left: 0;
}
.toast-move {
  transition: transform 0.25s ease;
}
</style>
