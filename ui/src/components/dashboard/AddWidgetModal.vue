<template>
  <Teleport to="body">
    <div v-if="modelValue" class="modal-overlay" @click.self="close">
      <!-- CHANGED: .modal -> .nd-modal -->
      <div class="nd-modal">
        <div class="modal-header">
          <h3>Add Widget</h3>
          <button class="close-btn" @click="close">✕</button>
        </div>
        <div class="modal-body">
          <p>Select widget type to add:</p>
          <div class="widget-type-buttons">
            <!-- ... buttons ... -->
            <!-- Visualization Widgets -->
            <button class="widget-type-btn" @click="selectType('text')">
              <div class="widget-type-icon">📝</div>
              <div class="widget-type-name">Text Widget</div>
              <div class="widget-type-desc">Display latest value</div>
            </button>
            <button class="widget-type-btn" @click="selectType('status')">
              <div class="widget-type-icon">🚦</div>
              <div class="widget-type-name">Status Indicator</div>
              <div class="widget-type-desc">State mapping & watchdog</div>
            </button>
            <button class="widget-type-btn" @click="selectType('chart')">
              <div class="widget-type-icon">📈</div>
              <div class="widget-type-name">Chart Widget</div>
              <div class="widget-type-desc">Line chart over time</div>
            </button>
            <button class="widget-type-btn" @click="selectType('stat')">
              <div class="widget-type-icon">📊</div>
              <div class="widget-type-name">Stat Card</div>
              <div class="widget-type-desc">KPI with trend</div>
            </button>
            <button class="widget-type-btn" @click="selectType('gauge')">
              <div class="widget-type-icon">⏲️</div>
              <div class="widget-type-name">Gauge Widget</div>
              <div class="widget-type-desc">Circular meter</div>
            </button>
            
            <!-- Control Widgets -->
            <button class="widget-type-btn" @click="selectType('button')">
              <div class="widget-type-icon">📤</div>
              <div class="widget-type-name">Button Widget</div>
              <div class="widget-type-desc">Publish messages</div>
            </button>
            <button class="widget-type-btn" @click="selectType('switch')">
              <div class="widget-type-icon">🔄</div>
              <div class="widget-type-name">Switch Widget</div>
              <div class="widget-type-desc">Toggle control</div>
            </button>
            <button class="widget-type-btn" @click="selectType('slider')">
              <div class="widget-type-icon">🎚️</div>
              <div class="widget-type-name">Slider Widget</div>
              <div class="widget-type-desc">Range control</div>
            </button>
            
            <!-- Data Widgets -->
            <button class="widget-type-btn" @click="selectType('kv')">
              <div class="widget-type-icon">🗄️</div>
              <div class="widget-type-name">KV Widget</div>
              <div class="widget-type-desc">Display KV values</div>
            </button>

            <!-- PocketBase Widget -->
            <button class="widget-type-btn" @click="selectType('pocketbase')">
              <div class="widget-type-icon">🐬</div>
              <div class="widget-type-name">PocketBase</div>
              <div class="widget-type-desc">Query database records</div>
            </button>
            
            <!-- Map Widget -->
            <button class="widget-type-btn" @click="selectType('map')">
              <div class="widget-type-icon">🗺️</div>
              <div class="widget-type-name">Map Widget</div>
              <div class="widget-type-desc">Geographic location</div>
            </button>

            <!-- Console Widget -->
            <button class="widget-type-btn" @click="selectType('console')">
              <div class="widget-type-icon">📟</div>
              <div class="widget-type-name">Console</div>
              <div class="widget-type-desc">Live log stream</div>
            </button>

            <!-- Publisher Widget -->
            <button class="widget-type-btn" @click="selectType('publisher')">
              <div class="widget-type-icon">📨</div>
              <div class="widget-type-name">Publisher</div>
              <div class="widget-type-desc">Send ad-hoc messages</div>
            </button>

            <!-- Markdown Widget -->
            <button class="widget-type-btn" @click="selectType('markdown')">
              <div class="widget-type-icon">📝</div>
              <div class="widget-type-name">Markdown</div>
              <div class="widget-type-desc">Static text & images</div>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import type { WidgetType } from '@/types/dashboard'

interface Props {
  modelValue: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select': [type: WidgetType]
}>()

function close() {
  emit('update:modelValue', false)
}

function selectType(type: WidgetType) {
  emit('select', type)
  close()
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  backdrop-filter: blur(2px);
}

/* CHANGED: .modal -> .nd-modal */
.nd-modal {
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid oklch(var(--b3));
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
}

.close-btn {
  background: none;
  border: none;
  color: oklch(var(--bc) / 0.5);
  font-size: 24px;
  cursor: pointer;
  transition: color 0.2s;
}

.close-btn:hover {
  color: oklch(var(--bc));
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
}

.modal-body p {
  margin: 0 0 16px 0;
  color: oklch(var(--bc));
  font-size: 14px;
}

.widget-type-buttons {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 12px;
}

.widget-type-btn {
  background: oklch(var(--b1));
  border: 2px solid oklch(var(--b3));
  border-radius: 8px;
  padding: 20px 10px;
  cursor: pointer;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: oklch(var(--bc));
  transition: all 0.2s;
}

.widget-type-btn:hover {
  border-color: oklch(var(--a));
  background: oklch(var(--in) / 0.1);
  transform: translateY(-2px);
}

.widget-type-icon {
  font-size: 32px;
}

.widget-type-name {
  font-size: 14px;
  font-weight: 600;
  color: oklch(var(--bc));
}

.widget-type-desc {
  font-size: 11px;
  color: oklch(var(--bc) / 0.5);
  line-height: 1.3;
}

@media (max-width: 600px) {
  .widget-type-buttons {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  }
  
  .widget-type-btn {
    padding: 16px 8px;
  }
  
  .widget-type-icon {
    font-size: 28px;
  }
}
</style>
