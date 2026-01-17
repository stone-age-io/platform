<!-- ui/src/components/common/ResponseModal.vue -->
<template>
  <Teleport to="body">
    <div v-if="modelValue" class="modal-overlay" @click.self="close">
      <!-- CHANGED: .modal -> .nd-modal to avoid DaisyUI conflict -->
      <div class="nd-modal response-modal" :class="status">
        <div class="modal-header">
          <div class="header-content">
            <span class="status-icon">{{ statusIcon }}</span>
            <h3>{{ title }}</h3>
          </div>
          <button class="close-btn" @click="close">✕</button>
        </div>
        
        <div class="modal-body">
          <div class="meta-row">
            <span class="meta-tag" :class="status">{{ status.toUpperCase() }}</span>
            <span v-if="latency !== undefined" class="meta-latency">
              ⏱ {{ latency }}ms
            </span>
          </div>
          
          <div class="response-content">
            <JsonViewer :data="formattedData" />
          </div>
        </div>
        
        <div class="modal-footer">
          <button class="btn-secondary" @click="copyToClipboard">
            {{ copyLabel }}
          </button>
          <button class="btn-primary" @click="close" ref="closeBtn">
            Close
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, nextTick, watch } from 'vue'
import JsonViewer from './JsonViewer.vue'

const props = defineProps<{
  modelValue: boolean
  title: string
  status: 'success' | 'error'
  data: string
  latency?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const closeBtn = ref<HTMLButtonElement | null>(null)
const copyLabel = ref('Copy')

const statusIcon = computed(() => props.status === 'success' ? '✓' : '⚠️')

const formattedData = computed(() => {
  try {
    return JSON.parse(props.data)
  } catch {
    return props.data
  }
})

function close() {
  emit('update:modelValue', false)
}

async function copyToClipboard() {
  try {
    const text = typeof formattedData.value === 'object'
      ? JSON.stringify(formattedData.value, null, 2)
      : String(formattedData.value)

    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      // Fallback
      const textArea = document.createElement("textarea")
      textArea.value = text
      textArea.style.position = "fixed"
      textArea.style.left = "-9999px"
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
    }

    copyLabel.value = 'Copied!'
    setTimeout(() => {
      copyLabel.value = 'Copy'
    }, 2000)

  } catch (err) {
    console.warn('Clipboard copy failed:', err)
    copyLabel.value = 'Error'
    setTimeout(() => {
      copyLabel.value = 'Copy'
    }, 2000)
  }
}

watch(() => props.modelValue, (isOpen) => {
  if (isOpen) {
    nextTick(() => {
      closeBtn.value?.focus()
    })
  }
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
  z-index: 9999;
  animation: fadeIn 0.2s ease-out;
  backdrop-filter: blur(2px);
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

/* CHANGED: .modal -> .nd-modal */
.nd-modal {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  width: 100%;
  max-width: 600px;
  min-width: 400px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* Status variants */
.nd-modal.success { border-top: 4px solid var(--color-success); }
.nd-modal.error { border-top: 4px solid var(--color-error); }

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
}

.header-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-icon { font-size: 20px; }
.nd-modal.success .status-icon { color: var(--color-success); }
.nd-modal.error .status-icon { color: var(--color-error); }

.modal-header h3 { margin: 0; font-size: 18px; font-weight: 600; }

.close-btn {
  background: none;
  border: none;
  color: var(--muted);
  font-size: 24px;
  cursor: pointer;
  padding: 4px;
  line-height: 1;
  transition: color 0.2s;
}

.close-btn:hover { color: var(--text); }

.modal-body {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.meta-tag {
  font-size: 11px;
  font-weight: 700;
  padding: 4px 8px;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.meta-tag.success { background: var(--color-success-bg); color: var(--color-success); }
.meta-tag.error { background: var(--color-error-bg); color: var(--color-error); }

.meta-latency {
  font-size: 12px;
  color: var(--muted);
  font-family: var(--mono);
}

.response-content {
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 16px;
  overflow: auto;
  max-height: 400px;
}

.modal-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  background: rgba(0,0,0,0.02);
}

.btn-primary, .btn-secondary {
  padding: 8px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-primary {
  background: var(--color-primary);
  color: white;
}

.btn-primary:hover {
  background: var(--color-primary-hover);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
  border: 1px solid var(--border);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
}

/* Mobile adjustments */
@media (max-width: 480px) {
  .nd-modal {
    width: 95%;
    min-width: auto;
  }
}
</style>
