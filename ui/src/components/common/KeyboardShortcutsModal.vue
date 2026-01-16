<template>
  <Teleport to="body">
    <div v-if="modelValue" class="modal-overlay" @click.self="close">
      <!-- CHANGED: .modal -> .nd-modal -->
      <div class="nd-modal">
        <div class="modal-header">
          <h3>Keyboard Shortcuts</h3>
          <button class="close-btn" @click="close">✕</button>
        </div>
        <div class="modal-body">
          <div class="shortcuts-list">
            <div v-for="(s, i) in shortcuts" :key="i" class="shortcut-row">
              <div class="keys">
                <kbd>{{ formatKey(s.key) }}</kbd>
              </div>
              <div class="desc">{{ s.description }}</div>
            </div>
            
            <!-- Always show Help shortcut -->
            <div class="shortcut-row">
              <div class="keys"><kbd>?</kbd></div>
              <div class="desc">Show this help menu</div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <div class="hint">
            Shortcuts are disabled while typing in text fields.
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import type { ShortcutDefinition } from '@/composables/useKeyboardShortcuts'

defineProps<{
  modelValue: boolean
  shortcuts: ShortcutDefinition[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function close() {
  emit('update:modelValue', false)
}

function formatKey(key: string): string {
  if (key.toLowerCase() === 'escape') return 'Esc'
  return key.toUpperCase()
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
  max-width: 400px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  display: flex;
  flex-direction: column;
  max-height: 80vh;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid oklch(var(--b3));
}

.modal-header h3 { margin: 0; font-size: 18px; }

.close-btn {
  background: none;
  border: none;
  color: oklch(var(--bc) / 0.5);
  font-size: 24px;
  cursor: pointer;
}

.modal-body { 
  padding: 0; 
  overflow-y: auto;
}

.shortcuts-list {
  padding: 20px;
}

.shortcut-row {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255,255,255,0.05);
}

.shortcut-row:last-child {
  border-bottom: none;
  margin-bottom: 0;
  padding-bottom: 0;
}

.keys {
  width: 60px;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

kbd {
  background: rgba(255,255,255,0.1);
  border: 1px solid oklch(var(--b3));
  border-radius: 4px;
  padding: 4px 8px;
  font-family: var(--mono);
  font-size: 14px;
  font-weight: 600;
  min-width: 32px;
  text-align: center;
  box-shadow: 0 2px 0 rgba(0,0,0,0.2);
  color: oklch(var(--bc));
}

.desc {
  font-size: 14px;
  color: oklch(var(--bc));
}

.modal-footer {
  padding: 12px 20px;
  background: rgba(0,0,0,0.1);
  border-top: 1px solid oklch(var(--b3));
}

.hint {
  font-size: 12px;
  color: oklch(var(--bc) / 0.5);
  text-align: center;
  font-style: italic;
}
</style>
