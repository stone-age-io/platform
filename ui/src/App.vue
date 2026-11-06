<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useDashboardStore } from '@/stores/dashboard'
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
  // Auth hydration is not here: it runs pre-mount in main.ts.
  uiStore.initializeTheme()
})

// Logout. Deliberately not immediate: it reacts only to the transition, so a
// logged-out tab does not disconnect on open.
watch(() => authStore.isAuthenticated, async (isAuth) => {
  if (!isAuth) {
    try {
      await natsStore.disconnect()
    } catch (err) {
      console.error('Failed to disconnect NATS on logout:', err)
    }
  }
})

// Login and org switching. Watches currentOrgId, not currentMembership, so an
// identity edit on the settings page (which reconnects itself) does not race a
// second reconnect. An org change always drops the connection, since the NATS
// user belongs to the membership; tryAutoConnect reconnects only if enabled.
// Immediate, so a session restored pre-mount still auto-connects.
//
// On a real switch (prev is undefined only on the first run) reset the stores
// that outlive the page: widget buffers and local dashboards use the same keys
// in every org, so org A's values would keep drawing in org B.
watch(() => authStore.currentOrgId, async (orgId, prev) => {
  if (prev !== undefined) {
    useWidgetDataStore().clearAllBuffers()
    useDashboardStore().leaveRemoteDashboard()
  }
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

  <div class="toast toast-top toast-center md:toast-top md:toast-end z-[9999] w-full max-w-[calc(100vw-1rem)] md:w-auto md:max-w-sm">
    <TransitionGroup name="toast" tag="div" class="flex flex-col gap-2 w-full">
      <!-- Errors and warnings are role=alert (assertive); the rest role=status (polite). -->
      <div
        v-for="t in toast.toasts.value"
        :key="t.id"
        :class="['alert shadow-lg w-full md:min-w-[20rem]', {
            'alert-success': t.type === 'success',
            'alert-error': t.type === 'error',
            'alert-info': t.type === 'info',
            'alert-warning': t.type === 'warning',
          }]"
        :role="t.type === 'error' || t.type === 'warning' ? 'alert' : 'status'"
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

  <ConfirmDialog
    v-model="confirmState.show"
    :title="confirmState.options.title || 'Confirm'"
    :message="confirmState.options.message"
    :details="confirmState.options.details"
    :confirm-text="confirmState.options.confirmText"
    :cancel-text="confirmState.options.cancelText"
    :variant="confirmState.options.variant"
    :require-text="confirmState.options.requireText"
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
