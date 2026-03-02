<template>
  <div class="dashboard-grid-container" ref="containerRef">
    <!-- MOBILE LAYOUT -->
    <div 
      v-if="isMobile" 
      class="mobile-layout"
      :class="{ 'is-editing': !dashboardStore.isLocked }"
    >
      <div 
        v-for="widget in sortedWidgets" 
        :key="widget.id"
        class="mobile-widget-wrapper"
        :class="{
          'span-full': shouldSpanFullMobile(widget.id),
          'is-tall': isTallWidget(widget),
          'is-data-tall': isDataTallWidget(widget)
        }"
      >
        <WidgetContainer
          :config="widget"
          :is-mobile="true"
          @delete="handleWidgetDelete(widget.id)"
          @configure="handleWidgetConfigure(widget.id)"
          @duplicate="handleWidgetDuplicate(widget.id)"
          @fullscreen="handleWidgetFullscreen(widget.id)"
        />
      </div>
      
      <div v-if="widgets.length === 0" class="empty-state mobile">
        <div class="empty-icon">📊</div>
        <div class="empty-message">No widgets</div>
        <div class="empty-hint">Switch to desktop to add widgets</div>
      </div>
    </div>

    <!-- DESKTOP LAYOUT -->
    <GridLayout
      v-else-if="validLayoutItems.length > 0"
      :key="gridKey"
      v-model:layout="layoutItems"
      :col-num="activeColNum"
      :row-height="80"
      :is-draggable="isDraggable"
      :is-resizable="isResizable"
      :vertical-compact="true"
      :margin="[16, 16]"
      :use-css-transforms="true"
      :responsive="true"
      :breakpoints="breakpoints"
      :cols="cols"
      drag-allow-from=".vue-draggable-handle"
      drag-ignore-from=".vue-grid-item-no-drag"      
      @layout-updated="handleLayoutUpdate"
    >
      <GridItem
        v-for="item in validLayoutItems"
        :key="item.i"
        v-bind="item"
        :min-w="1"
        :min-h="1"
      >
        <WidgetContainer
          :config="getWidgetConfig(item.i)"
          :is-mobile="false"
          @delete="handleWidgetDelete(item.i)"
          @configure="handleWidgetConfigure(item.i)"
          @duplicate="handleWidgetDuplicate(item.i)"
          @fullscreen="handleWidgetFullscreen(item.i)"
        />
      </GridItem>
    </GridLayout>
    
    <div v-if="!isMobile && widgets.length === 0" class="empty-state">
      <div class="empty-icon">📊</div>
      <div class="empty-message">No widgets yet</div>
      <div class="empty-hint">
        <template v-if="!dashboardStore.isLocked">Click "Add Widget" to get started</template>
        <template v-else>Dashboard is locked</template>
      </div>
    </div>
    
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { GridLayout, GridItem } from 'grid-layout-plus'
import WidgetContainer from './WidgetContainer.vue'
import { useDashboardStore } from '@/stores/dashboard'
import type { WidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  widgets: WidgetConfig[]
  columnCount?: number
}>()

const emit = defineEmits<{
  deleteWidget: [widgetId: string]
  configureWidget: [widgetId: string]
  duplicateWidget: [widgetId: string]
  fullscreenWidget: [widgetId: string]
}>()

const dashboardStore = useDashboardStore()
const containerRef = ref<HTMLElement | null>(null)
const isMobile = ref(false)

const breakpoints = { lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 }

const gridKey = computed(() => {
  return dashboardStore.activeDashboardId || 'no-dashboard'
})

const cols = computed(() => {
  const count = props.columnCount || 12
  if (count > 0) {
    return { lg: count, md: count, sm: count, xs: 2, xxs: 2 }
  } else {
    return { lg: 12, md: 10, sm: 6, xs: 2, xxs: 2 }
  }
})

const activeColNum = computed(() => {
  const count = props.columnCount || 12
  return count > 0 ? count : 12
})

function checkMobile() {
  const width = containerRef.value ? containerRef.value.clientWidth : window.innerWidth
  isMobile.value = width < 768
}

const sortedWidgets = computed(() => {
  return [...props.widgets].sort((a, b) => {
    if (a.y !== b.y) return a.y - b.y
    return a.x - b.x
  })
})

function isIntrinsicallyFullWidth(widget: WidgetConfig): boolean {
  if (['map', 'chart', 'console', 'publisher', 'markdown', 'kvtable'].includes(widget.type)) return true
  if (widget.type === 'kv' && (widget.w > 2 || widget.h > 2)) return true
  if (['slider', 'gauge', 'stat'].includes(widget.type)) return true
  return false
}

function isTallWidget(widget: WidgetConfig): boolean {
  if (['map', 'chart', 'console', 'publisher', 'markdown'].includes(widget.type)) return true
  if (['gauge', 'stat'].includes(widget.type)) return true
  if (widget.type === 'kv' && widget.h > 2) return true
  return false
}

function isDataTallWidget(widget: WidgetConfig): boolean {
  return ['kvtable', 'pocketbase'].includes(widget.type)
}

const mobileWidgetLayout = computed(() => {
  const layoutMap = new Map<string, boolean>()
  let currentColumn = 0

  sortedWidgets.value.forEach((widget, index) => {
    if (isIntrinsicallyFullWidth(widget)) {
      layoutMap.set(widget.id, true)
      currentColumn = 0
    } else {
      if (currentColumn === 0) {
        const nextWidget = sortedWidgets.value[index + 1]
        const nextIsFull = !nextWidget || isIntrinsicallyFullWidth(nextWidget)

        if (nextIsFull) {
          layoutMap.set(widget.id, true)
          currentColumn = 0
        } else {
          layoutMap.set(widget.id, false)
          currentColumn = 1
        }
      } else {
        layoutMap.set(widget.id, false)
        currentColumn = 0 
      }
    }
  })

  return layoutMap
})

function shouldSpanFullMobile(widgetId: string): boolean {
  if (!dashboardStore.isLocked) return true
  return mobileWidgetLayout.value.get(widgetId) ?? true
}

const isDraggable = computed(() => !dashboardStore.isLocked)
const isResizable = computed(() => !dashboardStore.isLocked)

function mapWidgetsToLayout(widgets: WidgetConfig[]) {
  return widgets.map(w => ({
    i: w.id, x: w.x, y: w.y, w: w.w, h: w.h,
  }))
}

const layoutItems = ref(mapWidgetsToLayout(props.widgets))

const validWidgetIds = computed(() => {
  return new Set(props.widgets.map(w => w.id))
})

const validLayoutItems = computed(() => {
  return layoutItems.value.filter(item => validWidgetIds.value.has(item.i))
})

function getWidgetConfig(id: string): WidgetConfig | undefined {
  return props.widgets.find(w => w.id === id)
}

function hasWidgetSetChanged(currentIds: string[], newIds: string[]): boolean {
  if (currentIds.length !== newIds.length) return true
  const newIdSet = new Set(newIds)
  return currentIds.some(id => !newIdSet.has(id))
}

watch(() => props.widgets, (newWidgets) => {
  if (isMobile.value) return
  
  const currentLayoutIds = layoutItems.value.map(item => item.i)
  const newWidgetIds = newWidgets.map(w => w.id)
  
  if (hasWidgetSetChanged(currentLayoutIds, newWidgetIds)) {
    layoutItems.value = mapWidgetsToLayout(newWidgets)
  }
}, { deep: true })

function handleLayoutUpdate(newLayout: Array<{ i: string; x: number; y: number; w: number; h: number }>) {
  if (dashboardStore.isLocked || isMobile.value) return
  
  const validUpdates = newLayout.filter(item => validWidgetIds.value.has(item.i))
  layoutItems.value = newLayout
  
  const updates = validUpdates.map(item => ({
    id: item.i, x: item.x, y: item.y, w: item.w, h: item.h
  }))
  
  dashboardStore.batchUpdateLayout(updates)
}

function handleWidgetDelete(id: string) { emit('deleteWidget', id) }
function handleWidgetConfigure(id: string) { emit('configureWidget', id) }
function handleWidgetDuplicate(id: string) { emit('duplicateWidget', id) }
function handleWidgetFullscreen(id: string) { emit('fullscreenWidget', id) }

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.dashboard-grid-container {
  height: 100%;
  width: 100%;
  position: relative;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px;
  background: var(--bg);
}

.mobile-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  padding-bottom: 40px;
  transition: all 0.3s ease;
}

.mobile-layout.is-editing {
  grid-template-columns: 1fr; 
}

.mobile-widget-wrapper {
  height: auto;
  min-height: 80px; 
  display: flex;
  flex-direction: column;
}

.mobile-layout.is-editing .mobile-widget-wrapper {
  min-height: 140px;
}

.mobile-widget-wrapper.span-full {
  grid-column: span 2;
}

.mobile-widget-wrapper.is-tall {
  min-height: 250px;
}

.mobile-widget-wrapper.is-data-tall {
  min-height: 400px;
}

.mobile-layout.is-editing .mobile-widget-wrapper.span-full {
  grid-column: span 1;
}

:deep(.vue-grid-item) {
  transition: all 0.2s ease;
  background: transparent;
  touch-action: none;
}

:deep(.vue-grid-item .widget-body) {
  touch-action: auto;
}

:deep(.vue-grid-item.vue-grid-placeholder) {
  background: var(--color-info-bg) !important;
  border: 2px dashed var(--color-accent);
  border-radius: 8px;
  opacity: 0.3;
}

:deep(.vue-resizable-handle) {
  opacity: 0;
  transition: opacity 0.2s;
  background: var(--color-accent);
  border-radius: 0 0 8px 0;
}

:deep(.vue-grid-item:hover .vue-resizable-handle) {
  opacity: 0.6;
}

:deep(.vue-resizable-handle:hover) {
  opacity: 1 !important;
}

.empty-state {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  color: var(--muted);
  pointer-events: none;
  z-index: 0;
}

.empty-state.mobile {
  position: relative;
  top: auto;
  left: auto;
  transform: none;
  grid-column: span 2;
  padding: 40px 0;
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-message {
  font-size: 20px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text);
}

.empty-hint {
  font-size: 14px;
}

@media (max-width: 480px) {
  .dashboard-grid-container {
    padding: 8px;
  }
  .mobile-layout {
    gap: 8px;
  }
}
</style>
