<script setup lang="ts">
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import { useUIStore } from '@/stores/ui'

const uiStore = useUIStore()
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
        <router-view />
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
