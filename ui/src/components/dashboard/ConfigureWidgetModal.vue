<template>
  <Teleport to="body">
    <div v-if="modelValue && widgetId" class="modal-overlay" @click.self="close">
      <!-- CHANGED: .modal -> .nd-modal -->
      <div class="nd-modal" :class="{ 'modal-large': widgetType === 'map' }">
        <div class="modal-header">
          <h3>Configure Widget</h3>
          <button class="close-btn" @click="close">✕</button>
        </div>
        <div class="modal-body">
          <!-- Common Title -->
          <ConfigCommon :form="form" :errors="errors" />
          
          <!-- Data Source (Shared by visualization widgets) -->
          <ConfigDataSource 
            v-if="showDataSourceConfig"
            :form="form" 
            :errors="errors" 
            :allow-multiple="widgetType === 'console'"
          />

          <!-- Widget Specific Config (Dynamic) -->
          <component 
            v-if="activeConfigComponent"
            :is="activeConfigComponent"
            :form="form"
            :errors="errors"
          />
          
          <div class="modal-actions">
            <button class="btn-secondary" @click="close">
              Cancel
            </button>
            <button class="btn-primary" @click="save">
              Save Changes
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import { useWidgetForm } from '@/composables/useWidgetForm'
import ConfigCommon from './config/ConfigCommon.vue'
import ConfigDataSource from './config/ConfigDataSource.vue'

const props = defineProps<{
  modelValue: boolean
  widgetId: string | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const { form, errors, widgetType, activeConfigComponent, showDataSourceConfig, save, close } =
  useWidgetForm({
    widgetId: toRef(props, 'widgetId'),
    onSaved: () => emit('saved'),
    onClose: () => emit('update:modelValue', false),
  })
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
  z-index: 1000;
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

.nd-modal.modal-large {
  max-width: 800px;
  max-height: 90vh;
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

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid oklch(var(--b3));
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.btn-primary {
  background: oklch(var(--p));
  color: white;
}

.btn-primary:hover {
  background: oklch(var(--pf));
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.1);
  color: oklch(var(--bc));
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
}
</style>
