<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'

const uiStore = useUIStore()
const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()

// Every page under this layout reads data scoped to the active organization, so
// a switch remounts the page rather than asking each one to notice. A fresh
// component starts on page one with an empty search and loads in its own
// onMounted, exactly as if it had been navigated to. Pages need no org-change
// handling of their own: there used to be a window event with 18 hand-copied
// handlers, and they had drifted (only one reset the page number).
//
// A record page (any route with a param: /things/:id, /nats/streams/:name/edit)
// is the exception. Its record belongs to the organization being left, so a
// remount could only fail -- a not-found toast, then the page's own redirect. It
// goes to the list its path sits under instead, and the new page mounts fresh
// on its own. The router's catch-all sends a prefix with no route to `/`.
//
// Two transitions are deliberately left alone. Null to an organization (the
// first one chosen at sign-in) leaves no organization behind, so even a record
// page just remounts. An organization to null is logout: remounting then would
// fire every request of the new page unauthenticated a moment before the
// redirect to /login.
const pageKey = ref(0)
watch(() => authStore.currentOrgId, (orgId, previous) => {
  if (!orgId) return
  const path = route.matched[route.matched.length - 1]?.path ?? ''
  const firstParam = path.indexOf('/:')
  if (previous && firstParam > 0) {
    router.replace(path.slice(0, firstParam))
  } else {
    pageKey.value++
  }
})
</script>

<template>
  <div class="drawer lg:drawer-open" :class="{ 'kiosk-active': uiStore.kioskMode }">
    <!-- Drawer toggle for mobile -->
    <input id="sidebar-drawer" type="checkbox" class="drawer-toggle" />

    <!-- Main content area -->
    <div class="drawer-content flex flex-col">
      <!-- Header -->
      <AppHeader />

      <!-- Page content -->
      <main class="flex-1 overflow-y-auto p-4 lg:p-6 bg-base-200">
        <router-view :key="pageKey" />
      </main>
    </div>

    <!-- Sidebar -->
    <div class="drawer-side z-40">
      <label for="sidebar-drawer" class="drawer-overlay"></label>
      <AppSidebar />
    </div>
  </div>
</template>

<style scoped>
/* Kiosk mode: hide all chrome so the dashboard fills the viewport */
.kiosk-active .drawer-side {
  display: none !important;
}

.kiosk-active :deep(.navbar) {
  display: none !important;
}

/* Remove padding and disable scrollbars on main content area in kiosk mode —
   the visualizer manages its own overflow internally */
.kiosk-active main {
  padding: 0 !important;
  overflow: hidden !important;
}
</style>
