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
      <TriangleAlert class="error-icon" :size="24" aria-hidden="true" />
      <div class="error-message">Configuration missing</div>
    </div>
    
    <template v-else>
      <div
        v-if="shouldShowHeader"
        class="widget-header vue-draggable-handle"
        :class="{ 'mobile-compact': mobileCompact }"
      >
        <div class="widget-title" :title="config.title">{{ config.title }}</div>

        <span v-if="age" class="widget-age" :title="`Last message: ${age}`">{{ dashboardStore.isLocked ? age.replace(/ ago$/, '') : age }}</span>

        <!-- Mobile expand button for data-dense widgets -->
        <button v-if="mobileExpandable" class="icon-btn" title="Expand" :aria-label="`Expand ${config.title}`" @click="$emit('fullscreen')"><Maximize :size="16" aria-hidden="true" /></button>

        <template v-if="!mobileCompact">
          <div class="widget-actions">
            <button class="icon-btn" title="Full Screen" :aria-label="`Full screen: ${config.title}`" @click="$emit('fullscreen')"><Maximize :size="16" aria-hidden="true" /></button>

            <template v-if="!dashboardStore.isLocked">
              <button class="icon-btn" title="Duplicate" :aria-label="`Duplicate ${config.title}`" @click="$emit('duplicate')"><Copy :size="16" aria-hidden="true" /></button>
              <button class="icon-btn" title="Configure" :aria-label="`Configure ${config.title}`" @click="$emit('configure')"><SlidersVertical :size="16" aria-hidden="true" /></button>
              <button class="icon-btn danger" title="Delete" :aria-label="`Delete ${config.title}`" @click="handleDelete"><X :size="16" aria-hidden="true" /></button>
            </template>
          </div>
        </template>
      </div>
      
      <div class="widget-body">
        <div v-if="error" class="widget-error">
          <TriangleAlert class="error-icon" :size="24" aria-hidden="true" />
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
import { Copy, Maximize, SlidersVertical, TriangleAlert, X } from '@lucide/vue'
import type { WidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  config?: WidgetConfig
  isMobile?: boolean
  isFullscreen?: boolean
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

// `?? null`, not `!`: a dashboard can be imported from a file, so `type` may be
// unknown. An unknown type renders the error state instead of throwing.
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

// Age of the newest buffered message. Deliberately a label only, with no
// staleness threshold: the right one differs per sensor and the reader knows it.
// KV-fed and self-sourced widgets (Status has its own watchdog) show nothing.
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
  border-radius: var(--rounded-box, 0.75rem); /* the theme's card radius */
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  /* Not `all`: that also eased the size and position the grid sets while a
     tile is dragged or resized, so it trailed the pointer. */
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
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

/* Disconnected: the banner above the grid says why; each widget only dims. */
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

/* Locked: the title is a label inside the card, not a grey strip. The strip and
   the uppercase mean editing: it is the drag handle. */
.is-locked .widget-header {
  height: auto;
  min-height: 0;
  padding: 10px 12px 0;
  background: transparent;
  border-bottom: none;
}

.is-locked .widget-title {
  font-size: 13px;
  font-weight: 500;
  text-transform: none;
  letter-spacing: 0;
}

/* Keep the label row short: the buttons may be taller than the text. */
.is-locked .icon-btn {
  margin-top: -6px;
  margin-bottom: -6px;
}

/* With a mouse, a locked tile shows its full-screen button only on hover, over
   the header corner so the title keeps the width. Touch has no hover, so there
   it stays visible in the row. */
@media (hover: hover) {
  .is-locked:not(.is-mobile) .widget-header {
    position: relative;
  }
  .is-locked:not(.is-mobile) .widget-actions {
    position: absolute;
    top: 4px;
    right: 6px;
    background: oklch(var(--b1));
    border-radius: 6px;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .is-locked:not(.is-mobile):hover .widget-actions,
  .is-locked:not(.is-mobile) .widget-actions:focus-within {
    opacity: 1;
  }
  /* The button takes the age's corner while it shows, rather than sitting
     on top of it. */
  .is-locked:not(.is-mobile):hover .widget-age,
  .is-locked:not(.is-mobile):has(.widget-actions:focus-within) .widget-age {
    visibility: hidden;
  }
}

/* A finger needs about 44px; 28px is a mouse-sized target. The button keeps
   its look and only its hit area grows, so a compact header stays short. */
.is-mobile .icon-btn {
  min-width: 44px;
  min-height: 44px;
  margin: -8px -4px;
}

/* Fixed width so the header does not shift as the text counts up. 7ch fits
   "59m ago"; a locked tile shows the short form ("59m"), so 4ch. */
.widget-age {
  font-size: 11px;
  font-family: var(--mono, monospace);
  color: oklch(var(--bc) / 0.45);
  white-space: nowrap;
  flex-shrink: 0;
  min-width: 7ch;
  text-align: right;
}

.is-locked .widget-age {
  min-width: 4ch;
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

