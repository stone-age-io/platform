<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed, nextTick, provide } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import { useToast } from '@/composables/useToast'

import DashboardSidebar from '@/components/dashboard/DashboardSidebar.vue'
import DashboardGrid from '@/components/dashboard/DashboardGrid.vue'
import WidgetContainer from '@/components/dashboard/WidgetContainer.vue'
import AddWidgetModal from '@/components/dashboard/AddWidgetModal.vue'
import ConfigureWidgetModal from '@/components/dashboard/ConfigureWidgetModal.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import VariableBar from '@/components/dashboard/VariableBar.vue'
import VariableEditorModal from '@/components/dashboard/VariableEditorModal.vue'
import type { WidgetType } from '@/types/dashboard'

const BADGE_WIDGET_TYPES: WidgetType[] = [
  'button', 'switch', 'slider', 'publisher', 'kv', 'kvtable', 'text', 'status', 'stat'
]

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const toast = useToast()

const {
  subscribeAllWidgets,
  unsubscribeAllWidgets,
  createWidget,
  deleteWidget,
  duplicateWidget,
} = useWidgetOperations()

// UI State
const isSidebarOpen = ref(false)
const showAddWidget = ref(false)
const showConfigWidget = ref(false)
const showVariableModal = ref(false)
const showVariableBar = ref(false)
const configWidgetId = ref<string | null>(null)
const fullScreenWidgetId = ref<string | null>(null)

const hasVariables = computed(() => (dashboardStore.activeDashboard?.variables?.length || 0) > 0)

const fullScreenWidget = computed(() => {
  if (!fullScreenWidgetId.value) return null
  return dashboardStore.getWidget(fullScreenWidgetId.value)
})

// --- Global Confirmation Logic ---
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

// --- Actions ---
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

function handleSave() {
  if (dashboardStore.activeDashboard?.storage === 'kv') {
    dashboardStore.saveRemoteDashboard()
  } else {
    dashboardStore.saveToStorage()
  }
  toast.success('Dashboard saved')
}

function handleReloadRemote() {
  if (dashboardStore.activeDashboard?.kvKey) {
    dashboardStore.loadRemoteDashboard(dashboardStore.activeDashboard.kvKey)
  }
}

function handleRefreshDashboard() {
  unsubscribeAllWidgets(false)
  nextTick(() => {
    subscribeAllWidgets()
    toast.info('Dashboard data refreshed')
  })
}

// --- Lifecycle ---
function handleOrgChange() {
  if (natsStore.isConnected) {
    dashboardStore.refreshRemoteKeys()
  }
}

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (dashboardStore.isDirty) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onMounted(() => {
  dashboardStore.loadFromStorage()
  if (natsStore.isConnected) {
    subscribeAllWidgets()
  }
  window.addEventListener('organization-changed', handleOrgChange)
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onUnmounted(() => {
  unsubscribeAllWidgets(true)
  window.removeEventListener('organization-changed', handleOrgChange)
  window.removeEventListener('beforeunload', handleBeforeUnload)
})

// Watchers
watch(() => natsStore.isConnected, (connected) => {
  if (connected) subscribeAllWidgets()
})

watch(() => dashboardStore.activeDashboardId, async () => {
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
</script>

<template>
  <div class="badge-dashboard-view">
    <Transition name="slide-sidebar">
      <div v-if="isSidebarOpen" class="sidebar-wrapper">
        <DashboardSidebar @close="isSidebarOpen = false" />
      </div>
    </Transition>

    <div v-if="isSidebarOpen" class="sidebar-backdrop" @click="isSidebarOpen = false"></div>

    <div class="visualizer-main">
      <!-- Toolbar -->
      <div class="visualizer-toolbar">
        <div class="toolbar-left">
          <button class="sidebar-toggle-btn" :class="{ 'is-active': isSidebarOpen }" @click="isSidebarOpen = !isSidebarOpen">&#128452;&#65039;</button>
          <div class="dashboard-info">
            <h1 class="dashboard-name">{{ dashboardStore.activeDashboard?.name || 'No Dashboard Selected' }}</h1>
            <button v-if="dashboardStore.remoteChanged" class="badge badge-warning gap-1 cursor-pointer" @click="handleReloadRemote">Update &#8635;</button>
            <button v-else-if="dashboardStore.isDirty" class="badge badge-warning gap-1 cursor-pointer" @click="handleSave">Save &#9679;</button>
            <span v-else-if="dashboardStore.activeDashboard?.storage === 'kv'" class="badge badge-info gap-1">Shared</span>
          </div>
        </div>

        <div class="toolbar-right">
          <button
            class="btn btn-sm btn-square btn-ghost"
            @click="handleRefreshDashboard"
            title="Refresh Dashboard Data"
            :disabled="!natsStore.isConnected"
          >
            &#128260;
          </button>
          <button v-if="hasVariables || !dashboardStore.isLocked" class="btn btn-sm btn-square" :class="showVariableBar ? 'btn-active' : 'btn-ghost'" @click="showVariableBar = !showVariableBar"><span class="font-mono font-bold">{ }</span></button>
          <button class="btn btn-sm btn-square btn-ghost" @click="dashboardStore.toggleLock(); toast.info(dashboardStore.isLocked ? 'Dashboard locked' : 'Dashboard unlocked')">{{ dashboardStore.isLocked ? '&#128274;' : '&#128275;' }}</button>
          <button v-if="!dashboardStore.isLocked" class="btn btn-sm btn-primary" @click="showAddWidget = true">+ <span class="hidden sm:inline ml-1">Widget</span></button>
        </div>
      </div>

      <div v-if="showVariableBar" class="border-b border-base-300 bg-base-100">
        <VariableBar @edit="showVariableModal = true" @close="showVariableBar = false" />
      </div>

      <!-- Grid Content -->
      <div class="visualizer-content">
        <DashboardGrid v-if="dashboardStore.activeWidgets.length > 0" :widgets="dashboardStore.activeWidgets" :column-count="dashboardStore.activeDashboard?.columnCount" @delete-widget="handleDeleteWidget" @configure-widget="handleConfigureWidget" @duplicate-widget="handleDuplicateWidget" @fullscreen-widget="toggleFullScreen" />
        <div v-else class="h-full flex flex-col items-center justify-center text-base-content/50 p-8 text-center">
          <span class="text-6xl mb-4 opacity-50">&#128202;</span>
          <h3 class="text-xl font-bold">Empty Dashboard</h3>
          <p class="mt-2 text-sm max-w-xs mx-auto">
            <template v-if="!dashboardStore.isLocked">Tap <strong>+</strong> to add your first widget.</template>
            <template v-else>Unlock the dashboard to add widgets.</template>
          </p>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <AddWidgetModal v-model="showAddWidget" :allowed-types="BADGE_WIDGET_TYPES" @select="handleCreateWidget" />
    <ConfigureWidgetModal v-model="showConfigWidget" :widget-id="configWidgetId" @saved="handleWidgetConfigSaved" />
    <VariableEditorModal v-model="showVariableModal" />

    <!-- Full Screen Widget -->
    <div v-if="fullScreenWidgetId && fullScreenWidget" class="fixed inset-0 z-[100] bg-base-100 flex flex-col">
      <div class="p-4 border-b border-base-300 flex justify-between items-center bg-base-200">
        <h2 class="font-bold text-lg">{{ fullScreenWidget.title }}</h2>
        <button class="btn btn-sm btn-circle btn-ghost" @click="exitFullScreen">&#10005;</button>
      </div>
      <div class="flex-1 p-6 overflow-hidden">
        <WidgetContainer :config="fullScreenWidget" :is-mobile="false" :is-fullscreen="true" @delete="handleDeleteWidget(fullScreenWidget.id)" @configure="handleConfigureWidget(fullScreenWidget.id)" @duplicate="handleDuplicateWidget(fullScreenWidget.id)" @fullscreen="exitFullScreen" />
      </div>
    </div>

    <ConfirmDialog v-model="confirmState.show" :title="confirmState.title" :message="confirmState.message" :confirm-text="confirmState.confirmText" variant="warning" @confirm="handleGlobalConfirm" />
  </div>
</template>

<style scoped>
.badge-dashboard-view {
  display: flex;
  flex-direction: column;
  /* Mobile: subtract AppHeader height (4rem) */
  height: calc(100vh - 4rem);
  /* Break out of MainLayout <main> padding */
  margin: -1rem;
  width: calc(100% + 2rem);
  overflow: hidden;
  background: oklch(var(--b2));
  position: relative;
}

/* Desktop: AppHeader hidden, sidebar separate — use full viewport */
@media (min-width: 1024px) {
  .badge-dashboard-view {
    margin: -1.5rem;
    width: calc(100% + 3rem);
    height: 100vh;
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
  font-size: 1.25rem;
  padding: 0.25rem;
  border-radius: 0.25rem;
  cursor: pointer;
  transition: background 0.2s;
  border: 1px solid transparent;
}

.sidebar-toggle-btn:hover { background: oklch(var(--b2)); }
.sidebar-toggle-btn.is-active { background: oklch(var(--b2)); border-color: oklch(var(--b3)); }

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
  max-width: 300px;
}

.visualizer-content {
  flex: 1;
  overflow: hidden;
  position: relative;
}

@media (max-width: 640px) {
  .visualizer-toolbar { padding: 0.5rem; }
  .dashboard-name { font-size: 0.9rem; max-width: 120px; }
  .dashboard-info { gap: 0.25rem; margin-left: 0.25rem; }
  .sidebar-toggle-btn { font-size: 1rem; }
}
</style>
