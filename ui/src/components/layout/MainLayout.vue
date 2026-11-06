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

// Every page here reads data scoped to the active organization, so an org
// switch remounts the page instead of each page handling it: it starts on page
// one with an empty search and loads in its own onMounted.
//
// The switch goes through the router, so the guard in router/index.ts judges the
// page against the NEW role: admin in one organization and viewer in the next,
// on NATS Users, lands on `/` as a fresh visit would. A page that passes is the
// same location, which the router reuses rather than remounts, so the key does
// that part.
//
// A record page (any route with a param: /things/:id, /nats/streams/:name/edit)
// goes to the list its path sits under instead, since its record belongs to the
// organization being left. The router's catch-all sends a prefix with no route
// to `/`.
//
// Null to an organization (the first one chosen at sign-in) leaves nothing
// behind, so even a record page stays put. An organization to null is logout:
// nothing happens, because a remount would fire the new page's requests
// unauthenticated just before the redirect to /login.
const pageKey = ref(0)
watch(() => authStore.currentOrgId, async (orgId, previous) => {
  if (!orgId) return
  const before = route.fullPath
  const path = route.matched[route.matched.length - 1]?.path ?? ''
  const firstParam = path.indexOf('/:')
  const target = previous && firstParam > 0
    ? { path: path.slice(0, firstParam) }
    : { path: route.path, query: route.query, hash: route.hash }
  await router.replace({ ...target, force: true })
  if (route.fullPath === before) pageKey.value++
})
</script>

<template>
  <div class="drawer lg:drawer-open" :class="{ 'kiosk-active': uiStore.kioskMode }">
    <!-- Drawer toggle for mobile -->
    <input id="sidebar-drawer" type="checkbox" class="drawer-toggle" />

    <div class="drawer-content flex flex-col">
      <AppHeader />

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
