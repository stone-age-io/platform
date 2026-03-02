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
          <!-- Data Display -->
          <div class="widget-category">
            <div class="category-label">Data Display</div>
            <div class="widget-type-buttons">
              <button class="widget-type-btn" @click="selectType('text')">
                <div class="widget-type-icon">📝</div>
                <div class="widget-type-name">Text</div>
                <div class="widget-type-desc">Display latest value</div>
              </button>
              <button class="widget-type-btn" @click="selectType('stat')">
                <div class="widget-type-icon">📊</div>
                <div class="widget-type-name">Stat Card</div>
                <div class="widget-type-desc">KPI with trend</div>
              </button>
              <button class="widget-type-btn" @click="selectType('gauge')">
                <div class="widget-type-icon">⏲️</div>
                <div class="widget-type-name">Gauge</div>
                <div class="widget-type-desc">Circular meter</div>
              </button>
              <button class="widget-type-btn" @click="selectType('status')">
                <div class="widget-type-icon">🚦</div>
                <div class="widget-type-name">Status</div>
                <div class="widget-type-desc">State mapping & watchdog</div>
              </button>
              <button class="widget-type-btn" @click="selectType('chart')">
                <div class="widget-type-icon">📈</div>
                <div class="widget-type-name">Chart</div>
                <div class="widget-type-desc">Line chart over time</div>
              </button>
              <button class="widget-type-btn" @click="selectType('kv')">
                <div class="widget-type-icon">🗄️</div>
                <div class="widget-type-name">KV Value</div>
                <div class="widget-type-desc">Single KV entry</div>
              </button>
              <button class="widget-type-btn" @click="selectType('kvtable')">
                <div class="widget-type-icon">📋</div>
                <div class="widget-type-name">KV Table</div>
                <div class="widget-type-desc">Live KV bucket table</div>
              </button>
              <button class="widget-type-btn" @click="selectType('pocketbase')">
                <div class="widget-type-icon">🐬</div>
                <div class="widget-type-name">PocketBase</div>
                <div class="widget-type-desc">Query database records</div>
              </button>
            </div>
          </div>

          <!-- Controls -->
          <div class="widget-category">
            <div class="category-label">Controls</div>
            <div class="widget-type-buttons">
              <button class="widget-type-btn" @click="selectType('button')">
                <div class="widget-type-icon">📤</div>
                <div class="widget-type-name">Button</div>
                <div class="widget-type-desc">Publish messages</div>
              </button>
              <button class="widget-type-btn" @click="selectType('switch')">
                <div class="widget-type-icon">🔄</div>
                <div class="widget-type-name">Switch</div>
                <div class="widget-type-desc">Toggle control</div>
              </button>
              <button class="widget-type-btn" @click="selectType('slider')">
                <div class="widget-type-icon">🎚️</div>
                <div class="widget-type-name">Slider</div>
                <div class="widget-type-desc">Range control</div>
              </button>
              <button class="widget-type-btn" @click="selectType('publisher')">
                <div class="widget-type-icon">📨</div>
                <div class="widget-type-name">Publisher</div>
                <div class="widget-type-desc">Send ad-hoc messages</div>
              </button>
            </div>
          </div>

          <!-- Layout -->
          <div class="widget-category">
            <div class="category-label">Layout</div>
            <div class="widget-type-buttons">
              <button class="widget-type-btn" @click="selectType('markdown')">
                <div class="widget-type-icon">📝</div>
                <div class="widget-type-name">Markdown</div>
                <div class="widget-type-desc">Static text & images</div>
              </button>
              <button class="widget-type-btn" @click="selectType('map')">
                <div class="widget-type-icon">🗺️</div>
                <div class="widget-type-name">Map</div>
                <div class="widget-type-desc">Geographic location</div>
              </button>
              <button class="widget-type-btn" @click="selectType('console')">
                <div class="widget-type-icon">📟</div>
                <div class="widget-type-name">Console</div>
                <div class="widget-type-desc">Live log stream</div>
              </button>
            </div>
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

.widget-category {
  margin-bottom: 16px;
}

.widget-category:last-child {
  margin-bottom: 0;
}

.category-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: oklch(var(--bc) / 0.5);
  margin-bottom: 8px;
  padding-left: 2px;
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
