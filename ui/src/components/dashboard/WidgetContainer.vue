<!-- ui/src/components/dashboard/WidgetContainer.vue -->
<template>
  <div 
    class="widget-container" 
    :class="{ 
      'is-locked': dashboardStore.isLocked, 
      'is-mobile': isMobile,
      'is-offline': !natsStore.isConnected
    }"
  >
    <div v-if="!config" class="widget-error">
      <div class="error-icon">⚠️</div>
      <div class="error-message">Configuration missing</div>
    </div>
    
    <template v-else>
      <!-- HEADER -->
      <div
        v-if="shouldShowHeader"
        class="widget-header vue-draggable-handle"
        :class="{ 'mobile-compact': mobileCompact }"
      >
        <div class="widget-title" :title="config.title">{{ config.title }}</div>

        <!-- Mobile expand button for data-dense widgets -->
        <button v-if="mobileExpandable" class="icon-btn" title="Expand" @click="$emit('fullscreen')">⛶</button>

        <template v-if="!mobileCompact">
          <div v-if="!natsStore.isConnected" class="offline-indicator" title="Disconnected from NATS">
            ⚠️
          </div>

          <div class="widget-actions">
            <button class="icon-btn" title="Full Screen" @click="$emit('fullscreen')">⛶</button>

            <template v-if="!dashboardStore.isLocked">
              <button class="icon-btn" title="Duplicate" @click="$emit('duplicate')">📋</button>
              <button class="icon-btn" title="Configure" @click="$emit('configure')">⚙️</button>
              <button class="icon-btn danger" title="Delete" @click="handleDelete">✕</button>
            </template>
          </div>
        </template>
      </div>
      
      <div class="widget-body">
        <div v-if="error" class="widget-error">
          <div class="error-icon">⚠️</div>
          <div class="error-message">{{ error }}</div>
        </div>
        
        <component 
          v-else
          :is="widgetComponent" 
          :config="config"
          :layout-mode="isMobile ? 'card' : 'standard'"
          :is-fullscreen="isFullscreen"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onErrorCaptured, defineAsyncComponent } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useNatsStore } from '@/stores/nats'
import { useConfirm } from '@/composables/useConfirm'
import type { WidgetConfig } from '@/types/dashboard'

// Lightweight widgets - imported synchronously
import TextWidget from '@/components/widgets/TextWidget.vue'
import ButtonWidget from '@/components/widgets/ButtonWidget.vue'
import SwitchWidget from '@/components/widgets/SwitchWidget.vue'
import SliderWidget from '@/components/widgets/SliderWidget.vue'
import StatCardWidget from '@/components/widgets/StatCardWidget.vue'
import StatusWidget from '@/components/widgets/StatusWidget.vue'

// Heavy widgets - lazy-loaded to reduce initial bundle size
const ChartWidget = defineAsyncComponent(() => import('@/components/widgets/ChartWidget.vue'))
const GaugeWidget = defineAsyncComponent(() => import('@/components/widgets/GaugeWidget.vue'))
const MapWidget = defineAsyncComponent(() => import('@/components/widgets/MapWidget.vue'))
const KvWidget = defineAsyncComponent(() => import('@/components/widgets/KvWidget.vue'))
const ConsoleWidget = defineAsyncComponent(() => import('@/components/widgets/ConsoleWidget.vue'))
const PublisherWidget = defineAsyncComponent(() => import('@/components/widgets/PublisherWidget.vue'))
const MarkdownWidget = defineAsyncComponent(() => import('@/components/widgets/MarkdownWidget.vue'))
const PocketBaseWidget = defineAsyncComponent(() => import('@/components/widgets/PocketBaseWidget.vue'))
const KvTableWidget = defineAsyncComponent(() => import('@/components/widgets/KvTableWidget.vue'))

const props = defineProps<{
  config?: WidgetConfig
  isMobile?: boolean
  isFullscreen?: boolean // <--- Added prop
}>()

const emit = defineEmits<{
  delete: []
  configure: []
  duplicate: []
  fullscreen: []
}>()

const dashboardStore = useDashboardStore()
const natsStore = useNatsStore()
const { confirm } = useConfirm()
const error = ref<string | null>(null)

const widgetComponent = computed(() => {
  if (!props.config) return null
  switch (props.config.type) {
    case 'text': return TextWidget
    case 'chart': return ChartWidget
    case 'button': return ButtonWidget
    case 'kv': return KvWidget
    case 'switch': return SwitchWidget
    case 'slider': return SliderWidget
    case 'stat': return StatCardWidget
    case 'gauge': return GaugeWidget
    case 'map': return MapWidget
    case 'console': return ConsoleWidget
    case 'publisher': return PublisherWidget
    case 'status': return StatusWidget
    case 'markdown': return MarkdownWidget
    case 'pocketbase': return PocketBaseWidget
    case 'kvtable': return KvTableWidget
    default: return null
  }
})

const MOBILE_TITLED_TYPES = new Set([
  'kvtable', 'pocketbase', 'gauge', 'markdown', 'stat', 'chart', 'map'
])
const MOBILE_EXPANDABLE_TYPES = new Set(['kvtable', 'map'])

const shouldShowHeader = computed(() => {
  if (!dashboardStore.isLocked) return true
  if (props.isMobile) return MOBILE_TITLED_TYPES.has(props.config?.type || '')
  return true
})
const mobileCompact = computed(() => props.isMobile && dashboardStore.isLocked)
const mobileExpandable = computed(() =>
  mobileCompact.value && MOBILE_EXPANDABLE_TYPES.has(props.config?.type || '')
)

async function handleDelete() {
  if (!props.config) return
  const confirmed = await confirm({
    title: 'Delete Widget',
    message: `Are you sure you want to delete "${props.config.title}"?`,
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (confirmed) {
    emit('delete')
  }
}

onErrorCaptured((err) => {
  console.error(`Widget error (${props.config?.id}):`, err)
  error.value = err.message || 'Widget error'
  return false
})
</script>

<style scoped>
.widget-container {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: oklch(var(--b1)); /* Base-100 */
  border: 1px solid oklch(var(--b3)); /* Base-300 */
  border-radius: 0.75rem; /* rounded-xl */
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transition: all 0.2s ease;
}

.widget-container:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.widget-container.is-locked {
  border-color: transparent;
  background: oklch(var(--b1));
}

.widget-container.is-offline {
  border-color: oklch(var(--er)); /* Error color */
}

.widget-container.is-offline .widget-body {
  opacity: 0.7;
  filter: grayscale(0.8);
}

.offline-indicator {
  font-size: 14px;
  margin-right: 4px;
  animation: pulse 2s infinite;
  cursor: help;
  user-select: none;
}

@keyframes pulse {
  0% { opacity: 0.5; }
  50% { opacity: 1; }
  100% { opacity: 0.5; }
}

.widget-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: oklch(var(--b3)); /* Base-200 */
  border-bottom: 1px solid oklch(var(--b3));
  flex-shrink: 0;
  gap: 8px;
  height: 40px;
}

.widget-title {
  font-size: 13px;
  font-weight: 600;
  color: oklch(var(--bc));
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.widget-header.mobile-compact {
  height: 28px;
  padding: 2px 10px;
}

.mobile-compact .widget-title {
  font-size: 11px;
}

.widget-actions {
  display: flex;
  gap: 4px;
}

.icon-btn {
  background: transparent;
  border: none;
  color: oklch(var(--bc) / 0.5);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  font-size: 14px;
  min-width: 28px;
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.icon-btn:hover {
  background: oklch(var(--b3));
  color: oklch(var(--bc));
}

.icon-btn.danger:hover {
  background: oklch(var(--er) / 0.2);
  color: oklch(var(--er));
}

.widget-body {
  flex: 1;
  min-height: 0;
  position: relative;
  container-type: size;
  display: flex;
  flex-direction: column;
}

.widget-body > * {
  flex: 1;
}

.widget-error {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 12px;
  color: oklch(var(--er));
  text-align: center;
}

.error-icon { font-size: 24px; margin-bottom: 4px; }
.error-message { font-size: 12px; line-height: 1.4; }
</style>

