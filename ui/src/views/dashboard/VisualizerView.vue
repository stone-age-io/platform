<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed, nextTick, provide } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useUIStore } from '@/stores/ui'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import { useToast } from '@/composables/useToast'

import DashboardSidebar from '@/components/dashboard/DashboardSidebar.vue'
import DashboardGrid from '@/components/dashboard/DashboardGrid.vue'
import WidgetContainer from '@/components/dashboard/WidgetContainer.vue'
import AddWidgetModal from '@/components/dashboard/AddWidgetModal.vue'
import ConfigureWidgetModal from '@/components/dashboard/ConfigureWidgetModal.vue'
import KeyboardShortcutsModal from '@/components/common/KeyboardShortcutsModal.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DebugPanel from '@/components/common/DebugPanel.vue'
import VariableBar from '@/components/dashboard/VariableBar.vue'
import VariableEditorModal from '@/components/dashboard/VariableEditorModal.vue'
import { Braces, Bug, EllipsisVertical, LayoutDashboard, Lock, LockOpen, RefreshCw, X } from '@lucide/vue'
import type { WidgetType } from '@/types/dashboard'

// The `dashboard` role is offered every widget type: each uses the caller's own
// NATS credential, which is the real limit, and none edits PocketBase inventory.

const authStore = useAuthStore()
const router = useRouter()
const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const uiStore = useUIStore()
const toast = useToast()

// Restricted mode: no kiosk/debug/grid selector. Keyboard shortcuts stay on
// (K does nothing here, since kiosk is off). Keyed on the role,
// not the path, so it need not track the router.
const isRestrictedMode = computed(() => authStore.isDashboardUser)

const {
  subscribeAllWidgets,
  unsubscribeAllWidgets,
  createWidget,
  deleteWidget,
  duplicateWidget,
} = useWidgetOperations()

const isSidebarOpen = ref(false)
const showAddWidget = ref(false)
const showConfigWidget = ref(false)
const showShortcutsModal = ref(false)
const showDebugPanel = ref(false)
const showVariableModal = ref(false)
const showVariableBar = ref(false)
const configWidgetId = ref<string | null>(null)
const fullScreenWidgetId = ref<string | null>(null)

const hasVariables = computed(() => (dashboardStore.activeDashboard?.variables?.length || 0) > 0)
const isShared = computed(() => dashboardStore.activeDashboard?.storage === 'kv')

// The ⋮ menu is daisyUI's focus dropdown, which stays open until focus leaves it.
function fromMenu(action: () => void) {
  ;(document.activeElement as HTMLElement | null)?.blur()
  action()
}

// --- Kiosk Mode (disabled for the dashboard role) ---
const isKioskMode = computed(() => !isRestrictedMode.value && uiStore.kioskMode)
const showKioskHint = ref(false)
let kioskHintTimer: ReturnType<typeof setTimeout> | null = null

watch(isKioskMode, (active) => {
  if (active) {
    isSidebarOpen.value = false
    showConfigWidget.value = false
    showAddWidget.value = false
    showShortcutsModal.value = false
    showDebugPanel.value = false
    showVariableModal.value = false

    showKioskHint.value = true
    if (kioskHintTimer) clearTimeout(kioskHintTimer)
    kioskHintTimer = setTimeout(() => { showKioskHint.value = false }, 3000)
  } else {
    showKioskHint.value = false
    if (kioskHintTimer) { clearTimeout(kioskHintTimer); kioskHintTimer = null }
  }
})

const fullScreenWidget = computed(() => {
  if (!fullScreenWidgetId.value) return null
  return dashboardStore.getWidget(fullScreenWidgetId.value)
})

// Provided to descendants as 'requestConfirm'.
const confirmState = ref({
  show: false,
  title: 'Confirm Action',
  message: 'Are you sure?',
  confirmText: 'Confirm',
  onConfirm: null as (() => void) | null
})

function requestConfirm(title: string, message: string, onConfirm: () => void, confirmText = 'Confirm') {
  confirmState.value = { show: true, title, message, confirmText, onConfirm }
}

function handleGlobalConfirm() {
  if (confirmState.value.onConfirm) confirmState.value.onConfirm()
  confirmState.value.show = false
}

provide('requestConfirm', requestConfirm)

function toggleSidebar() {
  isSidebarOpen.value = !isSidebarOpen.value
}

function handleCreateWidget(type: WidgetType) {
  createWidget(type)
}

function handleDeleteWidget(widgetId: string) {
  deleteWidget(widgetId)
  if (fullScreenWidgetId.value === widgetId) exitFullScreen()
}

function handleDuplicateWidget(widgetId: string) {
  duplicateWidget(widgetId)
  toast.success('Widget duplicated')
}

function handleConfigureWidget(widgetId: string) {
  configWidgetId.value = widgetId
  showConfigWidget.value = true
}

function handleWidgetConfigSaved() {
  showConfigWidget.value = false
  toast.success('Widget updated')
}

function toggleFullScreen(widgetId: string) {
  fullScreenWidgetId.value = fullScreenWidgetId.value === widgetId ? null : widgetId
}

function exitFullScreen() {
  fullScreenWidgetId.value = null
}

// Await the result: a shared save can lose a revision conflict, and claiming
// success then is how an edit disappears without anyone noticing.
async function handleSave() {
  const saved = dashboardStore.activeDashboard?.storage === 'kv'
    ? await dashboardStore.saveRemoteDashboard()
    : dashboardStore.saveToStorage()
  if (saved) toast.success('Dashboard saved')
  else toast.error(dashboardStore.storageError ?? 'Dashboard not saved')
}

function handleReloadRemote() {
  const key = dashboardStore.activeDashboard?.kvKey
  if (!key) return
  if (!dashboardStore.isDirty) return void dashboardStore.loadRemoteDashboard(key)
  requestConfirm(
    'Load the newer version?',
    'Someone saved a newer version of this dashboard. Loading it discards your unsaved changes.',
    () => dashboardStore.loadRemoteDashboard(key),
    'Discard and load',
  )
}

// Leaving the Visualizer drops a shared dashboard's unsaved edits; the
// beforeunload handler below covers closing the tab, this covers the app's own
// links. A switched organization remounts the page instead and is not caught.
let leaveConfirmed = false
onBeforeRouteLeave((to) => {
  if (leaveConfirmed || !dashboardStore.isDirty) return true
  requestConfirm(
    'Discard unsaved changes?',
    `"${dashboardStore.activeDashboard?.name || 'This dashboard'}" has changes that are not saved. Leaving discards them.`,
    () => { leaveConfirmed = true; router.push(to.fullPath) },
    'Discard',
  )
  return false
})

function handleRefreshDashboard() {
  unsubscribeAllWidgets(false) // false = wipe data
  nextTick(() => {
    subscribeAllWidgets()
    // Self-managed widgets (kv, kvtable, switch, slider, pocketbase, status-kv)
    // bypass subscribeAllWidgets and listen for this instead.
    window.dispatchEvent(new Event('dashboard:refresh'))
    toast.info('Dashboard data refreshed')
  })
}

function handleGridChange(event: Event) {
  const select = event.target as HTMLSelectElement
  const val = parseInt(select.value)
  if (dashboardStore.activeDashboard) {
    dashboardStore.updateDashboard(dashboardStore.activeDashboard.id, { columnCount: val })
  }
}

const { shortcuts } = useKeyboardShortcuts([
  { key: 's', description: 'Save Dashboard', handler: handleSave },
  { key: 'n', description: 'Add New Widget', handler: () => { if (!dashboardStore.isLocked) showAddWidget.value = true } },
  { key: 'r', description: 'Refresh Dashboard Data', handler: handleRefreshDashboard },
  { key: 'b', description: 'Toggle Dashboard Sidebar', handler: toggleSidebar },
  { key: 'a', description: 'Toggle App Sidebar', handler: () => uiStore.toggleCompact() },
  { key: 'v', description: 'Toggle Variables', handler: () => { if (hasVariables.value || !dashboardStore.isLocked) showVariableBar.value = !showVariableBar.value } },
  { key: 'l', description: 'Lock Dashboard', handler: () => { if (!dashboardStore.isLocked) dashboardStore.toggleLock() } },
  { key: 'u', description: 'Unlock Dashboard', handler: () => { if (dashboardStore.isLocked) dashboardStore.toggleLock() } },
  { key: 'k', description: 'Toggle Kiosk Mode', handler: () => uiStore.toggleKiosk() },
  { key: 'Escape', description: 'Close Modals / Exit Full Screen / Exit Kiosk', handler: () => {
      if (uiStore.kioskMode) { uiStore.kioskMode = false }
      else if (fullScreenWidgetId.value) exitFullScreen()
      else {
        showConfigWidget.value = false
        showAddWidget.value = false
        showShortcutsModal.value = false
        showDebugPanel.value = false
        showVariableModal.value = false
        isSidebarOpen.value = false
      }
    }
  },
  { key: '?', description: 'Show Shortcuts', handler: () => { showShortcutsModal.value = true } }
])

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (dashboardStore.isDirty) {
    e.preventDefault()
    e.returnValue = '' // Required for Chrome
  }
}

onMounted(() => {
  dashboardStore.loadFromStorage()
  if (natsStore.isConnected) {
    subscribeAllWidgets()
  }
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onUnmounted(() => {
  // true = keep data warm for when the user comes back
  unsubscribeAllWidgets(true)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  if (kioskHintTimer) clearTimeout(kioskHintTimer)
  // Otherwise MainLayout stays in kiosk layout on the next page.
  if (uiStore.kioskMode) uiStore.kioskMode = false
})

watch(() => natsStore.isConnected, (connected) => {
  if (connected) subscribeAllWidgets()
})

// activeLoadId, not the id: reopening a shared dashboard (Update) keeps its id,
// and a changed widget subject must still be resubscribed.
watch(() => dashboardStore.activeLoadId, async () => {
  unsubscribeAllWidgets(true) 
  if (window.innerWidth < 1024) isSidebarOpen.value = false
  await nextTick()
  if (natsStore.isConnected) subscribeAllWidgets()
})

watch(() => dashboardStore.currentVariableValues, () => {
  if (natsStore.isConnected) {
    unsubscribeAllWidgets(true)
    subscribeAllWidgets()
  }
}, { deep: true })

// No watcher on the widget list: createWidget() subscribes the new widget
// itself, and one here would subscribe it twice.
</script>

<template>
  <div class="visualizer-view" :class="{ 'kiosk-active': isKioskMode }">
    <Transition name="slide-sidebar">
      <div v-if="isSidebarOpen" class="sidebar-wrapper">
        <DashboardSidebar ref="sidebarRef" @close="isSidebarOpen = false" />
      </div>
    </Transition>

    <div v-if="isSidebarOpen" class="sidebar-backdrop" @click="isSidebarOpen = false"></div>
    
    <div class="visualizer-main">
      <div v-show="!isKioskMode" class="visualizer-toolbar">
        <div class="toolbar-left">
          <button
            class="sidebar-toggle-btn"
            :class="{ 'is-active': isSidebarOpen }"
            @click="isSidebarOpen = !isSidebarOpen"
            title="Switch Dashboard (B)"
            aria-label="Toggle dashboard sidebar"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <line x1="9" y1="3" x2="9" y2="21" />
            </svg>
          </button>
          <div class="dashboard-info">
            <h1 class="dashboard-name">{{ dashboardStore.activeDashboard?.name || 'No Dashboard Selected' }}</h1>
            <!-- Status only. On a phone it moves into the ⋮ menu, so the name keeps the room. -->
            <span v-if="isShared && !dashboardStore.remoteChanged && !dashboardStore.isDirty" class="badge badge-info gap-1 hidden sm:inline-flex">Shared</span>
          </div>
        </div>
        
        <!-- Below sm only Save/Update, + and the lock stay inline; the rest is in the ⋮ menu. -->
        <div class="toolbar-right">
          <button v-if="dashboardStore.remoteChanged" class="btn btn-sm btn-warning" title="A newer version of this shared dashboard was saved" @click="handleReloadRemote">Update</button>
          <button v-else-if="dashboardStore.isDirty" class="btn btn-sm btn-outline" title="Save Dashboard (S)" @click="handleSave">Save</button>

          <button 
            class="btn btn-sm btn-square btn-ghost hidden sm:flex" 
            @click="handleRefreshDashboard"
            title="Refresh Dashboard Data (R)"
            aria-label="Refresh dashboard data"
            :disabled="!natsStore.isConnected"
          >
            <RefreshCw :size="16" aria-hidden="true" />
          </button>

          <div v-if="!isRestrictedMode && !dashboardStore.isLocked" class="hidden md:block">
            <select :value="dashboardStore.activeDashboard?.columnCount ?? 12" @change="handleGridChange" class="select select-sm select-bordered font-mono">
              <option :value="0">Auto</option>
              <option :value="4">4 Cols</option>
              <option :value="6">6 Cols</option>
              <option :value="8">8 Cols</option>
              <option :value="10">10 Cols</option>
              <option :value="12">12 Cols</option>
              <option :value="16">16 Cols</option>
              <option :value="20">20 Cols</option>
            </select>
          </div>
          <button v-if="hasVariables || !dashboardStore.isLocked" class="btn btn-sm btn-square hidden sm:flex" :class="showVariableBar ? 'btn-active' : 'btn-ghost'" :aria-label="showVariableBar ? 'Hide dashboard variables' : 'Show dashboard variables'" :aria-pressed="showVariableBar" :title="showVariableBar ? 'Hide variables (V)' : 'Variables (V)'" @click="showVariableBar = !showVariableBar"><Braces :size="16" aria-hidden="true" /></button>
          <button class="btn btn-sm btn-square btn-ghost" :aria-label="dashboardStore.isLocked ? 'Unlock dashboard layout' : 'Lock dashboard layout'" :title="dashboardStore.isLocked ? 'Unlock layout (U)' : 'Lock layout (L)'" @click="dashboardStore.toggleLock()"><component :is="dashboardStore.isLocked ? Lock : LockOpen" :size="16" aria-hidden="true" /></button>
          <button v-if="!dashboardStore.isLocked" class="btn btn-sm btn-primary" aria-label="Add widget" @click="showAddWidget = true">+ <span class="hidden sm:inline ml-1">Widget</span></button>
          <button v-if="!isRestrictedMode" class="btn btn-sm btn-square btn-ghost hidden sm:flex" aria-label="Open debug panel" title="Debug panel" @click="showDebugPanel = true"><Bug :size="16" aria-hidden="true" /></button>

          <!-- Phone overflow. daisyUI's focus dropdown: it closes on blur, so each item blurs. -->
          <div class="dropdown dropdown-end sm:hidden">
            <div tabindex="0" role="button" class="btn btn-sm btn-square btn-ghost" aria-label="More dashboard actions"><EllipsisVertical :size="16" aria-hidden="true" /></div>
            <ul tabindex="0" class="dropdown-content menu menu-sm z-[50] mt-2 w-52 rounded-box border border-base-300 bg-base-100 p-1 shadow-lg">
              <li><button :disabled="!natsStore.isConnected" @click="fromMenu(handleRefreshDashboard)"><RefreshCw :size="16" aria-hidden="true" />Refresh data</button></li>
              <li v-if="hasVariables || !dashboardStore.isLocked"><button @click="fromMenu(() => { showVariableBar = !showVariableBar })"><Braces :size="16" aria-hidden="true" />{{ showVariableBar ? 'Hide variables' : 'Variables' }}</button></li>
              <li v-if="!isRestrictedMode"><button @click="fromMenu(() => { showDebugPanel = true })"><Bug :size="16" aria-hidden="true" />Debug panel</button></li>
              <li v-if="isShared" class="menu-title !px-3 !py-1.5 font-normal">Shared dashboard</li>
            </ul>
          </div>
        </div>
      </div>
      
      <div v-if="showVariableBar && !isKioskMode" class="border-b border-base-300 bg-base-100">
        <VariableBar @edit="showVariableModal = true" @close="showVariableBar = false" />
      </div>
      
      <!-- One notice for the whole screen, kiosk included: a wall display that
           silently stopped updating is the failure this exists for. -->
      <div v-if="!natsStore.isConnected" class="px-4 py-1.5 text-sm border-b border-warning/40 bg-warning/15" role="status">
        {{ natsStore.status === 'disconnected' ? 'Disconnected from NATS' : 'Connecting to NATS…' }}: values are not updating
      </div>

      <!-- The store records every load, save and limit failure here; nothing
           else shows it, so without this a failed save is silent. -->
      <div v-if="dashboardStore.storageError" class="flex items-center gap-2 px-4 py-1.5 text-sm border-b border-error/40 bg-error/15" role="alert">
        <span class="flex-1">{{ dashboardStore.storageError }}</span>
        <button class="btn btn-xs btn-ghost btn-square" aria-label="Dismiss storage error" @click="dashboardStore.clearStorageError()"><X :size="14" aria-hidden="true" /></button>
      </div>

      <div class="visualizer-content">
        <DashboardGrid v-if="dashboardStore.activeWidgets.length > 0" :widgets="dashboardStore.activeWidgets" :column-count="dashboardStore.activeDashboard?.columnCount" @delete-widget="handleDeleteWidget" @configure-widget="handleConfigureWidget" @duplicate-widget="handleDuplicateWidget" @fullscreen-widget="toggleFullScreen" />
        <div v-else class="h-full flex flex-col items-center justify-center text-base-content/50 p-8 text-center">
          <LayoutDashboard class="mb-4" :size="56" :stroke-width="1.25" aria-hidden="true" />
          <h3 class="text-xl font-bold">Empty Dashboard</h3>
          <p class="mt-2 text-sm max-w-xs mx-auto">
            <template v-if="!dashboardStore.isLocked">Tap <strong>+</strong> to add your first widget.</template>
            <template v-else>Unlock the dashboard to add widgets.</template>
          </p>
        </div>
      </div>
    </div>
    
    <AddWidgetModal v-model="showAddWidget" @select="handleCreateWidget" />
    <ConfigureWidgetModal v-model="showConfigWidget" :widget-id="configWidgetId" @saved="handleWidgetConfigSaved" />
    <KeyboardShortcutsModal v-model="showShortcutsModal" :shortcuts="shortcuts" />
    <VariableEditorModal v-model="showVariableModal" />
    <DebugPanel v-model="showDebugPanel" />
    
    <div v-if="fullScreenWidgetId && fullScreenWidget" class="fixed inset-0 z-[100] bg-base-100 flex flex-col">
      <div class="p-4 border-b border-base-300 flex justify-between items-center bg-base-200">
        <h2 class="font-bold text-lg">{{ fullScreenWidget.title }}</h2>
        <button class="btn btn-sm btn-circle btn-ghost" aria-label="Exit full screen" @click="exitFullScreen"><X :size="16" aria-hidden="true" /></button>
      </div>
      <div class="flex-1 p-6 overflow-hidden">
        <WidgetContainer :config="fullScreenWidget" :is-mobile="false" :is-fullscreen="true" @delete="handleDeleteWidget(fullScreenWidget.id)" @configure="handleConfigureWidget(fullScreenWidget.id)" @duplicate="handleDuplicateWidget(fullScreenWidget.id)" @fullscreen="exitFullScreen" />
      </div>
    </div>
    
    <ConfirmDialog v-model="confirmState.show" :title="confirmState.title" :message="confirmState.message" :confirm-text="confirmState.confirmText" variant="warning" @confirm="handleGlobalConfirm" />

    <!-- Kiosk mode hint -->
    <Transition name="kiosk-hint">
      <div v-if="showKioskHint" class="kiosk-hint">
        Kiosk mode — press <kbd>K</kbd> or <kbd>Esc</kbd> to exit
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.visualizer-view {
  display: flex;
  flex-direction: column;
  /* Mobile: subtract AppHeader (3rem). dvh, where supported, is the height
     left after a phone's own URL bar; plain vh hides the last tile under it. */
  height: calc(100vh - 3rem);
  height: calc(100dvh - 3rem);
  margin: -1rem; 
  width: calc(100% + 2rem);
  overflow: hidden;
  background: oklch(var(--b2));
  border-radius: 0;
  border: none;
  position: relative;
}

@media (min-width: 1024px) {
  .visualizer-view {
    margin: -1.5rem;
    width: calc(100% + 3rem);
    /* Desktop: Use full viewport height since AppHeader is hidden */
    height: 100vh;
    height: 100dvh;
  }
}

.visualizer-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  z-index: 1;
}

.sidebar-wrapper {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  width: 280px;
  z-index: 20;
  box-shadow: 4px 0 15px rgba(0,0,0,0.1);
  height: 100%;
}

@media (max-width: 1024px) {
  .sidebar-wrapper {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100%;
    max-width: 320px;
    z-index: 100;
    box-shadow: 0 0 20px rgba(0,0,0,0.3);
  }
  .sidebar-backdrop {
    position: fixed !important;
    z-index: 99 !important;
  }
}

.sidebar-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0,0,0,0.3);
  backdrop-filter: blur(2px);
  z-index: 15;
}

.slide-sidebar-enter-active,
.slide-sidebar-leave-active {
  transition: transform 0.3s ease;
}

.slide-sidebar-enter-from,
.slide-sidebar-leave-to {
  transform: translateX(-100%);
}

.visualizer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background: oklch(var(--b1));
  border-bottom: 1px solid oklch(var(--b3));
  flex-shrink: 0;
  gap: 0.5rem;
  position: sticky;
  top: 0;
  z-index: 40;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
  min-width: 0; 
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0; 
}

.sidebar-toggle-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border-radius: 0.5rem;
  cursor: pointer;
  background: oklch(var(--b2));
  color: oklch(var(--bc) / 0.7);
  border: 1px solid oklch(var(--b3));
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}

.sidebar-toggle-btn:hover {
  background: oklch(var(--b3));
  color: oklch(var(--bc));
}

.sidebar-toggle-btn.is-active {
  background: oklch(var(--p) / 0.12);
  border-color: oklch(var(--p) / 0.3);
  color: oklch(var(--p));
}

.dashboard-info {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-left: 0.5rem;
  min-width: 0;
  flex: 1;
}

.dashboard-name {
  font-weight: 700;
  font-size: 1.125rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.visualizer-content {
  flex: 1;
  overflow: hidden;
  position: relative;
}

/* Touch screens: an invisible ::after grows each 32px button to a 44px hit
   target without changing the layout. Keyed on the pointer, not the width. */
@media (pointer: coarse) {
  .toolbar-left .sidebar-toggle-btn,
  .toolbar-right .btn {
    position: relative;
  }
  .toolbar-left .sidebar-toggle-btn::after,
  .toolbar-right .btn::after {
    content: '';
    position: absolute;
    inset: -6px -4px;
  }
}

@media (max-width: 640px) {
  .visualizer-toolbar { padding: 0.25rem 0.5rem; }
  /* No width cap: .dashboard-info already gives the name what the buttons leave. */
  .dashboard-name { font-size: 0.9rem; }
  .dashboard-info { gap: 0.25rem; margin-left: 0.25rem; }
}

/* Kiosk: fill the viewport. MainLayout drops the <main> padding in kiosk mode,
   so the negative-margin breakout is reset. */
.visualizer-view.kiosk-active {
  height: 100vh;
  margin: 0;
  width: 100%;
}

.kiosk-hint {
  position: fixed;
  top: 1.5rem;
  left: 50%;
  transform: translateX(-50%);
  background: oklch(var(--n));
  color: oklch(var(--nc));
  padding: 0.625rem 1.25rem;
  border-radius: 0.5rem;
  font-size: 0.875rem;
  font-weight: 500;
  z-index: 200;
  pointer-events: none;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
}

.kiosk-hint kbd {
  background: rgba(255, 255, 255, 0.15);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-family: var(--font-mono, monospace);
  font-size: 0.8125rem;
  margin: 0 0.125rem;
}

.kiosk-hint-enter-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.kiosk-hint-leave-active {
  transition: opacity 0.8s ease, transform 0.8s ease;
}

.kiosk-hint-enter-from {
  opacity: 0;
  transform: translateX(-50%) translateY(-0.5rem);
}

.kiosk-hint-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-0.5rem);
}
</style>
