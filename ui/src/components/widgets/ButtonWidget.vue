<template>
  <div class="button-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Card Layout (Mobile) -->
    <template v-if="layoutMode === 'card'">
      <div 
        class="card-content" 
        :style="buttonStyle"
        :class="{ 'is-disabled': isDisabled }"
      >
        <div class="card-info">
          <div class="card-title">{{ config.title }}</div>
          <div class="card-label">
            <span v-if="isLoading" class="spinner-small"></span>
            <span v-else>{{ currentLabel }}</span>
          </div>
        </div>
      </div>
      
      <button 
        class="card-overlay-btn"
        :disabled="isDisabled"
        @click="handleClick"
      >
        <div v-if="showSuccess" class="ripple-effect"></div>
      </button>
    </template>

    <!-- Standard Layout (Desktop) -->
    <template v-else>
      <button 
        class="widget-button"
        :style="buttonStyle"
        :class="{ 'success-state': showSuccess, 'error-state': showError, 'loading-state': isLoading }"
        :disabled="isDisabled"
        @click="handleClick"
      >
        <div class="button-content">
          <span v-if="isLoading" class="spinner"></span>
          <span v-else-if="currentIcon" class="button-icon">{{ currentIcon }}</span>
          <span class="button-label">{{ currentLabel }}</span>
        </div>
        <div v-if="showSuccess" class="success-ripple"></div>
      </button>
    </template>
    
    <!-- Response Modal -->
    <ResponseModal
      v-model="showResponseModal"
      :title="config.title"
      :status="responseStatus"
      :data="responseData"
      :latency="responseLatency"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useDesignTokens } from '@/composables/useDesignTokens'
import type { WidgetConfig } from '@/types/dashboard'
import { encodeString, decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'
import ResponseModal from '@/components/common/ResponseModal.vue'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const { semanticColors } = useDesignTokens()

// State
const showSuccess = ref(false)
const showError = ref(false)
const isLoading = ref(false)

// Response Modal State
const showResponseModal = ref(false)
const responseData = ref('')
const responseStatus = ref<'success' | 'error'>('success')
const responseLatency = ref(0)

// Config
const defaultLabel = computed(() => props.config.buttonConfig?.label || 'Publish')
const buttonColor = computed(() => props.config.buttonConfig?.color || semanticColors.value.primary)
const actionType = computed(() => props.config.buttonConfig?.actionType || 'publish')
const timeout = computed(() => props.config.buttonConfig?.timeout || 1000)

const isDisabled = computed(() => !natsStore.isConnected || showSuccess.value || isLoading.value)

const currentLabel = computed(() => {
  if (!natsStore.isConnected) return 'Offline'
  if (isLoading.value) return 'Waiting...'
  if (showSuccess.value) return actionType.value === 'request' ? 'Done' : 'Sent!'
  if (showError.value) return 'Error'
  return defaultLabel.value
})

const currentIcon = computed(() => {
  if (!natsStore.isConnected) return ''
  if (showSuccess.value) return '✓'
  if (showError.value) return '✕'
  return actionType.value === 'request' ? '⇄' : null
})

const buttonStyle = computed(() => {
  if (showSuccess.value) {
    return {
      backgroundColor: semanticColors.value.success,
      borderColor: semanticColors.value.success,
      color: 'white'
    }
  }
  if (showError.value) {
    return {
      backgroundColor: semanticColors.value.error,
      borderColor: semanticColors.value.error,
      color: 'white'
    }
  }
  
  return {
    backgroundColor: buttonColor.value,
    borderColor: buttonColor.value,
    '--hover-bg': adjustColorOpacity(buttonColor.value, 0.8)
  }
})

const publishSubject = computed(() => {
  const raw = props.config.buttonConfig?.publishSubject || 'button.clicked'
  return resolveTemplate(raw, dashboardStore.currentVariableValues)
})

const publishPayload = computed(() => {
  const raw = props.config.buttonConfig?.payload || '{}'
  return resolveTemplate(raw, dashboardStore.currentVariableValues)
})

async function handleClick() {
  if (!natsStore.nc) return
  
  const subject = publishSubject.value
  const payload = encodeString(publishPayload.value)

  if (actionType.value === 'request') {
    await handleRequest(subject, payload)
  } else {
    handlePublish(subject, payload)
  }
}

function handlePublish(subject: string, payload: Uint8Array) {
  try {
    natsStore.nc!.publish(subject, payload)
    triggerSuccess()
  } catch (err) {
    triggerError()
  }
}

async function handleRequest(subject: string, payload: Uint8Array) {
  isLoading.value = true
  const start = Date.now()
  
  try {
    const msg = await natsStore.nc!.request(subject, payload, { timeout: timeout.value })
    responseLatency.value = Date.now() - start
    responseData.value = decodeBytes(msg.data)
    responseStatus.value = 'success'
    triggerSuccess()
    showResponseModal.value = true
  } catch (err: any) {
    responseLatency.value = Date.now() - start
    responseData.value = err.message || 'Request failed'
    responseStatus.value = 'error'
    triggerError()
    showResponseModal.value = true
  } finally {
    isLoading.value = false
  }
}

function triggerSuccess() {
  showSuccess.value = true
  setTimeout(() => { showSuccess.value = false }, 1500)
}

function triggerError() {
  showError.value = true
  setTimeout(() => { showError.value = false }, 1500)
}

function adjustColorOpacity(hex: string, opacity: number) {
  if (!hex.startsWith('#')) return hex
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `rgba(${r}, ${g}, ${b}, ${opacity})`
}
</script>

<style scoped>
.button-widget {
  height: 100%;
  width: 100%;
  position: relative;
}

/* --- CARD LAYOUT --- */
.button-widget.card-layout {
  padding: 0; 
  display: flex;
}

.card-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 12px;
  color: white; 
  transition: background-color 0.2s ease, opacity 0.2s ease;
}

.card-content.is-disabled {
  opacity: 0.7;
  filter: grayscale(0.2);
}

.card-info {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 100%;
  overflow: hidden;
  text-align: center;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: inherit; 
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
  margin-bottom: 2px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.card-label {
  font-size: 13px;
  color: rgba(255,255,255,0.9);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
  min-height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-overlay-btn {
  position: absolute;
  inset: 0;
  background: transparent;
  border: none;
  cursor: pointer;
  z-index: 5;
  overflow: hidden;
}

.card-overlay-btn:disabled {
  cursor: not-allowed;
}

.ripple-effect {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 100px;
  height: 100px;
  background: rgba(255,255,255,0.3);
  border-radius: 50%;
  transform: translate(-50%, -50%) scale(0);
  animation: ripple-card 0.5s ease-out forwards;
}

@keyframes ripple-card {
  to { transform: translate(-50%, -50%) scale(4); opacity: 0; }
}

/* --- STANDARD LAYOUT --- */
.button-widget:not(.card-layout) {
  padding: 8px;
  display: flex;
}

.widget-button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  color: white;
  font-weight: 600;
  font-size: 14px;
  position: relative;
  overflow: hidden;
  user-select: none;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.widget-button:hover:not(:disabled):not(.success-state):not(.loading-state) {
  background-color: var(--hover-bg) !important;
}

.widget-button:active:not(:disabled) {
  transform: translateY(0);
}

.widget-button:disabled {
  opacity: 0.8;
  cursor: not-allowed;
}

.button-content {
  display: flex;
  align-items: center;
  gap: 8px;
  z-index: 2;
}

.button-icon { font-size: 1.2em; line-height: 1; }

.success-ripple {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 20px;
  height: 20px;
  background: rgba(255, 255, 255, 0.4);
  border-radius: 50%;
  transform: translate(-50%, -50%);
  animation: ripple 0.6s ease-out;
}

@keyframes ripple {
  0% { transform: scale(0); opacity: 0.5; }
  100% { transform: scale(4); opacity: 0; }
}

/* Spinners */
.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.spinner-small {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
