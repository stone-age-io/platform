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
          <div v-for="category in categories" :key="category.name" class="widget-category">
            <div class="category-label">{{ category.name }}</div>
            <div class="widget-type-buttons">
              <button
                v-for="w in category.widgets"
                :key="w.type"
                class="widget-type-btn"
                @click="selectType(w.type)"
              >
                <div class="widget-type-icon">{{ w.icon }}</div>
                <div class="widget-type-name">{{ w.label }}</div>
                <div class="widget-type-desc">{{ w.description }}</div>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { WIDGETS, WIDGET_CATEGORIES, type WidgetDefinition } from './widgets'
import type { WidgetType } from '@/types/dashboard'

defineProps<{
  modelValue: boolean
}>()

// This palette was sixteen hand-written <button> blocks, which made it the
// seventh place a new widget had to be added — and the one whose omission is
// total, since a widget missing from here cannot be added at all.
//
// Order comes from the declaration order of WIDGETS (object key order), not
// from WIDGET_TYPES, which is sorted for its own reasons.
const categories = computed(() => {
  const all = Object.entries(WIDGETS) as [WidgetType, WidgetDefinition][]
  return WIDGET_CATEGORIES.map(name => ({
    name,
    widgets: all
      .filter(([, w]) => w.category === name)
      .map(([type, w]) => ({ type, ...w })),
  }))
})

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
