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

        <span v-if="age" class="widget-age" :title="`Last message: ${age}`">{{ age }}</span>

        <!-- Mobile expand button for data-dense widgets -->
        <button v-if="mobileExpandable" class="icon-btn" title="Expand" @click="$emit('fullscreen')">⛶</button>

        <template v-if="!mobileCompact">
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
import { computed, ref, watch, onErrorCaptured } from 'vue'
import { useNow } from '@vueuse/core'
import { useDashboardStore } from '@/stores/dashboard'
import { useNatsStore } from '@/stores/nats'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useConfirm } from '@/composables/useConfirm'
import { formatAge } from '@/utils/format'
import { WIDGETS } from './widgets'
import type { WidgetConfig } from '@/types/dashboard'

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
const dataStore = useWidgetDataStore()
const { confirm } = useConfirm()
const error = ref<string | null>(null)

// A 16-case switch and two hardcoded Sets of type strings used to live here.
// They are one object now: ./widgets.ts. The Sets were the worse half — leaving
// a type out of either was an error nowhere, it just made the widget behave
// wrong on a phone, where nobody developing it was looking.
//
// `?? null` rather than a non-null assertion: a dashboard is user data and can
// be imported from a file, so `type` is only as trustworthy as the JSON it came
// in on. An unknown type renders the error state below instead of throwing.
const widget = computed(() => (props.config ? WIDGETS[props.config.type] ?? null : null))

const widgetComponent = computed(() => widget.value?.component ?? null)

const shouldShowHeader = computed(() => {
  if (!dashboardStore.isLocked) return true
  if (props.isMobile) return widget.value?.mobileTitled ?? false
  return true
})
const mobileCompact = computed(() => props.isMobile && dashboardStore.isLocked)
const mobileExpandable = computed(() =>
  mobileCompact.value && (widget.value?.mobileExpandable ?? false)
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

// How old the newest message is, for the widgets fed by the shared message
// buffer. Deliberately just a label: no threshold, no colour, no setting. A
// sensor that reports hourly and one that reports every second need different
// answers to "stale", and the person reading the screen already knows which is
// which. Widgets that read KV or draw their own source (Status has its own
// watchdog) show nothing here.
const now = useNow({ interval: 1000 })
const age = computed(() => {
  if (!props.config || !widget.value?.standardDataSource) return null
  const buf = dataStore.getBuffer(props.config.id)
  if (buf.length === 0) return null
  return formatAge(now.value.getTime() - buf[buf.length - 1].timestamp)
})

// A widget that threw stays on the error card until its config changes, so
// fixing the config in the modal brings it back without a page reload.
watch(() => props.config, () => { error.value = null }, { deep: true })

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

/* Only while editing: on a locked or wall-mounted dashboard nothing is clickable
   to lift. */
.widget-container:not(.is-locked):hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.widget-container.is-locked {
  border-color: transparent;
  background: oklch(var(--b1));
}

/* Disconnected: the banner above the grid says why, once. Each widget only
   dims, rather than sixteen red borders saying the same thing. */
.widget-container.is-offline .widget-body {
  opacity: 0.6;
}

.widget-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: oklch(var(--b2) / 0.5);
  border-bottom: 1px solid oklch(var(--b2));
  flex-shrink: 0;
  gap: 8px;
  height: 40px;
}

.widget-title {
  font-size: 12px;
  font-weight: 600;
  color: oklch(var(--bc) / 0.7);
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

.widget-age {
  font-size: 11px;
  font-family: var(--mono, monospace);
  color: oklch(var(--bc) / 0.45);
  white-space: nowrap;
  flex-shrink: 0;
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

